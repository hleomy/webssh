import { useEffect, useRef, useState, useCallback } from 'react'
import { useApp } from '../../store'
import * as api from '../../utils/api'
import type { SftpFile } from '../../types'
import { formatSize, formatDate, formatMode } from '../../utils/helpers'

interface Props {
  serverId: string
}

interface LocalFile {
  name: string
  path: string
  size: number
  is_dir: boolean
  mod_time: string
}

export default function SftpPanel({ serverId }: Props) {
  const { showToast } = useApp()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [drag, setDrag] = useState<'upload' | 'download' | null>(null)

  // Remote state
  const [remotePath, setRemotePath] = useState('/')
  const [remoteFiles, setRemoteFiles] = useState<SftpFile[]>([])
  const [remoteLoading, setRemoteLoading] = useState(false)
  const [remoteSelected, setRemoteSelected] = useState<Set<string>>(new Set())

  // Local state
  const [localPath, setLocalPath] = useState('')
  const [localFiles, setLocalFiles] = useState<LocalFile[]>([])
  const [localLoading, setLocalLoading] = useState(false)
  const [localSelected, setLocalSelected] = useState<Set<string>>(new Set())
  const [localDirHandle, setLocalDirHandle] = useState<any>(null)

  // Editor
  const [editing, setEditing] = useState<{ path: string; content: string; remote: boolean } | null>(null)

  // Transfer status
  const [transfers, setTransfers] = useState<{ name: string; progress: number; status: string }[]>([])

  const hasFSAccess = typeof (window as any).showDirectoryPicker === 'function'

  // ========== REMOTE ==========
  const listRemote = useCallback(async (p: string) => {
    setRemoteLoading(true)
    setRemoteSelected(new Set())
    try {
      const r = await api.get<SftpFile[]>(`/servers/${serverId}/sftp/list`, { path: p })
      setRemoteFiles(r)
      setRemotePath(p)
    } catch (e: any) {
      showToast(e.message || '远端列表失败', 'error')
    } finally {
      setRemoteLoading(false)
    }
  }, [serverId])

  useEffect(() => {
    listRemote('/')
  }, [serverId])

  const remoteGoUp = () => {
    if (remotePath === '/') return
    const idx = remotePath.lastIndexOf('/')
    listRemote(idx === 0 ? '/' : remotePath.slice(0, idx))
  }

  const remoteEnter = (f: SftpFile) => {
    if (f.is_dir) listRemote(f.path)
    else openRemoteEditor(f)
  }

  const remoteSelect = (f: SftpFile, e: React.MouseEvent) => {
    e.stopPropagation()
    setRemoteSelected(prev => {
      const next = new Set(prev)
      if (e.ctrlKey || e.metaKey) {
        if (next.has(f.path)) next.delete(f.path); else next.add(f.path)
      } else {
        next.clear()
        next.add(f.path)
      }
      return next
    })
  }

  const remoteMkdir = async () => {
    const name = prompt('新目录名')
    if (!name) return
    const full = (remotePath === '/' ? '' : remotePath) + '/' + name
    try {
      await api.post(`/servers/${serverId}/sftp/mkdir?path=${encodeURIComponent(full)}`)
      showToast('创建成功', 'success')
      listRemote(remotePath)
    } catch (e: any) {
      showToast(e.message || '创建失败', 'error')
    }
  }

  const remoteDelete = async (f: SftpFile) => {
    const targets = remoteSelected.size > 0 && remoteSelected.has(f.path)
      ? Array.from(remoteSelected)
      : [f.path]
    if (!confirm(`确认删除 ${targets.length} 个项目？`)) return
    try {
      for (const p of targets) {
        const isDir = remoteFiles.find(r => r.path === p)?.is_dir
        await api.del(`/servers/${serverId}/sftp/delete?path=${encodeURIComponent(p)}&recursive=${isDir}`)
      }
      showToast(`删除 ${targets.length} 个项目成功`, 'success')
      listRemote(remotePath)
    } catch (e: any) {
      showToast(e.message || '删除失败', 'error')
    }
  }

  const remoteRename = async (f: SftpFile) => {
    const newName = prompt('新名称', f.name)
    if (!newName || newName === f.name) return
    const parent = f.path.slice(0, f.path.length - f.name.length)
    try {
      await api.post(`/servers/${serverId}/sftp/rename?old=${encodeURIComponent(f.path)}&new=${encodeURIComponent((parent + newName).replace('//', '/'))}`)
      showToast('重命名成功', 'success')
      listRemote(remotePath)
    } catch (e: any) {
      showToast(e.message || '重命名失败', 'error')
    }
  }

  const openRemoteEditor = async (f: SftpFile) => {
    try {
      const r = await api.get<{ content: string }>(`/servers/${serverId}/sftp/read?path=${encodeURIComponent(f.path)}`)
      setEditing({ path: f.path, content: r.content, remote: true })
    } catch (e: any) {
      showToast(e.message || '读取失败', 'error')
    }
  }

  const saveRemoteEditor = async () => {
    if (!editing) return
    try {
      const encoded = btoa(unescape(encodeURIComponent(editing.content)))
      await api.post(`/servers/${serverId}/sftp/write`, { path: editing.path, content: encoded })
      showToast('保存成功', 'success')
      setEditing(null)
      listRemote(remotePath)
    } catch (e: any) {
      showToast(e.message || '保存失败', 'error')
    }
  }

  // ========== LOCAL ==========
  const listLocal = async (dirHandle: any) => {
    setLocalLoading(true)
    setLocalSelected(new Set())
    try {
      const entries: LocalFile[] = []
      const iter = (dirHandle as any).values()
      for await (const entry of iter) {
        const handle = await (dirHandle as any).getFileHandle(entry.name).catch(() => null)
        const dirHandle2 = await (dirHandle as any).getDirectoryHandle(entry.name).catch(() => null)
        let size = 0
        let mtime = ''
        if (handle) {
          const file = await handle.getFile()
          size = file.size
          mtime = new Date(file.lastModified).toISOString()
        }
        entries.push({
          name: entry.name,
          path: entry.name,
          size,
          is_dir: !!dirHandle2,
          mod_time: mtime
        })
      }
      entries.sort((a, b) => {
        if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1
        return a.name.localeCompare(b.name)
      })
      setLocalFiles(entries)
      setLocalPath(dirHandle.name || '/')
      setLocalDirHandle(dirHandle)
    } catch (e: any) {
      if (e.name !== 'AbortError') {
        showToast(e.message || '本地目录读取失败', 'error')
      }
    } finally {
      setLocalLoading(false)
    }
  }

  const openLocalDir = async () => {
    if (!hasFSAccess) {
      showToast('当前浏览器不支持本地目录浏览（请使用 Chrome/Edge）', 'error')
      return
    }
    try {
      const dirHandle = await (window as any).showDirectoryPicker()
      listLocal(dirHandle)
    } catch (e: any) {
      if (e.name !== 'AbortError') {
        showToast(e.message || '选择目录失败', 'error')
      }
    }
  }

  const localGoUp = async () => {
    if (!localDirHandle) return
    try {
      const parent = await localDirHandle.getParent()
      listLocal(parent)
    } catch {
      showToast('已到根目录', 'info')
    }
  }

  const localEnter = async (f: LocalFile) => {
    if (!f.is_dir || !localDirHandle) return
    try {
      const sub = await localDirHandle.getDirectoryHandle(f.name)
      listLocal(sub)
    } catch (e: any) {
      showToast(e.message || '打开目录失败', 'error')
    }
  }

  const localSelect = (f: LocalFile, e: React.MouseEvent) => {
    e.stopPropagation()
    setLocalSelected(prev => {
      const next = new Set(prev)
      if (e.ctrlKey || e.metaKey) {
        if (next.has(f.name)) next.delete(f.name); else next.add(f.name)
      } else {
        next.clear()
        next.add(f.name)
      }
      return next
    })
  }

  const localDelete = async (f: LocalFile) => {
    if (!localDirHandle) return
    if (!confirm(`确认删除 ${f.name}？`)) return
    try {
      await localDirHandle.removeEntry(f.name, { recursive: f.is_dir })
      showToast('删除成功', 'success')
      listLocal(localDirHandle)
    } catch (e: any) {
      showToast(e.message || '删除失败', 'error')
    }
  }

  // ========== TRANSFERS ==========
  const uploadToRemote = async (files: LocalFile[]) => {
    if (!localDirHandle || files.length === 0) return
    const newTransfers = files.map(f => ({ name: f.name, progress: 0, status: 'uploading' }))
    setTransfers(prev => [...prev, ...newTransfers])
    let success = 0
    for (let i = 0; i < files.length; i++) {
      const f = files[i]
      if (f.is_dir) continue
      try {
        const fileHandle = await localDirHandle.getFileHandle(f.name)
        const file = await fileHandle.getFile()
        const form = new FormData()
        form.append('file', file)
        const remote = (remotePath === '/' ? '' : remotePath) + '/' + f.name
        await api.post(`/servers/${serverId}/sftp/upload?path=${encodeURIComponent(remote)}`, form, {
          headers: { 'Content-Type': 'multipart/form-data' }
        })
        success++
        setTransfers(prev => prev.map((t, idx) => idx === prev.length - files.length + i ? { ...t, progress: 100, status: 'done' } : t))
      } catch (e: any) {
        setTransfers(prev => prev.map((t, idx) => idx === prev.length - files.length + i ? { ...t, status: 'error: ' + e.message } : t))
      }
    }
    showToast(`上传完成 ${success}/${files.length}`, success === files.length ? 'success' : 'info')
    listRemote(remotePath)
    if (localDirHandle) listLocal(localDirHandle)
    setTimeout(() => setTransfers(prev => prev.slice(files.length)), 3000)
  }

  const downloadToLocal = async (files: SftpFile[]) => {
    if (!localDirHandle || files.length === 0) return
    const newTransfers = files.map(f => ({ name: f.name, progress: 0, status: 'downloading' }))
    setTransfers(prev => [...prev, ...newTransfers])
    let success = 0
    for (let i = 0; i < files.length; i++) {
      const f = files[i]
      if (f.is_dir) continue
      try {
        const token = localStorage.getItem('webssh_token')
        const resp = await fetch(`/api/servers/${serverId}/sftp/download?path=${encodeURIComponent(f.path)}&token=${token}`)
        const blob = await resp.blob()
        const writable = await localDirHandle.getFileHandle(f.name, { create: true })
        const ws = await writable.createWritable()
        await ws.write(blob)
        await ws.close()
        success++
        setTransfers(prev => prev.map((t, idx) => idx === prev.length - files.length + i ? { ...t, progress: 100, status: 'done' } : t))
      } catch (e: any) {
        setTransfers(prev => prev.map((t, idx) => idx === prev.length - files.length + i ? { ...t, status: 'error: ' + e.message } : t))
      }
    }
    showToast(`下载完成 ${success}/${files.length}`, success === files.length ? 'success' : 'info')
    if (localDirHandle) listLocal(localDirHandle)
    setTimeout(() => setTransfers(prev => prev.slice(files.length)), 3000)
  }

  // ========== EDITOR ==========
  const saveEditor = async () => {
    if (!editing) return
    if (editing.remote) {
      await saveRemoteEditor()
    } else {
      if (!localDirHandle) return
      try {
        const handle = await localDirHandle.getFileHandle(editing.path.split('/').pop()!, { create: true })
        const writable = await handle.createWritable()
        await writable.write(editing.content)
        await writable.close()
        showToast('保存成功', 'success')
        setEditing(null)
        listLocal(localDirHandle)
      } catch (e: any) {
        showToast(e.message || '保存失败', 'error')
      }
    }
  }

  if (editing) {
    return (
      <div className="editor">
        <div className="editor-toolbar">
          <span className="sftp-path">[{editing.remote ? '远端' : '本地'}] {editing.path}</span>
          <button onClick={saveEditor} className="primary">保存</button>
          <button onClick={() => setEditing(null)}>取消</button>
        </div>
        <textarea
          value={editing.content}
          onChange={(e) => setEditing({ ...editing, content: e.target.value })}
          spellCheck={false}
        />
      </div>
    )
  }

  return (
    <div className="xftp-container">
      {/* ========== 远端面板 ========== */}
      <div className="xftp-panel">
        <div className="xftp-panel-header">
          <span className="xftp-panel-label remote">远端服务器</span>
        </div>
        <div className="xftp-pathbar">
          <button onClick={remoteGoUp} disabled={remotePath === '/'} className="xftp-nav-btn">↑</button>
          <input
            className="xftp-path-input"
            value={remotePath}
            onKeyDown={(e) => { if (e.key === 'Enter') listRemote(remotePath) }}
            onChange={(e) => setRemotePath(e.target.value)}
          />
          <button onClick={() => listRemote(remotePath)} className="xftp-nav-btn">↻</button>
        </div>
        <div className="xftp-filelist">
          <div className="xftp-filelist-header">
            <span className="col-name">名称</span>
            <span className="col-size">大小</span>
            <span className="col-mode">权限</span>
            <span className="col-time">修改时间</span>
          </div>
          <div className="xftp-filelist-body">
            {remoteLoading && <div className="xftp-empty">加载中...</div>}
            {!remoteLoading && remoteFiles.length === 0 && <div className="xftp-empty">空目录</div>}
            {remoteFiles.map((f) => (
              <div
                className={`xftp-file-row ${remoteSelected.has(f.path) ? 'selected' : ''}`}
                key={f.path}
                onClick={(e) => remoteSelect(f, e)}
                onDoubleClick={() => remoteEnter(f)}
              >
                <span className="col-name">
                  <span className="xftp-icon">{f.is_dir ? '📁' : '📄'}</span>
                  {f.name}
                </span>
                <span className="col-size muted">{f.is_dir ? '' : formatSize(f.size)}</span>
                <span className="col-mode muted">{formatMode(f.mode)}</span>
                <span className="col-time muted">{formatDate(f.mod_time)}</span>
              </div>
            ))}
          </div>
        </div>
        <div className="xftp-toolbar">
          <button onClick={remoteMkdir} title="新建目录">新建目录</button>
          <button onClick={() => {
            const sel = Array.from(remoteSelected).map(p => remoteFiles.find(f => f.path === p)).filter(Boolean) as SftpFile[]
            if (sel.length) downloadToLocal(sel)
          }} disabled={remoteSelected.size === 0 || !localDirHandle} title="下载到本地">← 下载</button>
          <button onClick={() => remoteSelected.size > 0 && remoteRename(remoteFiles.find(f => remoteSelected.has(f.path))!)} disabled={remoteSelected.size !== 1}>重命名</button>
          <button onClick={() => remoteSelected.size > 0 && remoteDelete(remoteFiles.find(f => remoteSelected.has(f.path))!)} disabled={remoteSelected.size === 0} className="danger">删除</button>
          <span className="xftp-count">{remoteFiles.length} 项</span>
        </div>
      </div>

      {/* ========== 中间操作栏 ========== */}
      <div className="xftp-actions">
        <button
          className="xftp-action-btn upload"
          onClick={() => {
            if (!localDirHandle) { showToast('请先打开本地目录', 'info'); return }
            const sel = Array.from(localSelected).map(n => localFiles.find(f => f.name === n)).filter(Boolean) as LocalFile[]
            if (sel.length) uploadToRemote(sel)
          }}
          disabled={localSelected.size === 0 || !localDirHandle}
          title="上传选中文件到远端"
        >
          ↑ 上传
        </button>
        <button
          className="xftp-action-btn download"
          onClick={() => {
            if (!localDirHandle) { showToast('请先打开本地目录', 'info'); return }
            const sel = Array.from(remoteSelected).map(p => remoteFiles.find(f => f.path === p)).filter(Boolean) as SftpFile[]
            if (sel.length) downloadToLocal(sel)
          }}
          disabled={remoteSelected.size === 0 || !localDirHandle}
          title="下载选中文件到本地"
        >
          ↓ 下载
        </button>
      </div>

      {/* ========== 本地面板 ========== */}
      <div className="xftp-panel">
        <div className="xftp-panel-header">
          <span className="xftp-panel-label local">本地磁盘</span>
        </div>
        <div className="xftp-pathbar">
          <button onClick={localGoUp} disabled={!localDirHandle} className="xftp-nav-btn">↑</button>
          <input
            className="xftp-path-input"
            value={localPath}
            readOnly
            placeholder="未选择本地目录"
          />
          <button onClick={openLocalDir} className="xftp-nav-btn" title="选择本地目录">
            {hasFSAccess ? '📂' : '选择'}
          </button>
        </div>
        <div className="xftp-filelist">
          <div className="xftp-filelist-header">
            <span className="col-name">名称</span>
            <span className="col-size">大小</span>
            <span className="col-time">修改时间</span>
          </div>
          <div className="xftp-filelist-body">
            {!localDirHandle && (
              <div className="xftp-empty">
                <div>点击右上角 📂 选择本地目录</div>
                <div className="muted" style={{ marginTop: 8 }}>
                  需要 Chrome / Edge 浏览器
                </div>
              </div>
            )}
            {localDirHandle && localLoading && <div className="xftp-empty">加载中...</div>}
            {localDirHandle && !localLoading && localFiles.length === 0 && <div className="xftp-empty">空目录</div>}
            {localFiles.map((f) => (
              <div
                className={`xftp-file-row ${localSelected.has(f.name) ? 'selected' : ''}`}
                key={f.name}
                onClick={(e) => localSelect(f, e)}
                onDoubleClick={() => localEnter(f)}
              >
                <span className="col-name">
                  <span className="xftp-icon">{f.is_dir ? '📁' : '📄'}</span>
                  {f.name}
                </span>
                <span className="col-size muted">{f.is_dir ? '' : formatSize(f.size)}</span>
                <span className="col-time muted">{formatDate(f.mod_time)}</span>
              </div>
            ))}
          </div>
        </div>
        <div className="xftp-toolbar">
          <button
            onClick={() => {
              const sel = Array.from(localSelected).map(n => localFiles.find(f => f.name === n)).filter(Boolean) as LocalFile[]
              if (sel.length) uploadToRemote(sel)
            }}
            disabled={localSelected.size === 0 || !localDirHandle}
            title="上传到远端"
          >
            ↑ 上传
          </button>
          <button onClick={() => localDirHandle && listLocal(localDirHandle)} disabled={!localDirHandle}>刷新</button>
          <button onClick={openLocalDir}>{localDirHandle ? '切换目录' : '打开目录'}</button>
          <button
            onClick={async () => {
              if (!localDirHandle) return
              const sel = Array.from(localSelected)
              for (const n of sel) { try { await localDirHandle.removeEntry(n, { recursive: true }) } catch {} }
              if (sel.length) { showToast(`删除 ${sel.length} 个`, 'success'); listLocal(localDirHandle) }
            }}
            disabled={localSelected.size === 0}
            className="danger"
          >
            删除
          </button>
          <span className="xftp-count">{localFiles.length} 项</span>
        </div>
      </div>

      {/* ========== 传输状态 ========== */}
      {transfers.length > 0 && (
        <div className="xftp-transfers">
          {transfers.map((t, i) => (
            <span key={i} className={`xftp-transfer-item ${t.status === 'done' ? 'done' : t.status.startsWith('error') ? 'error' : ''}`}>
              {t.name} {t.status}
            </span>
          ))}
        </div>
      )}

      {/* 上传用 input（fallback） */}
      <input type="file" ref={fileInputRef} style={{ display: 'none' }} multiple />
    </div>
  )
}

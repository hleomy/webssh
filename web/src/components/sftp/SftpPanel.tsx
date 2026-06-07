import { useEffect, useRef, useState } from 'react'
import { useApp } from '../../store'
import * as api from '../../utils/api'
import type { SftpFile } from '../../types'
import { formatSize, formatDate, formatMode } from '../../utils/helpers'

interface Props {
  serverId: string
}

export default function SftpPanel({ serverId }: Props) {
  const { showToast } = useApp()
  const [path, setPath] = useState('/')
  const [files, setFiles] = useState<SftpFile[]>([])
  const [loading, setLoading] = useState(false)
  const [editing, setEditing] = useState<{ path: string; content: string; isNew?: boolean } | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const list = async (p: string) => {
    setLoading(true)
    try {
      const r = await api.get<SftpFile[]>(`/servers/${serverId}/sftp/list`, { path: p })
      setFiles(r)
      setPath(p)
    } catch (e: any) {
      showToast(e.message || '列表失败', 'error')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    list('/')
  }, [serverId])

  const goUp = () => {
    if (path === '/') return
    const idx = path.lastIndexOf('/')
    const parent = idx === 0 ? '/' : path.slice(0, idx)
    list(parent)
  }

  const enter = (f: SftpFile) => {
    if (f.is_dir) list(f.path)
    else openEditor(f)
  }

  const remove = async (f: SftpFile) => {
    if (!confirm(`确认删除 ${f.name}？${f.is_dir ? '（目录及子项）' : ''}`)) return
    try {
      await api.del(`/servers/${serverId}/sftp/delete?path=${encodeURIComponent(f.path)}&recursive=${f.is_dir}`)
      showToast('删除成功', 'success')
      list(path)
    } catch (e: any) {
      showToast(e.message || '删除失败', 'error')
    }
  }

  const rename = async (f: SftpFile) => {
    const newName = prompt('新名称', f.name)
    if (!newName || newName === f.name) return
    const parent = f.path.slice(0, f.path.length - f.name.length)
    const newPath = (parent + newName).replace('//', '/')
    try {
      await api.post(`/servers/${serverId}/sftp/rename?old=${encodeURIComponent(f.path)}&new=${encodeURIComponent(newPath)}`)
      showToast('重命名成功', 'success')
      list(path)
    } catch (e: any) {
      showToast(e.message || '重命名失败', 'error')
    }
  }

  const mkdir = async () => {
    const name = prompt('目录名')
    if (!name) return
    const full = (path === '/' ? '' : path) + '/' + name
    try {
      await api.post(`/servers/${serverId}/sftp/mkdir?path=${encodeURIComponent(full)}`)
      showToast('创建成功', 'success')
      list(path)
    } catch (e: any) {
      showToast(e.message || '创建失败', 'error')
    }
  }

  const newFile = async () => {
    const name = prompt('文件名')
    if (!name) return
    const full = (path === '/' ? '' : path) + '/' + name
    setEditing({ path: full, content: '', isNew: true })
  }

  const openEditor = async (f: SftpFile) => {
    try {
      const r = await api.get<{ content: string }>(`/servers/${serverId}/sftp/read?path=${encodeURIComponent(f.path)}`)
      setEditing({ path: f.path, content: r.content })
    } catch (e: any) {
      showToast(e.message || '读取失败', 'error')
    }
  }

  const saveEditor = async () => {
    if (!editing) return
    try {
      const encoded = btoa(unescape(encodeURIComponent(editing.content)))
      await api.post(`/servers/${serverId}/sftp/write`, { path: editing.path, content: encoded })
      showToast('保存成功', 'success')
      setEditing(null)
      list(path)
    } catch (e: any) {
      showToast(e.message || '保存失败', 'error')
    }
  }

  const downloadFile = (f: SftpFile) => {
    if (f.is_dir) return
    const token = localStorage.getItem('webssh_token')
    const url = `/api/servers/${serverId}/sftp/download?path=${encodeURIComponent(f.path)}&token=${token}`
    const a = document.createElement('a')
    a.href = url
    a.download = f.name
    document.body.appendChild(a)
    a.click()
    a.remove()
  }

  const uploadClick = () => fileInputRef.current?.click()

  const onUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const f = e.target.files?.[0]
    if (!f) return
    const form = new FormData()
    form.append('file', f)
    try {
      const remote = (path === '/' ? '' : path) + '/' + f.name
      await api.post(`/servers/${serverId}/sftp/upload?path=${encodeURIComponent(remote)}`, form, {
        headers: { 'Content-Type': 'multipart/form-data' }
      })
      showToast('上传成功', 'success')
      list(path)
    } catch (e: any) {
      showToast(e.message || '上传失败', 'error')
    } finally {
      e.target.value = ''
    }
  }

  if (editing) {
    return (
      <div className="editor">
        <div className="editor-toolbar">
          <span className="sftp-path">{editing.path}</span>
          <button onClick={saveEditor} className="primary">保存</button>
          <button onClick={() => setEditing(null)}>取消</button>
        </div>
        <textarea value={editing.content} onChange={(e) => setEditing({ ...editing, content: e.target.value })} />
      </div>
    )
  }

  return (
    <div className="sftp-panel" style={{ flexDirection: 'column' }}>
      <div className="sftp-toolbar">
        <button onClick={goUp} disabled={path === '/'}>↑</button>
        <span className="sftp-path">{path}</span>
        <button onClick={() => list(path)}>刷新</button>
        <button onClick={mkdir}>新建目录</button>
        <button onClick={newFile}>新建文件</button>
        <button onClick={uploadClick}>上传</button>
        <input
          type="file"
          ref={fileInputRef}
          style={{ display: 'none' }}
          onChange={onUpload}
        />
      </div>
      <div className="sftp-list">
        <div className="sftp-row header">
          <span></span>
          <span>名称</span>
          <span>大小</span>
          <span>权限</span>
          <span>修改时间</span>
        </div>
        {loading && <div className="empty">加载中...</div>}
        {!loading && files.length === 0 && <div className="empty">空目录</div>}
        {files.map((f) => (
          <div className="sftp-row" key={f.path} onDoubleClick={() => enter(f)}>
            <span>{f.is_dir ? '📁' : '📄'}</span>
            <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{f.name}</span>
            <span className="muted">{f.is_dir ? '-' : formatSize(f.size)}</span>
            <span className="muted">{formatMode(f.mode)}</span>
            <span className="muted">{formatDate(f.mod_time)}</span>
            <div style={{ display: 'flex', gap: 4, gridColumn: '1 / -1' }}>
              {!f.is_dir && <button onClick={() => openEditor(f)}>编辑</button>}
              <button onClick={() => downloadFile(f)} disabled={f.is_dir}>下载</button>
              <button onClick={() => rename(f)}>重命名</button>
              <button onClick={() => remove(f)} className="danger">删除</button>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

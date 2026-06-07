import { useState, useMemo } from 'react'
import { useApp } from '../../store'
import type { ServerItem } from '../../types'
import * as api from '../../utils/api'

interface Props {
  onAdd: () => void
  onEdit: (s: ServerItem) => void
}

export default function Sidebar({ onAdd, onEdit }: Props) {
  const { servers, openTab, showToast, removeServer, updateServer } = useApp()
  const [search, setSearch] = useState('')
  const [filter, setFilter] = useState<'all' | 'fav'>('all')

  const grouped = useMemo(() => {
    const filtered = servers.filter((s) => {
      if (filter === 'fav' && !s.is_favorite) return false
      if (search && !`${s.name}${s.host}${s.tags || ''}`.toLowerCase().includes(search.toLowerCase())) return false
      return true
    })
    const groups: Record<string, ServerItem[]> = {}
    for (const s of filtered) {
      const g = s.group || '默认'
      if (!groups[g]) groups[g] = []
      groups[g].push(s)
    }
    return groups
  }, [servers, search, filter])

  const toggleFav = async (s: ServerItem, e: React.MouseEvent) => {
    e.stopPropagation()
    try {
      await api.post(`/servers/${s.id}/favorite`)
      updateServer({ ...s, is_favorite: !s.is_favorite })
    } catch (e: any) {
      showToast(e.message || '操作失败', 'error')
    }
  }

  const del = async (s: ServerItem, e: React.MouseEvent) => {
    e.stopPropagation()
    if (!confirm(`确认删除服务器「${s.name}」？`)) return
    try {
      await api.del(`/servers/${s.id}`)
      removeServer(s.id)
      showToast('已删除', 'success')
    } catch (e: any) {
      showToast(e.message || '删除失败', 'error')
    }
  }

  const connect = async (s: ServerItem) => {
    openTab(s, 'terminal')
    try {
      const r = await api.post<{ session_id: string }>(`/servers/${s.id}/connect`, {
        cols: 80,
        rows: 24,
        term: 'xterm-256color'
      })
      useApp.getState().updateTab(`${s.id}-terminal`, { sessionId: r.session_id, isConnecting: false })
    } catch (e: any) {
      showToast(e.message || '连接失败', 'error')
    }
  }

  return (
    <div className="sidebar">
      <div className="sidebar-header">
        <h2>服务器列表</h2>
        <div style={{ display: 'flex', gap: 4 }}>
          <button onClick={() => setFilter(filter === 'all' ? 'fav' : 'all')} title="收藏筛选">
            {filter === 'fav' ? '★' : '☆'}
          </button>
          <button onClick={onAdd} title="新增">+</button>
        </div>
      </div>
      <div style={{ padding: 8 }}>
        <input
          style={{ width: '100%' }}
          placeholder="搜索..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>
      <div className="sidebar-body">
        {Object.keys(grouped).length === 0 && (
          <div className="empty">
            {servers.length === 0 ? '暂无服务器，点击 + 添加' : '无匹配结果'}
          </div>
        )}
        {Object.entries(grouped).map(([group, items]) => (
          <div className="server-group" key={group}>
            <div className="server-group-title">{group} ({items.length})</div>
            {items.map((s) => (
              <div
                className="server-item"
                key={s.id}
                onClick={() => connect(s)}
                onContextMenu={(e) => {
                  e.preventDefault()
                  const action = prompt('操作: 1=终端 2=SFTP 3=端口转发 4=编辑 5=删除', '1')
                  if (action === '1') connect(s)
                  else if (action === '2') openTab(s, 'sftp')
                  else if (action === '3') openTab(s, 'portforward')
                  else if (action === '4') onEdit(s)
                  else if (action === '5') del(s, e as any)
                }}
                title={`${s.username}@${s.host}:${s.port}`}
              >
                <span className="fav" onClick={(e) => toggleFav(s, e)}>
                  {s.is_favorite ? '★' : '☆'}
                </span>
                <span className="name">{s.name}</span>
                <div className="actions">
                  <button onClick={(e) => { e.stopPropagation(); openTab(s, 'sftp') }} title="SFTP">S</button>
                  <button onClick={(e) => { e.stopPropagation(); onEdit(s) }} title="编辑">✎</button>
                  <button onClick={(e) => del(s, e)} className="danger" title="删除">✕</button>
                </div>
              </div>
            ))}
          </div>
        ))}
      </div>
    </div>
  )
}

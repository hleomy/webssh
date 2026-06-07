import { useEffect, useState } from 'react'
import * as api from '../../utils/api'
import { useApp } from '../../store'
import type { PortForward, ServerItem } from '../../types'

interface Props {
  serverId: string
  onClose: () => void
}

export default function PortForwardModal({ serverId, onClose }: Props) {
  const { showToast } = useApp()
  const [list, setList] = useState<PortForward[]>([])
  const [loading, setLoading] = useState(false)
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState({
    name: '',
    type: 'local' as 'local' | 'remote',
    local_host: '127.0.0.1',
    local_port: 8080,
    remote_host: '127.0.0.1',
    remote_port: 80
  })

  const load = async () => {
    setLoading(true)
    try {
      const r = await api.get<PortForward[]>('/port-forwards')
      setList(r.filter((f) => f.server_id === serverId))
    } catch (e: any) {
      showToast(e.message || '加载失败', 'error')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [serverId])

  const create = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await api.post('/port-forwards', { ...form, server_id: serverId })
      showToast('创建成功', 'success')
      setShowForm(false)
      load()
    } catch (e: any) {
      showToast(e.message || '创建失败', 'error')
    }
  }

  const start = async (id: string) => {
    try {
      await api.post(`/port-forwards/${id}/start`)
      showToast('已启动', 'success')
      load()
    } catch (e: any) {
      showToast(e.message || '启动失败', 'error')
    }
  }

  const stop = async (id: string) => {
    try {
      await api.post(`/port-forwards/${id}/stop`)
      showToast('已停止', 'success')
      load()
    } catch (e: any) {
      showToast(e.message || '停止失败', 'error')
    }
  }

  const remove = async (id: string) => {
    if (!confirm('确认删除？')) return
    try {
      await api.del(`/port-forwards/${id}`)
      showToast('已删除', 'success')
      load()
    } catch (e: any) {
      showToast(e.message || '删除失败', 'error')
    }
  }

  return (
    <div className="modal-mask" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()} style={{ width: 640 }}>
        <div className="modal-header">
          <span>端口转发</span>
          <button className="ghost" onClick={onClose}>✕</button>
        </div>
        <div className="modal-body">
          <div style={{ marginBottom: 12, display: 'flex', justifyContent: 'space-between' }}>
            <span className="muted">本服务器转发规则</span>
            <button onClick={() => setShowForm(!showForm)}>{showForm ? '取消' : '新建'}</button>
          </div>
          {showForm && (
            <form onSubmit={create} style={{ background: 'var(--bg-tertiary)', padding: 12, borderRadius: 4, marginBottom: 12 }}>
              <div className="form-grid">
                <div className="form-row">
                  <label>名称</label>
                  <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
                </div>
                <div className="form-row">
                  <label>类型</label>
                  <select value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value as any })}>
                    <option value="local">本地转发</option>
                    <option value="remote">远程转发</option>
                  </select>
                </div>
              </div>
              <div className="form-grid">
                <div className="form-row">
                  <label>本地地址</label>
                  <input value={form.local_host} onChange={(e) => setForm({ ...form, local_host: e.target.value })} />
                </div>
                <div className="form-row">
                  <label>本地端口</label>
                  <input type="number" value={form.local_port} onChange={(e) => setForm({ ...form, local_port: parseInt(e.target.value) || 0 })} required />
                </div>
              </div>
              <div className="form-grid">
                <div className="form-row">
                  <label>远端地址</label>
                  <input value={form.remote_host} onChange={(e) => setForm({ ...form, remote_host: e.target.value })} required />
                </div>
                <div className="form-row">
                  <label>远端端口</label>
                  <input type="number" value={form.remote_port} onChange={(e) => setForm({ ...form, remote_port: parseInt(e.target.value) || 0 })} required />
                </div>
              </div>
              <button type="submit" className="primary">保存</button>
            </form>
          )}
          {loading && <div className="empty">加载中...</div>}
          {!loading && list.length === 0 && <div className="empty">暂无转发</div>}
          {list.map((f) => (
            <div key={f.id} className="sftp-row" style={{ gridTemplateColumns: '1fr 100px 100px 1fr 200px' }}>
              <span>
                <b>{f.name}</b> <span className="muted">({f.type === 'local' ? '本地' : '远程'})</span>
              </span>
              <span className="muted">{f.local_host}:{f.local_port}</span>
              <span className="muted">{f.remote_host}:{f.remote_port}</span>
              <span className="muted">{f.status === 'running' ? '🟢 运行中' : f.status === 'error' ? '🔴 错误' : '⚪ 已停止'}</span>
              <div style={{ display: 'flex', gap: 4 }}>
                {f.status === 'running' ? (
                  <button onClick={() => stop(f.id)}>停止</button>
                ) : (
                  <button onClick={() => start(f.id)} className="primary">启动</button>
                )}
                <button onClick={() => remove(f.id)} className="danger">删除</button>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

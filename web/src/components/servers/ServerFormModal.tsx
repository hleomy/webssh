import { useEffect, useState } from 'react'
import * as api from '../../utils/api'
import type { ServerItem } from '../../types'
import { useApp } from '../../store'

interface Props {
  server: ServerItem | null
  onClose: () => void
}

export default function ServerFormModal({ server, onClose }: Props) {
  const { addServer, updateServer, showToast } = useApp()
  const [form, setForm] = useState({
    name: server?.name || '',
    host: server?.host || '',
    port: server?.port || 22,
    username: server?.username || 'root',
    auth_type: (server?.auth_type || 'password') as 'password' | 'key',
    password: '',
    private_key: '',
    passphrase: '',
    description: server?.description || '',
    group: server?.group || '',
    tags: server?.tags || '',
    is_favorite: server?.is_favorite || false
  })
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (server) {
      setForm((f) => ({
        ...f,
        name: server.name,
        host: server.host,
        port: server.port,
        username: server.username,
        auth_type: server.auth_type,
        description: server.description || '',
        group: server.group || '',
        tags: server.tags || '',
        is_favorite: server.is_favorite
      }))
    }
  }, [server])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    try {
      if (server) {
        const r = await api.put<ServerItem>(`/servers/${server.id}`, form)
        updateServer(r)
        showToast('更新成功', 'success')
      } else {
        const r = await api.post<ServerItem>('/servers', form)
        addServer(r)
        showToast('创建成功', 'success')
      }
      onClose()
    } catch (e: any) {
      showToast(e.message || '保存失败', 'error')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="modal-mask" onClick={onClose}>
      <form className="modal" onClick={(e) => e.stopPropagation()} onSubmit={submit} style={{ width: 560 }}>
        <div className="modal-header">
          <span>{server ? '编辑服务器' : '新建服务器'}</span>
          <button className="ghost" onClick={onClose}>✕</button>
        </div>
        <div className="modal-body">
          <div className="form-grid">
            <div className="form-row">
              <label>名称 *</label>
              <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
            </div>
            <div className="form-row">
              <label>分组</label>
              <input value={form.group} onChange={(e) => setForm({ ...form, group: e.target.value })} placeholder="如：生产、测试" />
            </div>
          </div>
          <div className="form-grid">
            <div className="form-row">
              <label>主机 *</label>
              <input value={form.host} onChange={(e) => setForm({ ...form, host: e.target.value })} placeholder="IP 或域名" required />
            </div>
            <div className="form-row">
              <label>端口</label>
              <input type="number" value={form.port} onChange={(e) => setForm({ ...form, port: parseInt(e.target.value) || 22 })} />
            </div>
          </div>
          <div className="form-row">
            <label>用户名 *</label>
            <input value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} required />
          </div>
          <div className="form-row">
            <label>认证方式</label>
            <select value={form.auth_type} onChange={(e) => setForm({ ...form, auth_type: e.target.value as any })}>
              <option value="password">密码</option>
              <option value="key">私钥</option>
            </select>
          </div>
          {form.auth_type === 'password' ? (
            <div className="form-row">
              <label>密码 {server ? '(留空则不修改)' : '*'}</label>
              <input type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} required={!server} />
            </div>
          ) : (
            <>
              <div className="form-row">
                <label>私钥 {server ? '(留空则不修改)' : '*'}</label>
                <textarea
                  className="code"
                  value={form.private_key}
                  onChange={(e) => setForm({ ...form, private_key: e.target.value })}
                  placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                  required={!server}
                />
              </div>
              <div className="form-row">
                <label>私钥密码（可选）</label>
                <input type="password" value={form.passphrase} onChange={(e) => setForm({ ...form, passphrase: e.target.value })} />
              </div>
            </>
          )}
          <div className="form-row">
            <label>标签</label>
            <input value={form.tags} onChange={(e) => setForm({ ...form, tags: e.target.value })} placeholder="逗号分隔" />
          </div>
          <div className="form-row">
            <label>描述</label>
            <textarea value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} style={{ minHeight: 60 }} />
          </div>
          <div className="form-row">
            <label>
              <input
                type="checkbox"
                checked={form.is_favorite}
                onChange={(e) => setForm({ ...form, is_favorite: e.target.checked })}
                style={{ width: 'auto', marginRight: 6 }}
              />
              收藏
            </label>
          </div>
        </div>
        <div className="modal-footer">
          <button type="button" onClick={onClose}>取消</button>
          <button type="submit" className="primary" disabled={loading}>
            {loading ? '保存中...' : '保存'}
          </button>
        </div>
      </form>
    </div>
  )
}

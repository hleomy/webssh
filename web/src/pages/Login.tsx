import { FormEvent, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import * as api from '../utils/api'
import { useApp } from '../store'

export default function Login() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [err, setErr] = useState('')
  const { setToken, setUser, showToast } = useApp()
  const nav = useNavigate()

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setErr('')
    setLoading(true)
    try {
      const r = await api.post<{ token: string; user: any }>('/auth/login', {
        username,
        password
      })
      setToken(r.token)
      setUser(r.user)
      showToast('登录成功', 'success')
      nav('/')
    } catch (e: any) {
      setErr(e.message || '登录失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="auth-page">
      <form className="auth-card" onSubmit={submit}>
        <h1>WebSSH 登录</h1>
        <div className="form-row">
          <label>用户名</label>
          <input
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="请输入用户名"
            autoFocus
            required
          />
        </div>
        <div className="form-row">
          <label>密码</label>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="请输入密码"
            required
          />
        </div>
        {err && <div className="form-row" style={{ color: 'var(--error)', fontSize: 12 }}>{err}</div>}
        <div className="actions">
          <button type="submit" className="primary" disabled={loading}>
            {loading ? '登录中...' : '登录'}
          </button>
          <div className="switch" onClick={() => nav('/register')}>
            还没有账号？立即注册
          </div>
        </div>
      </form>
    </div>
  )
}

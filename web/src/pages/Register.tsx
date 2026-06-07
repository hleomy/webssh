import { FormEvent, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import * as api from '../utils/api'
import { useApp } from '../store'

export default function Register() {
  const [username, setUsername] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [loading, setLoading] = useState(false)
  const [err, setErr] = useState('')
  const { showToast, setInitialized } = useApp()
  const nav = useNavigate()

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setErr('')
    if (password !== confirm) {
      setErr('两次密码不一致')
      return
    }
    if (password.length < 6) {
      setErr('密码至少 6 位')
      return
    }
    setLoading(true)
    try {
      await api.post('/auth/register', { username, email, password })
      showToast('注册成功，请登录', 'success')
      setInitialized(true)
      nav('/login')
    } catch (e: any) {
      setErr(e.message || '注册失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="auth-page">
      <form className="auth-card" onSubmit={submit}>
        <h1>创建管理员账号</h1>
        <div style={{ fontSize: 12, color: 'var(--text-secondary)', marginBottom: 16, textAlign: 'center' }}>
          首次使用需注册管理员账号
        </div>
        <div className="form-row">
          <label>用户名</label>
          <input
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="3-32 位字符"
            minLength={3}
            maxLength={32}
            autoFocus
            required
          />
        </div>
        <div className="form-row">
          <label>邮箱</label>
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="用于找回密码"
            required
          />
        </div>
        <div className="form-row">
          <label>密码</label>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="至少 6 位"
            minLength={6}
            required
          />
        </div>
        <div className="form-row">
          <label>确认密码</label>
          <input
            type="password"
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            placeholder="再次输入密码"
            minLength={6}
            required
          />
        </div>
        {err && <div className="form-row" style={{ color: 'var(--error)', fontSize: 12 }}>{err}</div>}
        <div className="actions">
          <button type="submit" className="primary" disabled={loading}>
            {loading ? '注册中...' : '注册并登录'}
          </button>
          <div className="switch" onClick={() => nav('/login')}>
            已有账号？返回登录
          </div>
        </div>
      </form>
    </div>
  )
}

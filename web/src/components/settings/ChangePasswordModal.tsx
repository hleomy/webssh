import { useState } from 'react'
import * as api from '../../utils/api'
import { useApp } from '../../store'

interface Props {
  onClose: () => void
}

export default function ChangePasswordModal({ onClose }: Props) {
  const { showToast } = useApp()
  const [oldPassword, setOld] = useState('')
  const [newPassword, setNew] = useState('')
  const [confirm, setConfirm] = useState('')
  const [loading, setLoading] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (newPassword !== confirm) {
      showToast('两次密码不一致', 'error')
      return
    }
    if (newPassword.length < 6) {
      showToast('密码至少 6 位', 'error')
      return
    }
    setLoading(true)
    try {
      await api.post('/auth/change-password', { old_password: oldPassword, new_password: newPassword })
      showToast('密码已修改', 'success')
      onClose()
    } catch (e: any) {
      showToast(e.message || '修改失败', 'error')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="modal-mask" onClick={onClose}>
      <form className="modal" onClick={(e) => e.stopPropagation()} onSubmit={submit}>
        <div className="modal-header">
          <span>修改密码</span>
          <button type="button" className="ghost" onClick={onClose}>✕</button>
        </div>
        <div className="modal-body">
          <div className="form-row">
            <label>当前密码</label>
            <input type="password" value={oldPassword} onChange={(e) => setOld(e.target.value)} required autoFocus />
          </div>
          <div className="form-row">
            <label>新密码</label>
            <input type="password" value={newPassword} onChange={(e) => setNew(e.target.value)} minLength={6} required />
          </div>
          <div className="form-row">
            <label>确认新密码</label>
            <input type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} minLength={6} required />
          </div>
        </div>
        <div className="modal-footer">
          <button type="button" onClick={onClose}>取消</button>
          <button type="submit" className="primary" disabled={loading}>
            {loading ? '提交中...' : '提交'}
          </button>
        </div>
      </form>
    </div>
  )
}

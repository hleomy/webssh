import { useApp } from '../../store'

export default function ToastContainer() {
  const { toast } = useApp()
  if (!toast) return null
  return <div className={`toast ${toast.type}`}>{toast.msg}</div>
}

import { useApp } from '../../store'

export default function StatusBar() {
  const { user, settings, tabs, activeTabId, servers } = useApp()
  const active = tabs.find((t) => t.id === activeTabId)

  return (
    <div className="status-bar">
      <span>用户: {user?.username}</span>
      <span>主题: {settings.theme === 'dark' ? '深色' : '浅色'}</span>
      <span>服务器: {servers.length}</span>
      {active && <span>当前: {active.title}</span>}
      <span style={{ marginLeft: 'auto' }}>WebSSH v1.0</span>
    </div>
  )
}

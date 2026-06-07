import { useEffect, useState } from 'react'
import { useApp } from '../store'
import Sidebar from '../components/layout/Sidebar'
import TabBar from '../components/layout/TabBar'
import StatusBar from '../components/layout/StatusBar'
import ToastContainer from '../components/layout/ToastContainer'
import ServerFormModal from '../components/servers/ServerFormModal'
import SettingsModal from '../components/settings/SettingsModal'
import ChangePasswordModal from '../components/settings/ChangePasswordModal'
import PortForwardModal from '../components/portforward/PortForwardModal'
import TerminalPanel from '../components/terminal/TerminalPanel'
import SftpPanel from '../components/sftp/SftpPanel'
import type { ServerItem } from '../types'

export default function Main() {
  const { user, settings, tabs, activeTabId, servers, loadServers, openTab } = useApp()
  const [editing, setEditing] = useState<ServerItem | null>(null)
  const [showForm, setShowForm] = useState(false)
  const [showSettings, setShowSettings] = useState(false)
  const [showChangePwd, setShowChangePwd] = useState(false)
  const [showPortForward, setShowPortForward] = useState(false)

  useEffect(() => {
    loadServers().catch(() => {})
  }, [])

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', settings.theme)
  }, [settings.theme])

  const activeTab = tabs.find((t) => t.id === activeTabId)

  return (
    <div className="app">
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '8px 16px', background: 'var(--bg-secondary)', borderBottom: '1px solid var(--border)' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <span style={{ color: 'var(--accent)', fontWeight: 700, fontSize: 16 }}>WebSSH</span>
          <span className="muted">欢迎, {user?.username}</span>
        </div>
        <div style={{ display: 'flex', gap: 6 }}>
          <button onClick={() => { setEditing(null); setShowForm(true) }}>添加服务器</button>
          <button onClick={() => setShowSettings(true)}>设置</button>
        </div>
      </div>
      <div className="app-body">
        <Sidebar onAdd={() => { setEditing(null); setShowForm(true) }} onEdit={(s) => { setEditing(s); setShowForm(true) }} />
        <div className="main-area">
          <TabBar />
          <div className="tab-content">
            {tabs.length === 0 ? (
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', flexDirection: 'column', gap: 12, color: 'var(--text-muted)' }}>
                <div style={{ fontSize: 18 }}>未打开任何会话</div>
                <div className="muted">从左侧选择服务器进行连接，或点击「添加服务器」</div>
              </div>
            ) : activeTab ? (
              activeTab.type === 'terminal' ? (
                activeTab.sessionId ? (
                  <TerminalPanel
                    serverId={activeTab.serverId}
                    sessionId={activeTab.sessionId}
                    tabId={activeTab.id}
                    onClose={() => {}}
                  />
                ) : (
                  <div className="empty" style={{ paddingTop: 80 }}>正在建立 SSH 连接...</div>
                )
              ) : activeTab.type === 'sftp' ? (
                <SftpPanel serverId={activeTab.serverId} />
              ) : activeTab.type === 'portforward' ? (
                <div style={{ padding: 16 }}>
                  <button className="primary" onClick={() => setShowPortForward(true)}>管理端口转发</button>
                  <PortForwardModalWrapper visible={showPortForward} serverId={activeTab.serverId} onClose={() => setShowPortForward(false)} />
                </div>
              ) : null
            ) : (
              <div className="empty" style={{ paddingTop: 80 }}>请选择一个标签</div>
            )}
          </div>
          <StatusBar />
        </div>
      </div>
      {showForm && <ServerFormModal server={editing} onClose={() => { setShowForm(false); setEditing(null) }} />}
      {showSettings && <SettingsModal onClose={() => setShowSettings(false)} onChangePassword={() => { setShowSettings(false); setShowChangePwd(true) }} />}
      {showChangePwd && <ChangePasswordModal onClose={() => setShowChangePwd(false)} />}
      <ToastContainer />
    </div>
  )
}

function PortForwardModalWrapper({ visible, serverId, onClose }: { visible: boolean; serverId: string; onClose: () => void }) {
  if (!visible) return null
  return <PortForwardModal serverId={serverId} onClose={onClose} />
}

import { useApp } from '../../store'

export default function TabBar() {
  const { tabs, activeTabId, setActiveTab, closeTab } = useApp()

  if (tabs.length === 0) return null

  return (
    <div className="tab-bar">
      {tabs.map((t) => (
        <div
          key={t.id}
          className={`tab ${activeTabId === t.id ? 'active' : ''}`}
          onClick={() => setActiveTab(t.id)}
        >
          <span>{t.isConnecting ? '⏳ ' : ''}{t.title}</span>
          <span className="close" onClick={(e) => { e.stopPropagation(); closeTab(t.id) }}>✕</span>
        </div>
      ))}
    </div>
  )
}

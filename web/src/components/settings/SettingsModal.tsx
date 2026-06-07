import { useApp } from '../../store'

interface Props {
  onClose: () => void
  onChangePassword: () => void
}

export default function SettingsModal({ onClose, onChangePassword }: Props) {
  const { settings, setSettings, user, logout } = useApp()

  return (
    <div className="modal-mask" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <span>设置</span>
          <button className="ghost" onClick={onClose}>✕</button>
        </div>
        <div className="modal-body">
          <div className="form-row">
            <label>主题</label>
            <select
              value={settings.theme}
              onChange={(e) => setSettings({ theme: e.target.value as any })}
            >
              <option value="dark">深色</option>
              <option value="light">浅色</option>
            </select>
          </div>
          <div className="form-row">
            <label>字体大小: {settings.fontSize}px</label>
            <input
              type="range"
              min={10}
              max={24}
              value={settings.fontSize}
              onChange={(e) => setSettings({ fontSize: parseInt(e.target.value) })}
            />
          </div>
          <div className="form-row">
            <label>字体</label>
            <select
              value={settings.fontFamily}
              onChange={(e) => setSettings({ fontFamily: e.target.value })}
            >
              <option value='Menlo, Consolas, "Courier New", monospace'>Menlo / Consolas</option>
              <option value='"Cascadia Code", "Cascadia Mono", monospace'>Cascadia Code</option>
              <option value='"Source Code Pro", monospace'>Source Code Pro</option>
              <option value='"JetBrains Mono", monospace'>JetBrains Mono</option>
              <option value='"Courier New", monospace'>Courier New</option>
            </select>
          </div>
          <div className="form-row">
            <label>
              <input
                type="checkbox"
                checked={settings.cursorBlink}
                onChange={(e) => setSettings({ cursorBlink: e.target.checked })}
                style={{ width: 'auto', marginRight: 6 }}
              />
              光标闪烁
            </label>
          </div>
          <div className="divider" />
          <div className="form-row">
            <label>账号信息</label>
            <div className="muted">用户名: {user?.username} | 邮箱: {user?.email} | 角色: {user?.role}</div>
          </div>
        </div>
        <div className="modal-footer">
          <button onClick={onChangePassword}>修改密码</button>
          <button onClick={logout} className="danger">退出登录</button>
          <button onClick={onClose}>关闭</button>
        </div>
      </div>
    </div>
  )
}

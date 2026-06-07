import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { ServerItem, TerminalTab, AppSettings, User } from '../types'
import * as api from '../utils/api'
import { clearToken, getToken, setToken } from '../utils/api'

interface AppState {
  user: User | null
  token: string | null
  initialized: boolean
  servers: ServerItem[]
  tabs: TerminalTab[]
  activeTabId: string | null
  settings: AppSettings
  toast: { msg: string; type: 'success' | 'error' | 'info' } | null

  setUser: (u: User | null) => void
  setToken: (t: string | null) => void
  setInitialized: (v: boolean) => void
  loadServers: () => Promise<void>
  addServer: (s: ServerItem) => void
  updateServer: (s: ServerItem) => void
  removeServer: (id: string) => void
  openTab: (server: ServerItem, type?: 'terminal' | 'sftp' | 'portforward') => void
  closeTab: (id: string) => void
  setActiveTab: (id: string) => void
  updateTab: (id: string, patch: Partial<TerminalTab>) => void
  setSettings: (patch: Partial<AppSettings>) => Promise<void>
  showToast: (msg: string, type?: 'success' | 'error' | 'info') => void
  logout: () => void
}

const defaultSettings: AppSettings = {
  theme: 'dark',
  fontSize: 14,
  fontFamily: 'Menlo, Consolas, "Courier New", monospace',
  cursorBlink: true
}

export const useApp = create<AppState>()(
  persist(
    (set, get) => ({
      user: null,
      token: getToken(),
      initialized: false,
      servers: [],
      tabs: [],
      activeTabId: null,
      settings: defaultSettings,
      toast: null,

      setUser: (user) => set({ user }),
      setToken: (token) => {
        if (token) setToken(token)
        else clearToken()
        set({ token })
      },
      setInitialized: (v) => set({ initialized: v }),

      loadServers: async () => {
        const list = await api.get<ServerItem[]>('/servers')
        set({ servers: list })
      },
      addServer: (s) => set({ servers: [...get().servers, s] }),
      updateServer: (s) =>
        set({ servers: get().servers.map((x) => (x.id === s.id ? s : x)) }),
      removeServer: (id) => set({ servers: get().servers.filter((x) => x.id !== id) }),

      openTab: (server, type = 'terminal') => {
        const id = `${server.id}-${type}`
        const existing = get().tabs.find((t) => t.id === id)
        if (existing) {
          set({ activeTabId: id })
          return
        }
        const tab: TerminalTab = {
          id,
          title: `${server.name} (${type === 'terminal' ? '终端' : type === 'sftp' ? 'SFTP' : '转发'})`,
          serverId: server.id,
          type,
          isConnecting: type === 'terminal'
        }
        set({ tabs: [...get().tabs, tab], activeTabId: id })
      },

      closeTab: (id) => {
        const tabs = get().tabs.filter((t) => t.id !== id)
        let active = get().activeTabId
        if (active === id) active = tabs.length > 0 ? tabs[tabs.length - 1].id : null
        set({ tabs, activeTabId: active })
      },

      setActiveTab: (id) => set({ activeTabId: id }),

      updateTab: (id, patch) =>
        set({ tabs: get().tabs.map((t) => (t.id === id ? { ...t, ...patch } : t)) }),

      setSettings: async (patch) => {
        const next = { ...get().settings, ...patch }
        set({ settings: next })
        try {
          await api.post('/settings', { key: 'app_settings', value: JSON.stringify(next) })
        } catch {
          // ignore - will retry on next change
        }
      },

      showToast: (msg, type = 'info') => {
        set({ toast: { msg, type } })
        setTimeout(() => {
          if (get().toast?.msg === msg) set({ toast: null })
        }, 3000)
      },

      logout: () => {
        clearToken()
        set({ user: null, token: null, servers: [], tabs: [], activeTabId: null })
      }
    }),
    {
      name: 'webssh-app',
      partialize: (s) => ({ settings: s.settings })
    }
  )
)

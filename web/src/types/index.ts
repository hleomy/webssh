export interface User {
  id: string
  username: string
  email: string
  role: string
  last_login_at?: string
}

export interface ServerItem {
  id: string
  user_id: string
  name: string
  host: string
  port: number
  username: string
  auth_type: 'password' | 'key'
  description?: string
  group?: string
  tags?: string
  is_favorite: boolean
  created_at: string
  updated_at: string
  last_connect_at?: string
}

export interface SftpFile {
  name: string
  path: string
  size: number
  mode: string
  is_dir: boolean
  mod_time: string
}

export interface PortForward {
  id: string
  name: string
  server_id: string
  type: 'local' | 'remote'
  local_host: string
  local_port: number
  remote_host: string
  remote_port: number
  status: 'stopped' | 'running' | 'error'
  created_at: string
  updated_at: string
}

export interface SessionInfo {
  id: string
  user_id: string
  server_id: string
  name: string
  status: string
  cols: number
  rows: number
  created_at: string
  closed_at?: string
}

export interface TerminalTab {
  id: string
  title: string
  serverId: string
  sessionId?: string
  type: 'terminal' | 'sftp' | 'portforward'
  isConnecting?: boolean
}

export interface AppSettings {
  theme: 'dark' | 'light'
  fontSize: number
  fontFamily: string
  cursorBlink: boolean
}

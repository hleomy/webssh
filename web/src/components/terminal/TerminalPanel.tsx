import { useEffect, useRef } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { useApp } from '../../store'
import { getToken } from '../../utils/api'

interface Props {
  serverId: string
  sessionId: string
  tabId: string
  onClose: () => void
}

export default function TerminalPanel({ serverId, sessionId, tabId, onClose }: Props) {
  const containerRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const fitRef = useRef<FitAddon | null>(null)
  const { settings, updateTab, showToast } = useApp()

  useEffect(() => {
    if (!containerRef.current) return

    const term = new Terminal({
      fontSize: settings.fontSize,
      fontFamily: settings.fontFamily,
      cursorBlink: settings.cursorBlink,
      theme: settings.theme === 'light'
        ? { background: '#ffffff', foreground: '#1a1a1a' }
        : { background: '#1e1e1e', foreground: '#cccccc' },
      allowProposedApi: true,
      scrollback: 5000
    })
    const fit = new FitAddon()
    term.loadAddon(fit)
    term.loadAddon(new WebLinksAddon())
    term.open(containerRef.current)
    fit.fit()
    termRef.current = term
    fitRef.current = fit

    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
    const ws = new WebSocket(
      `${proto}//${location.host}/ws/ssh/${sessionId}?token=${getToken()}`
    )
    wsRef.current = ws

    let binaryData = ''
    let pendingData = false

    ws.onopen = () => {
      term.writeln('\x1b[32m[*] 已连接到 WebSocket，正在建立 SSH...\x1b[0m')
      const cols = term.cols
      const rows = term.rows
      ws.send(JSON.stringify({ type: 'resize', cols, rows, term: 'xterm-256color' }))
    }

    ws.onmessage = (evt) => {
      try {
        const msg = JSON.parse(evt.data)
        if (msg.type === 'data') {
          const decoded = atob(msg.data)
          binaryData += decoded
          pendingData = true
          scheduleFlush()
        } else if (msg.type === 'error') {
          term.writeln(`\r\n\x1b[31m[!] 错误: ${msg.message}\x1b[0m`)
        } else if (msg.type === 'pong') {
          // ignore
        }
      } catch (e) {
        console.error('WS message parse error', e)
      }
    }

    let flushTimer: any = null
    const scheduleFlush = () => {
      if (flushTimer) return
      flushTimer = setTimeout(() => {
        if (pendingData) {
          term.write(binaryData)
          binaryData = ''
          pendingData = false
        }
        flushTimer = null
      }, 16)
    }

    ws.onerror = () => {
      term.writeln('\r\n\x1b[31m[!] WebSocket 连接错误\x1b[0m')
    }
    ws.onclose = () => {
      term.writeln('\r\n\x1b[33m[*] 连接已关闭\x1b[0m')
      updateTab(tabId, { sessionId: undefined, isConnecting: false })
    }

    term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        const encoded = btoa(unescape(encodeURIComponent(data)))
        ws.send(JSON.stringify({ type: 'data', data: encoded }))
      }
    })

    const resizeObserver = new ResizeObserver(() => {
      try {
        fit.fit()
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({
            type: 'resize',
            cols: term.cols,
            rows: term.rows
          }))
        }
      } catch (e) {
        // ignore
      }
    })
    resizeObserver.observe(containerRef.current)

    const onKeyResize = () => {
      try {
        fit.fit()
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({
            type: 'resize',
            cols: term.cols,
            rows: term.rows
          }))
        }
      } catch {}
    }
    window.addEventListener('resize', onKeyResize)

    return () => {
      window.removeEventListener('resize', onKeyResize)
      resizeObserver.disconnect()
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'close' }))
        ws.close()
      }
      term.dispose()
    }
  }, [sessionId, tabId])

  return <div ref={containerRef} className="terminal-host" />
}

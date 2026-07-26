import { useEffect, useRef } from 'react'
import type { WsEvent } from './types'

const WS_BASE = import.meta.env.VITE_WS_BASE ?? 'ws://localhost:8081'

export function useProxyEvents(onEvent: (event: WsEvent) => void) {
  const handlerRef = useRef(onEvent)
  handlerRef.current = onEvent

  useEffect(() => {
    let socket: WebSocket | null = null
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null
    let closedByEffect = false

    function connect() {
      socket = new WebSocket(`${WS_BASE}/ws`)

      socket.onmessage = (msg) => {
        try {
          const event = JSON.parse(msg.data) as WsEvent
          handlerRef.current(event)
        } catch {
          // ignore malformed frames
        }
      }

      socket.onclose = () => {
        if (closedByEffect) return
        reconnectTimer = setTimeout(connect, 1500)
      }
    }

    connect()

    return () => {
      closedByEffect = true
      if (reconnectTimer) clearTimeout(reconnectTimer)
      socket?.close()
    }
  }, [])
}

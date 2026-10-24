import { useEffect, useRef, useState } from 'react'
import { openEventStream } from '../api/events'
import type { EventItem } from '../api/types'

const BASE_DELAY_MS = 1000
const MAX_DELAY_MS = 15000

/**
 * Escucha el stream de eventos con reconexión automática (1 s, 2 s, 4 s… máximo 15 s),
 * ignora ids ya entregados y cierra la conexión al desmontar.
 */
export function useEventStream(onEvent: (event: EventItem) => void): {
  connected: boolean
} {
  const [connected, setConnected] = useState(false)
  const onEventRef = useRef(onEvent)

  useEffect(() => {
    onEventRef.current = onEvent
  })

  useEffect(() => {
    let closed = false
    let attempt = 0
    let retryTimer: ReturnType<typeof setTimeout> | undefined
    let closeStream: (() => void) | undefined
    const seen = new Set<number>()

    function connect() {
      retryTimer = undefined
      closeStream = openEventStream({
        onOpen: () => {
          attempt = 0
          setConnected(true)
        },
        onEvent: (event) => {
          if (seen.has(event.id)) return
          seen.add(event.id)
          onEventRef.current(event)
        },
        onError: () => {
          closeStream?.()
          closeStream = undefined
          setConnected(false)
          if (closed || retryTimer !== undefined) return
          const delay = Math.min(BASE_DELAY_MS * 2 ** attempt, MAX_DELAY_MS)
          attempt += 1
          retryTimer = setTimeout(connect, delay)
        },
      })
    }

    connect()

    return () => {
      closed = true
      clearTimeout(retryTimer)
      closeStream?.()
    }
  }, [])

  return { connected }
}

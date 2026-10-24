import { normalizeEvent } from '../lib/events'
import { API_BASE, request } from './client'
import { mockGetEvents, mockOpenEventStream } from './mock'
import type { EventItem } from './types'

const useMock = import.meta.env.VITE_USE_MOCK === 'true'

// Rutas relativas a la base "/api" que añade client.ts. Contrato confirmado por P1.
export const EVENT_ENDPOINTS = {
  // GET → { ok: true, events: [...] }, del más reciente al más antiguo. Admite ?limit (máx. 200) y ?before=<id>.
  list: '/events',
  // Server-Sent Events: eventos con nombre "evento" cuyo data es un evento en JSON.
  stream: '/events/stream',
} as const

const INITIAL_EVENTS_LIMIT = 50

/** Nombre del evento SSE que envía el backend (`event: evento`). */
const STREAM_EVENT_NAME = 'evento'

export type EventStreamHandlers = {
  onOpen: () => void
  onEvent: (event: EventItem) => void
  /** La conexión falló o se cerró; quien llama decide si reconectar. */
  onError: () => void
}

export async function getEvents(): Promise<EventItem[]> {
  const response = useMock
    ? await mockGetEvents()
    : await request<unknown>(`${EVENT_ENDPOINTS.list}?limit=${INITIAL_EVENTS_LIMIT}`)

  const events = (response as { events?: unknown } | null)?.events
  if (!Array.isArray(events)) return []
  // Lo que no se pueda normalizar (sin id) se descarta sin romper la lista.
  return events.flatMap((item) => normalizeEvent(item) ?? [])
}

/** Abre el stream de eventos y devuelve la función que lo cierra. */
export function openEventStream(handlers: EventStreamHandlers): () => void {
  // Mismo camino para el stream real y el mock: data (texto JSON) → normalizar → entregar.
  const handleData = (data: string) => {
    try {
      const event = normalizeEvent(JSON.parse(data))
      if (event) handlers.onEvent(event)
    } catch {
      // JSON inválido: se ignora sin cerrar la conexión.
    }
  }

  if (useMock) {
    return mockOpenEventStream({ onOpen: handlers.onOpen, onMessage: handleData })
  }

  // Al reconectar, el navegador envía Last-Event-ID por su cuenta.
  const source = new EventSource(`${API_BASE}${EVENT_ENDPOINTS.stream}`, {
    withCredentials: true,
  })
  source.onopen = handlers.onOpen
  source.addEventListener(STREAM_EVENT_NAME, (message) => {
    handleData((message as MessageEvent<string>).data)
  })
  source.onerror = handlers.onError
  return () => source.close()
}

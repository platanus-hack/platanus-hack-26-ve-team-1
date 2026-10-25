import { useCallback, useEffect, useRef, useState } from 'react'
import { getEvents } from '../api/events'
import type { EventItem } from '../api/types'
import { useEventStream } from '../hooks/useEventStream'
import { useToast } from '../hooks/useToast'
import { getErrorMessage } from '../lib/errors'
import { getAttributeLabel, getResultInfo } from '../lib/events'
import { formatDateTime } from '../lib/format'
import { cn } from '../lib/cn'
import { Badge, Button, Card, EmptyState, Spinner } from './ui'

type LoadState =
  | { status: 'loading' }
  | { status: 'ready' }
  | { status: 'error'; error: unknown }

type EventList = {
  events: EventItem[]
  /** Ids llegados en vivo: solo ellos se animan al aparecer. */
  liveIds: Set<number>
}

function mergeEvents(current: EventItem[], incoming: EventItem[]): EventItem[] {
  const byId = new Map(current.map((event) => [event.id, event]))
  for (const event of incoming) byId.set(event.id, event)
  return [...byId.values()].sort((a, b) => b.hora.localeCompare(a.hora) || b.id - a.id)
}

function LiveIndicator({ connected }: { connected: boolean }) {
  return (
    <p
      role="status"
      className={cn(
        'inline-flex items-center gap-2 rounded-full px-4 py-1 text-base font-semibold ring-1 ring-inset',
        connected
          ? 'bg-approved-100 text-approved-900 ring-approved-600'
          : 'bg-neutral-100 text-neutral-800 ring-neutral-400',
      )}
    >
      <span
        aria-hidden="true"
        className={cn(
          'size-3 rounded-full',
          connected
            ? 'bg-approved-600 motion-safe:animate-pulse'
            : 'bg-neutral-500',
        )}
      />
      {connected ? 'En vivo' : 'Reconectando…'}
    </p>
  )
}

function EventRow({ event, isLive }: { event: EventItem; isLive: boolean }) {
  const result = getResultInfo(event.resultado)
  return (
    <li
      className={cn(
        'flex flex-wrap items-center justify-between gap-x-6 gap-y-2 border-l-4 px-4 py-4',
        event.emergencia ? 'border-amber-600 bg-amber-50' : 'border-transparent',
        isLive && 'motion-safe:animate-event-in',
      )}
    >
      <div className="flex min-w-0 flex-col gap-1">
        <p className="text-xl font-bold text-neutral-900">{event.local}</p>
        <p className="text-lg text-neutral-700">
          {getAttributeLabel(event.atributo)}
        </p>
        <p className="text-base text-neutral-600">
          <time dateTime={event.hora}>{formatDateTime(event.hora)}</time>
        </p>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        {event.emergencia && (
          <Badge variant="warning" className="px-4 py-1">
            Acceso de emergencia
          </Badge>
        )}
        <Badge variant={result.variant} className="px-4 py-1 text-lg">
          {result.label}
        </Badge>
      </div>
    </li>
  )
}

export function EventHistory({ onNewEvent }: { onNewEvent?: () => void }) {
  const { toast } = useToast()
  const [load, setLoad] = useState<LoadState>({ status: 'loading' })
  const [attempt, setAttempt] = useState(0)
  const [list, setList] = useState<EventList>({ events: [], liveIds: new Set() })

  const knownIdsRef = useRef(new Set<number>())
  const onNewEventRef = useRef(onNewEvent)

  useEffect(() => {
    onNewEventRef.current = onNewEvent
  })

  const handleLiveEvent = useCallback((event: EventItem) => {
    if (knownIdsRef.current.has(event.id)) return
    knownIdsRef.current.add(event.id)
    setList((current) => ({
      events: mergeEvents(current.events, [event]),
      liveIds: new Set(current.liveIds).add(event.id),
    }))
    onNewEventRef.current?.()
  }, [])

  const { connected } = useEventStream(handleLiveEvent)

  useEffect(() => {
    let cancelled = false
    getEvents()
      .then((events) => {
        if (cancelled) return
        for (const event of events) knownIdsRef.current.add(event.id)
        // Se fusiona por si el stream ya entregó eventos antes de terminar la carga.
        setList((current) => ({
          events: mergeEvents(current.events, events),
          liveIds: current.liveIds,
        }))
        setLoad({ status: 'ready' })
      })
      .catch((error: unknown) => {
        if (cancelled) return
        setLoad({ status: 'error', error })
        toast({
          title: 'No se pudo cargar tu historial',
          description: getErrorMessage(error),
          variant: 'error',
        })
      })
    return () => {
      cancelled = true
    }
  }, [attempt, toast])

  const reload = useCallback(() => {
    setLoad({ status: 'loading' })
    setAttempt((current) => current + 1)
  }, [])

  return (
    <section aria-labelledby="historial" className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-col gap-1">
          <h2 id="historial" className="text-2xl font-bold text-primary-900">
            Historial
          </h2>
          <p className="text-lg text-neutral-700">
            Quién consultó tu cédula y qué dato recibió.
          </p>
        </div>
        <LiveIndicator connected={connected} />
      </div>

      {load.status === 'loading' && (
        <div className="flex items-center gap-3 py-8 text-xl text-primary-800">
          <Spinner label="Cargando tu historial" className="[&>svg]:size-8" />
          <span aria-hidden="true">Cargando tu historial…</span>
        </div>
      )}

      {load.status === 'error' && list.events.length === 0 && (
        <EmptyState
          title="No pudimos cargar tu historial"
          description={getErrorMessage(load.error)}
          action={<Button onClick={reload}>Reintentar</Button>}
        />
      )}

      {load.status === 'ready' && list.events.length === 0 && (
        <EmptyState
          title="Aún no hay consultas"
          description="Cuando un comercio o servicio lea tu cédula, aparecerá aquí al instante."
        />
      )}

      <div role="log" aria-live="polite" aria-label="Historial de consultas">
        {list.events.length > 0 && (
          <Card className="p-0">
            <ul className="divide-y divide-neutral-200 overflow-hidden rounded-2xl">
              {list.events.map((event) => (
                <EventRow
                  key={event.id}
                  event={event}
                  isLive={list.liveIds.has(event.id)}
                />
              ))}
            </ul>
          </Card>
        )}
      </div>
    </section>
  )
}

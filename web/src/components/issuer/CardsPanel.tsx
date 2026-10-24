import { useEffect, useMemo, useRef, useState } from 'react'
import { revokeCard } from '../../api/card'
import type { Citizen } from '../../api/types'
import { useCitizens } from '../../hooks/useCitizens'
import { useToast } from '../../hooks/useToast'
import { getErrorMessage } from '../../lib/errors'
import { Button, EmptyState, Input, Modal, Spinner } from '../ui'
import { CitizensTable } from './CitizensTable'
import { EmitCardModal } from './EmitCardModal'

/** Minúsculas y sin tildes, para buscar "maria" y encontrar "María". */
function normalizeText(value: string): string {
  return value
    .normalize('NFD')
    .replace(/\p{Diacritic}/gu, '')
    .toLowerCase()
    .trim()
}

export function CardsPanel() {
  const { toast } = useToast()
  const { state, reload, refresh, markRevoked } = useCitizens()
  const [query, setQuery] = useState('')
  const [emitTarget, setEmitTarget] = useState<Citizen | null>(null)
  const [revokeTarget, setRevokeTarget] = useState<Citizen | null>(null)
  const [revoking, setRevoking] = useState(false)
  const revokingRef = useRef(false)
  const [revokeError, setRevokeError] = useState<string>()

  useEffect(() => {
    if (state.status === 'error') {
      toast({
        title: 'No se pudo cargar la lista de ciudadanos',
        description: getErrorMessage(state.error),
        variant: 'error',
      })
    }
  }, [state, toast])

  const filtered = useMemo(() => {
    if (state.status !== 'ready') return []
    const needle = normalizeText(query)
    if (!needle) return state.citizens
    return state.citizens.filter((citizen) =>
      normalizeText(`${citizen.nombre} ${citizen.email}`).includes(needle),
    )
  }, [state, query])

  function openRevoke(citizen: Citizen) {
    setRevokeError(undefined)
    setRevokeTarget(citizen)
  }

  async function handleRevoke() {
    if (!revokeTarget || revokingRef.current) return
    revokingRef.current = true
    setRevoking(true)
    setRevokeError(undefined)
    try {
      await revokeCard(revokeTarget.id)
      markRevoked(revokeTarget.id)
      toast({
        title: 'Cédula revocada',
        description: `La cédula de ${revokeTarget.nombre} ya no será aceptada.`,
        variant: 'ok',
      })
      setRevokeTarget(null)
    } catch (error) {
      // El modal sigue abierto y la cédula no cambia de estado.
      const message = getErrorMessage(error)
      setRevokeError(message)
      toast({ title: 'No se pudo revocar la cédula', description: message, variant: 'error' })
    } finally {
      revokingRef.current = false
      setRevoking(false)
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <p className="text-xl text-neutral-700">
        Emite y revoca las cédulas sIsA de los ciudadanos.
      </p>

      <div className="max-w-xl">
        <Input
          label="Buscar ciudadano"
          type="search"
          placeholder="Nombre o correo"
          autoComplete="off"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
      </div>

      {state.status === 'loading' && (
        <div className="flex items-center gap-3 py-8 text-xl text-primary-800">
          <Spinner label="Cargando ciudadanos" className="[&>svg]:size-8" />
          <span aria-hidden="true">Cargando ciudadanos…</span>
        </div>
      )}

      {state.status === 'error' && (
        <EmptyState
          title="No pudimos cargar los ciudadanos"
          description={getErrorMessage(state.error)}
          action={<Button onClick={reload}>Reintentar</Button>}
        />
      )}

      {state.status === 'ready' && state.citizens.length === 0 && (
        <EmptyState
          title="Aún no hay ciudadanos"
          description="Cuando alguien se registre en sIsA, aparecerá aquí para emitirle una cédula."
        />
      )}

      {state.status === 'ready' && state.citizens.length > 0 && filtered.length === 0 && (
        <EmptyState
          title="Sin resultados"
          description={`No encontramos ciudadanos que coincidan con «${query.trim()}».`}
          action={
            <Button variant="secondary" onClick={() => setQuery('')}>
              Limpiar búsqueda
            </Button>
          }
        />
      )}

      {filtered.length > 0 && (
        <CitizensTable
          citizens={filtered}
          onEmit={setEmitTarget}
          onRevoke={openRevoke}
          disabled={emitTarget !== null || revokeTarget !== null || revoking}
        />
      )}

      {emitTarget && (
        <EmitCardModal
          citizen={emitTarget}
          onClose={() => setEmitTarget(null)}
          onCompleted={refresh}
        />
      )}

      <Modal
        open={revokeTarget !== null}
        onClose={() => setRevokeTarget(null)}
        title="¿Revocar cédula?"
        footer={
          <>
            <Button
              variant="secondary"
              onClick={() => setRevokeTarget(null)}
              disabled={revoking}
              data-autofocus
            >
              Cancelar
            </Button>
            <Button variant="danger" onClick={handleRevoke} loading={revoking}>
              {revoking ? 'Revocando…' : 'Sí, revocar cédula'}
            </Button>
          </>
        }
      >
        {revokeTarget && (
          <>
            La cédula de <strong>{revokeTarget.nombre}</strong> dejará de ser aceptada por
            terminales y billeteras de inmediato. Podrás emitirle una nueva después.
          </>
        )}
        {revokeError && (
          <p role="alert" className="mt-3 font-semibold text-denied-700">
            No se pudo revocar: {revokeError}
          </p>
        )}
      </Modal>
    </div>
  )
}

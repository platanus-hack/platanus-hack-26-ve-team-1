import { useEffect, useRef, useState } from 'react'
import { deleteTerminal } from '../api/terminals'
import type { Terminal } from '../api/types'
import { TerminalForm } from '../components/merchant/TerminalForm'
import { TerminalsList } from '../components/merchant/TerminalsList'
import { TokenModal } from '../components/merchant/TokenModal'
import { UsageStats } from '../components/merchant/UsageStats'
import { PageTitle } from '../components/PageTitle'
import { Button, EmptyState, Modal, Spinner } from '../components/ui'
import { useTerminals } from '../hooks/useTerminals'
import { useToast } from '../hooks/useToast'
import { getErrorMessage } from '../lib/errors'

type FormTarget = { terminal?: Terminal }

export default function MerchantPage() {
  const { toast } = useToast()
  const { state, reload, refresh } = useTerminals()
  const [formTarget, setFormTarget] = useState<FormTarget | null>(null)
  // El token en claro vive solo aquí y solo mientras el modal está abierto.
  const [issuedToken, setIssuedToken] = useState<{ nombre: string; token: string } | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<Terminal | null>(null)
  const [deleting, setDeleting] = useState(false)
  const deletingRef = useRef(false)
  const [deleteError, setDeleteError] = useState<string>()
  const [statsVersion, setStatsVersion] = useState(0)

  useEffect(() => {
    if (state.status === 'error') {
      toast({
        title: 'No se pudo cargar la lista de terminales',
        description: getErrorMessage(state.error),
        variant: 'error',
      })
    }
  }, [state, toast])

  function handleSaved(saved: Terminal, mode: 'created' | 'updated') {
    if (mode === 'created' && saved.token) {
      setIssuedToken({ nombre: saved.nombre, token: saved.token })
    }
    refresh()
    setStatsVersion((version) => version + 1)
  }

  function openDelete(terminal: Terminal) {
    setDeleteError(undefined)
    setDeleteTarget(terminal)
  }

  async function handleDelete() {
    if (!deleteTarget || deletingRef.current) return
    deletingRef.current = true
    setDeleting(true)
    setDeleteError(undefined)
    try {
      await deleteTerminal(deleteTarget.id)
      toast({
        title: 'Terminal eliminado',
        description: `«${deleteTarget.nombre}» ya no podrá leer cédulas.`,
        variant: 'ok',
      })
      setDeleteTarget(null)
      refresh()
      setStatsVersion((version) => version + 1)
    } catch (error) {
      // El modal sigue abierto y el terminal no desaparece de la lista.
      const message = getErrorMessage(error)
      setDeleteError(message)
      toast({ title: 'No se pudo eliminar el terminal', description: message, variant: 'error' })
    } finally {
      deletingRef.current = false
      setDeleting(false)
    }
  }

  const terminals = state.status === 'ready' ? state.terminals : []
  const busy = formTarget !== null || deleteTarget !== null || issuedToken !== null

  return (
    <div className="flex flex-col gap-8">
      <header className="flex flex-col gap-1">
        <PageTitle>Configuración de terminales</PageTitle>
        <p className="text-xl text-neutral-700">
          Crea los terminales que leen la cédula sIsA y revisa cuánto se usan.
        </p>
      </header>

      <section aria-labelledby="mis-terminales" className="flex flex-col gap-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 id="mis-terminales" className="text-2xl font-bold text-primary-900">
            Mis terminales
          </h2>
          <Button onClick={() => setFormTarget({})} disabled={busy}>
            Nuevo terminal
          </Button>
        </div>

        {state.status === 'loading' && (
          <div className="flex items-center gap-3 py-8 text-xl text-primary-800">
            <Spinner label="Cargando terminales" className="[&>svg]:size-8" />
            <span aria-hidden="true">Cargando terminales…</span>
          </div>
        )}

        {state.status === 'error' && (
          <EmptyState
            title="No pudimos cargar los terminales"
            description={getErrorMessage(state.error)}
            action={<Button onClick={reload}>Reintentar</Button>}
          />
        )}

        {state.status === 'ready' && terminals.length === 0 && (
          <EmptyState
            title="Aún no tienes terminales"
            description="Crea el primero para empezar a verificar cédulas."
            action={<Button onClick={() => setFormTarget({})}>Nuevo terminal</Button>}
          />
        )}

        {terminals.length > 0 && (
          <TerminalsList
            terminals={terminals}
            onEdit={(terminal) => setFormTarget({ terminal })}
            onDelete={openDelete}
            disabled={busy}
          />
        )}
      </section>

      {state.status === 'ready' && <UsageStats terminals={terminals} version={statsVersion} />}

      {formTarget && (
        <TerminalForm
          terminal={formTarget.terminal}
          onClose={() => setFormTarget(null)}
          onSaved={handleSaved}
        />
      )}

      {issuedToken && (
        <TokenModal
          terminalName={issuedToken.nombre}
          token={issuedToken.token}
          onClose={() => setIssuedToken(null)}
        />
      )}

      <Modal
        open={deleteTarget !== null}
        onClose={() => setDeleteTarget(null)}
        title="¿Eliminar terminal?"
        footer={
          <>
            <Button
              variant="secondary"
              onClick={() => setDeleteTarget(null)}
              disabled={deleting}
              data-autofocus
            >
              Cancelar
            </Button>
            <Button variant="danger" onClick={handleDelete} loading={deleting}>
              {deleting ? 'Eliminando…' : 'Sí, eliminar terminal'}
            </Button>
          </>
        }
      >
        {deleteTarget && (
          <>
            El terminal <strong>{deleteTarget.nombre}</strong> dejará de funcionar de inmediato y
            su token quedará inválido.
          </>
        )}
        {deleteError && (
          <p role="alert" className="mt-3 font-semibold text-denied-700">
            No se pudo eliminar: {deleteError}
          </p>
        )}
      </Modal>
    </div>
  )
}

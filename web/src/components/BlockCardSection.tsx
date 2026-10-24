import { useState } from 'react'
import { revokeCard } from '../api/card'
import type { CardInfo } from '../api/types'
import { useToast } from '../hooks/useToast'
import { getErrorMessage } from '../lib/errors'
import { Badge, Button, Card, Modal } from './ui'

type BlockCardSectionProps = {
  card: CardInfo
  /** Se llama tras bloquear con éxito para actualizar el estado ya cargado. */
  onRevoked: () => void
}

export function BlockCardSection({ card, onRevoked }: BlockCardSectionProps) {
  const { toast } = useToast()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [blocking, setBlocking] = useState(false)
  const [blockError, setBlockError] = useState<string>()
  const revoked = card.estado === 'revocada'

  function openConfirm() {
    setBlockError(undefined)
    setConfirmOpen(true)
  }

  async function handleBlock() {
    setBlocking(true)
    setBlockError(undefined)
    try {
      await revokeCard()
      onRevoked()
      setConfirmOpen(false)
      toast({ title: 'Cédula bloqueada', description: 'Ningún lector puede consultar tus datos.', variant: 'ok' })
    } catch (error) {
      // El modal sigue abierto y la cédula no cambia de estado.
      const message = getErrorMessage(error)
      setBlockError(message)
      toast({ title: 'No se pudo bloquear tu cédula', description: message, variant: 'error' })
    } finally {
      setBlocking(false)
    }
  }

  return (
    <section aria-labelledby="bloquear-cedula" className="flex flex-col gap-4">
      <h2 id="bloquear-cedula" className="text-2xl font-bold text-primary-900">
        Bloquear mi cédula
      </h2>
      <Card className="flex flex-col gap-4">
        <div role="status" className="empty:hidden">
          {revoked && (
            <div className="flex flex-col gap-4">
              <div>
                <Badge variant="error" className="px-5 py-2 text-xl">
                  Cédula bloqueada
                </Badge>
              </div>
              <div className="flex flex-col gap-1">
                <h3 className="text-xl font-bold text-neutral-900">Reactivar</h3>
                <p className="text-lg text-neutral-700">Pide la reactivación al emisor.</p>
              </div>
            </div>
          )}
        </div>
        {!revoked && (
          <>
            <p className="text-lg text-neutral-700">
              Si perdiste tu cédula o crees que alguien la está usando, bloquéala. Ningún lector
              podrá consultar tus datos hasta que el emisor la reactive.
            </p>
            <div>
              <Button variant="danger" onClick={openConfirm}>
                Bloquear mi cédula
              </Button>
            </div>
          </>
        )}
      </Card>

      <Modal
        open={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        title="¿Bloquear tu cédula?"
        footer={
          <>
            <Button
              variant="secondary"
              onClick={() => setConfirmOpen(false)}
              disabled={blocking}
              data-autofocus
            >
              Cancelar
            </Button>
            <Button variant="danger" onClick={handleBlock} loading={blocking}>
              {blocking ? 'Bloqueando…' : 'Sí, bloquear mi cédula'}
            </Button>
          </>
        }
      >
        El bloqueo es inmediato: ningún terminal ni billetera aceptará tu cédula. Solo el emisor
        puede reactivarla.
        {blockError && (
          <p role="alert" className="mt-3 font-semibold text-denied-700">
            No se pudo bloquear: {blockError}
          </p>
        )}
      </Modal>
    </section>
  )
}

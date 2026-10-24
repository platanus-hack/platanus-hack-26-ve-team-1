import { useEffect, useRef, useState } from 'react'
import { orderEmission } from '../../api/issuer'
import type { Citizen } from '../../api/types'
import { useToast } from '../../hooks/useToast'
import { getErrorMessage } from '../../lib/errors'
import { Button, Modal } from '../ui'
import { EmissionStatus, type EmissionResult } from './EmissionStatus'

type Phase =
  | { name: 'confirm' }
  | { name: 'waiting' }
  | { name: 'done' }
  | { name: 'failed'; message: string }

type EmitCardModalProps = {
  citizen: Citizen
  onClose: () => void
  /** Se llama cuando la emisión termina bien, para actualizar la lista. */
  onCompleted: () => void
}

// Se monta solo mientras hay un ciudadano elegido: cerrar desmonta y corta el polling.
export function EmitCardModal({ citizen, onClose, onCompleted }: EmitCardModalProps) {
  const { toast } = useToast()
  const [phase, setPhase] = useState<Phase>({ name: 'confirm' })
  const [ordering, setOrdering] = useState(false)
  const [orderError, setOrderError] = useState<string>()
  const [confirmingClose, setConfirmingClose] = useState(false)
  const submittingRef = useRef(false)

  // Al cambiar de paso el botón enfocado desaparece: se lleva el foco al nuevo control principal.
  useEffect(() => {
    document.querySelector<HTMLElement>('dialog[open] [data-autofocus]')?.focus()
  }, [phase.name, confirmingClose])

  async function handleConfirm() {
    if (submittingRef.current) return
    submittingRef.current = true
    setOrdering(true)
    setOrderError(undefined)
    try {
      await orderEmission(citizen.id)
      setPhase({ name: 'waiting' })
    } catch (error) {
      const message = getErrorMessage(error)
      setOrderError(message)
      toast({ title: 'No se pudo iniciar la emisión', description: message, variant: 'error' })
    } finally {
      submittingRef.current = false
      setOrdering(false)
    }
  }

  function handleResult(result: EmissionResult) {
    setConfirmingClose(false)
    if (result.ok) {
      setPhase({ name: 'done' })
      onCompleted()
      toast({
        title: 'Cédula emitida',
        description: `La cédula de ${citizen.nombre} quedó grabada en la tarjeta.`,
        variant: 'ok',
      })
    } else {
      setPhase({ name: 'failed', message: result.message })
      toast({ title: 'La emisión no se completó', description: result.message, variant: 'error' })
    }
  }

  // Con la emisión pendiente, cerrar pide confirmación; al cerrar se deja de consultar el estado.
  function requestClose() {
    if (phase.name === 'waiting') {
      setConfirmingClose(true)
      return
    }
    onClose()
  }

  const title =
    phase.name === 'confirm'
      ? '¿Emitir cédula?'
      : phase.name === 'done'
        ? 'Cédula emitida'
        : 'Emisión en curso'

  const footer =
    phase.name === 'confirm' ? (
      <>
        <Button variant="secondary" onClick={onClose} disabled={ordering} data-autofocus>
          Cancelar
        </Button>
        <Button onClick={handleConfirm} loading={ordering}>
          {ordering ? 'Iniciando…' : 'Emitir cédula'}
        </Button>
      </>
    ) : phase.name === 'waiting' ? (
      confirmingClose ? (
        <>
          <Button variant="secondary" onClick={() => setConfirmingClose(false)} data-autofocus>
            Seguir esperando
          </Button>
          <Button variant="danger" onClick={onClose}>
            Cerrar de todos modos
          </Button>
        </>
      ) : (
        <Button variant="secondary" onClick={requestClose} data-autofocus>
          Cancelar emisión
        </Button>
      )
    ) : phase.name === 'failed' ? (
      <>
        <Button variant="secondary" onClick={onClose} data-autofocus>
          Cerrar
        </Button>
        <Button onClick={() => setPhase({ name: 'confirm' })}>Reintentar</Button>
      </>
    ) : (
      <Button onClick={onClose} data-autofocus>
        Listo
      </Button>
    )

  return (
    <Modal open onClose={requestClose} title={title} footer={footer}>
      {phase.name === 'confirm' && (
        <>
          Se emitirá una cédula sIsA para <strong>{citizen.nombre}</strong>. Necesitas una tarjeta
          virgen y el terminal en modo emisión.
          {orderError && (
            <p role="alert" className="mt-3 font-semibold text-denied-700">
              No se pudo iniciar: {orderError}
            </p>
          )}
        </>
      )}
      {phase.name === 'waiting' && (
        <>
          <EmissionStatus onResult={handleResult} />
          {confirmingClose && (
            <p role="alert" className="mt-3 font-semibold text-denied-700">
              La emisión sigue esperando la tarjeta. ¿Cerrar de todos modos?
            </p>
          )}
        </>
      )}
      {phase.name === 'done' && (
        <p role="status" aria-live="polite">
          La cédula de <strong>{citizen.nombre}</strong> quedó grabada en la tarjeta y ya está
          activa.
        </p>
      )}
      {phase.name === 'failed' && (
        <p role="alert" className="font-semibold text-denied-700">
          {phase.message}
        </p>
      )}
    </Modal>
  )
}

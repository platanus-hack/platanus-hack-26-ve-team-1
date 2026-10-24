import { useRef, useState } from 'react'
import { issueUniversityCredential } from '../../api/university'
import type { Student } from '../../api/types'
import { useToast } from '../../hooks/useToast'
import { getErrorMessage } from '../../lib/errors'
import { Button, Modal } from '../ui'

type IssueCredentialModalProps = {
  student: Student
  onClose: () => void
  /** Se llama tras emitir con éxito, para actualizar la lista. */
  onIssued: () => void
}

// Se monta solo mientras hay un estudiante elegido: cerrar lo desmonta.
export function IssueCredentialModal({ student, onClose, onIssued }: IssueCredentialModalProps) {
  const { toast } = useToast()
  const [issuing, setIssuing] = useState(false)
  const [error, setError] = useState<string>()
  const submittingRef = useRef(false)

  async function handleConfirm() {
    if (submittingRef.current) return
    submittingRef.current = true
    setIssuing(true)
    setError(undefined)
    try {
      await issueUniversityCredential(student.id)
      onIssued()
      toast({
        title: 'Carnet emitido',
        description: `El carnet universitario de ${student.nombre} quedó emitido.`,
        variant: 'ok',
      })
      onClose()
    } catch (failure) {
      // El modal sigue abierto y el estado del estudiante no cambia.
      const message = getErrorMessage(failure)
      setError(message)
      toast({ title: 'No se pudo emitir el carnet', description: message, variant: 'error' })
      submittingRef.current = false
      setIssuing(false)
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      title="¿Emitir carnet universitario?"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={issuing} data-autofocus>
            Cancelar
          </Button>
          <Button onClick={handleConfirm} loading={issuing}>
            {issuing ? 'Emitiendo…' : 'Sí, emitir carnet'}
          </Button>
        </>
      }
    >
      Se emitirá el carnet universitario sIsA de <strong>{student.nombre}</strong>.
      {error && (
        <p role="alert" className="mt-3 font-semibold text-denied-700">
          No se pudo emitir: {error}
        </p>
      )}
    </Modal>
  )
}

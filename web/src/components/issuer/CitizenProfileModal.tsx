import { useId, useRef, useState, type SubmitEvent } from 'react'
import { updateCitizenProfile } from '../../api/issuer'
import type { Citizen } from '../../api/types'
import { useToast } from '../../hooks/useToast'
import { getErrorMessage } from '../../lib/errors'
import { validateBirthDate, validateCedulaNumber } from '../../lib/validation'
import { Button, Input, Modal } from '../ui'

type CitizenProfileModalProps = {
  citizen: Citizen
  onClose: () => void
  /** Se llama tras guardar: la fila pasa a perfil completo sin recargar la lista. */
  onSaved: (citizenId: string) => void
}

type FieldErrors = { fechaNacimiento?: string; cedula?: string }

// Los valores solo viven en este modal: se descartan al cerrarlo y nunca se muestran en la tabla.
export function CitizenProfileModal({ citizen, onClose, onSaved }: CitizenProfileModalProps) {
  const { toast } = useToast()
  const formId = useId()
  const [fechaNacimiento, setFechaNacimiento] = useState('')
  const [cedula, setCedula] = useState('')
  const [errors, setErrors] = useState<FieldErrors>({})
  const [saving, setSaving] = useState(false)
  const submittingRef = useRef(false)
  // Fecha máxima del selector: hoy, en la zona horaria local (YYYY-MM-DD).
  const [today] = useState(() => new Date().toLocaleDateString('en-CA'))

  async function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    if (submittingRef.current) return
    const nextErrors: FieldErrors = {
      fechaNacimiento: validateBirthDate(fechaNacimiento),
      cedula: validateCedulaNumber(cedula),
    }
    setErrors(nextErrors)
    if (nextErrors.fechaNacimiento || nextErrors.cedula) return

    submittingRef.current = true
    setSaving(true)
    try {
      await updateCitizenProfile(citizen.id, {
        fechaNacimiento,
        cedula: cedula.trim(),
      })
      onSaved(citizen.id)
      toast({
        title: 'Perfil completado',
        description: `Ya puedes emitir la cédula de ${citizen.nombre}.`,
        variant: 'ok',
      })
      onClose()
    } catch (error) {
      // El modal sigue abierto y no se pierde lo escrito.
      toast({
        title: 'No se pudo guardar el perfil',
        description: getErrorMessage(error),
        variant: 'error',
      })
      submittingRef.current = false
      setSaving(false)
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      title="Completar perfil"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={saving}>
            Cancelar
          </Button>
          <Button type="submit" form={formId} loading={saving}>
            {saving ? 'Guardando…' : 'Guardar perfil'}
          </Button>
        </>
      }
    >
      <form id={formId} onSubmit={handleSubmit} noValidate className="flex flex-col gap-5">
        <p>
          Para emitir la cédula de <strong>{citizen.nombre}</strong> hacen falta su fecha de
          nacimiento y su número de cédula.
        </p>
        <Input
          label="Fecha de nacimiento"
          type="date"
          max={today}
          data-autofocus
          value={fechaNacimiento}
          onChange={(event) => setFechaNacimiento(event.target.value)}
          error={errors.fechaNacimiento}
        />
        <Input
          label="Número de cédula"
          inputMode="numeric"
          autoComplete="off"
          hint="Solo números, sin puntos ni letras."
          value={cedula}
          onChange={(event) => setCedula(event.target.value)}
          error={errors.cedula}
        />
      </form>
    </Modal>
  )
}

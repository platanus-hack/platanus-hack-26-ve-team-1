import { useId, useState, type SubmitEvent } from 'react'
import { saveNotifications } from '../api/notifications'
import { useAuth } from '../hooks/useAuth'
import { useToast } from '../hooks/useToast'
import { getErrorMessage } from '../lib/errors'
import { normalizePhone, validatePhone } from '../lib/validation'
import { Button, Card, Checkbox, Input } from './ui'

export function WhatsAppSettings() {
  const { user, updateUser } = useAuth()
  const { toast } = useToast()
  const blockHelpId = useId()
  const [telefono, setTelefono] = useState(user?.telefono ?? '')
  const [avisosActivados, setAvisosActivados] = useState(user?.avisosActivados ?? false)
  const [error, setError] = useState<string>()
  const [saving, setSaving] = useState(false)

  async function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    // El teléfono es obligatorio solo si se activan los avisos.
    const phoneError =
      telefono.trim() || avisosActivados ? validatePhone(telefono) : undefined
    setError(phoneError)
    if (phoneError) return

    const settings = {
      telefono: telefono.trim() ? normalizePhone(telefono) : null,
      avisosActivados,
    }
    setSaving(true)
    try {
      const saved = await saveNotifications(settings)
      updateUser({ telefono: saved.telefono, avisosActivados: saved.avisosActivados })
      toast({ title: 'Avisos guardados', variant: 'ok' })
    } catch (saveError) {
      toast({
        title: 'No se pudieron guardar los avisos',
        description: getErrorMessage(saveError),
        variant: 'error',
      })
    } finally {
      setSaving(false)
    }
  }

  return (
    <section aria-labelledby="avisos-whatsapp" className="flex flex-col gap-4">
      <div className="flex flex-col gap-1">
        <h2 id="avisos-whatsapp" className="text-2xl font-bold text-primary-900">
          Avisos por WhatsApp
        </h2>
        <p id={blockHelpId} className="text-lg text-neutral-700">
          Te avisamos por WhatsApp cuando alguien lee tu cédula. Si no reconoces un acceso,
          responde <strong>BLOQUEAR</strong> a ese mensaje y tu cédula se bloqueará.
        </p>
      </div>
      <Card>
        <form onSubmit={handleSubmit} noValidate className="flex flex-col gap-5">
          <Input
            label="Teléfono de WhatsApp"
            type="tel"
            autoComplete="tel"
            placeholder="+56 9 1234 5678"
            hint="Con código de país, por ejemplo +56 9 1234 5678."
            value={telefono}
            onChange={(event) => setTelefono(event.target.value)}
            error={error}
          />
          <Checkbox
            label="Activar avisos por WhatsApp"
            checked={avisosActivados}
            aria-describedby={blockHelpId}
            onChange={(event) => setAvisosActivados(event.target.checked)}
          />
          <div>
            <Button type="submit" loading={saving}>
              {saving ? 'Guardando…' : 'Guardar avisos'}
            </Button>
          </div>
        </form>
      </Card>
    </section>
  )
}

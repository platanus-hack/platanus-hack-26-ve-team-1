import { useCallback, useEffect, useId, useState, type ReactNode, type SubmitEvent } from 'react'
import { getOptionalData, saveOptionalData } from '../api/optionalData'
import type { Attribute, OptionalData } from '../api/types'
import { useToast } from '../hooks/useToast'
import { ATTRIBUTE_INFO } from '../lib/attributes'
import { getErrorMessage } from '../lib/errors'
import { BLOOD_TYPES, EMPTY_OPTIONAL_DATA, type BloodType } from '../lib/optionalData'
import {
  normalizePhone,
  validateAllergies,
  validatePhone,
} from '../lib/validation'
import { Badge, Button, Card, Checkbox, EmptyState, Input, Select, Spinner } from './ui'

type FormValues = {
  tipoSangre: BloodType | ''
  alergias: string
  contactoNombre: string
  contactoTelefono: string
  donante: boolean
}

type FieldErrors = Partial<
  Record<'tipoSangre' | 'alergias' | 'contactoNombre' | 'contactoTelefono', string>
>

type LoadState =
  | { status: 'loading' }
  | { status: 'ready' }
  | { status: 'error'; error: unknown }

function toFormValues(data: OptionalData): FormValues {
  return {
    tipoSangre: data.tipoSangre ?? '',
    alergias: data.alergias,
    contactoNombre: data.contactoEmergencia.nombre,
    contactoTelefono: data.contactoEmergencia.telefono,
    donante: data.donante,
  }
}

function toOptionalData(values: FormValues): OptionalData {
  return {
    tipoSangre: values.tipoSangre || null,
    alergias: values.alergias.trim(),
    contactoEmergencia: {
      nombre: values.contactoNombre.trim(),
      telefono: normalizePhone(values.contactoTelefono),
    },
    donante: values.donante,
  }
}

function validate(values: FormValues): FieldErrors {
  const errors: FieldErrors = {}
  if (values.tipoSangre && !BLOOD_TYPES.includes(values.tipoSangre)) {
    errors.tipoSangre = 'Elige un tipo de sangre de la lista.'
  }
  errors.alergias = validateAllergies(values.alergias)

  // El contacto es opcional, pero si se llena una parte hay que llenar la otra.
  const nombre = values.contactoNombre.trim()
  const telefono = values.contactoTelefono.trim()
  if (nombre || telefono) {
    if (!nombre) errors.contactoNombre = 'Ingresa el nombre del contacto.'
    errors.contactoTelefono = validatePhone(telefono)
  }
  return errors
}

function ReaderNote({ id, attribute }: { id: string; attribute: Attribute }) {
  return (
    <p id={id} className="text-base font-semibold text-neutral-600">
      Puede leerlo:{' '}
      <Badge variant="neutral" className="ml-1">
        {ATTRIBUTE_INFO[attribute].lector}
      </Badge>
    </p>
  )
}

function Group({
  title,
  attribute,
  children,
}: {
  title: string
  attribute: Attribute
  children: ReactNode
}) {
  const noteId = useId()
  return (
    <fieldset
      aria-describedby={noteId}
      className="flex flex-col gap-4 border-t border-neutral-200 pt-5"
    >
      <legend className="float-left mb-3 w-full text-xl font-bold text-neutral-900">
        {title}
      </legend>
      <ReaderNote id={noteId} attribute={attribute} />
      {children}
    </fieldset>
  )
}

export function OptionalDataForm() {
  const { toast } = useToast()
  const [load, setLoad] = useState<LoadState>({ status: 'loading' })
  const [attempt, setAttempt] = useState(0)
  const [values, setValues] = useState<FormValues>(() =>
    toFormValues(EMPTY_OPTIONAL_DATA),
  )
  const [errors, setErrors] = useState<FieldErrors>({})
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    let cancelled = false
    getOptionalData()
      .then((data) => {
        if (cancelled) return
        setValues(toFormValues(data))
        setLoad({ status: 'ready' })
      })
      .catch((error: unknown) => {
        if (cancelled) return
        setLoad({ status: 'error', error })
        toast({
          title: 'No se pudieron cargar tus datos opcionales',
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

  function update<K extends keyof FormValues>(key: K, value: FormValues[K]) {
    setValues((current) => ({ ...current, [key]: value }))
    setSaved(false)
  }

  async function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    const nextErrors = validate(values)
    setErrors(nextErrors)
    if (Object.values(nextErrors).some(Boolean)) return

    setSaving(true)
    try {
      const data = await saveOptionalData(toOptionalData(values))
      setValues(toFormValues(data))
      setSaved(true)
      toast({ title: 'Datos guardados', variant: 'ok' })
    } catch (error) {
      toast({
        title: 'No se pudieron guardar tus datos',
        description: getErrorMessage(error),
        variant: 'error',
      })
    } finally {
      setSaving(false)
    }
  }

  return (
    <section aria-labelledby="datos-opcionales" className="flex flex-col gap-4">
      <div className="flex flex-col gap-1">
        <h2 id="datos-opcionales" className="text-2xl font-bold text-primary-900">
          Mis datos opcionales
        </h2>
        <p className="text-lg text-neutral-700">
          Son voluntarios. Cada dato solo puede leerlo el tipo de terminal indicado.
        </p>
      </div>

      {load.status === 'loading' && (
        <div className="flex items-center gap-3 py-8 text-xl text-primary-800">
          <Spinner label="Cargando tus datos opcionales" className="[&>svg]:size-8" />
          <span aria-hidden="true">Cargando tus datos…</span>
        </div>
      )}

      {load.status === 'error' && (
        <EmptyState
          title="No pudimos cargar tus datos"
          description={getErrorMessage(load.error)}
          action={<Button onClick={reload}>Reintentar</Button>}
        />
      )}

      {load.status === 'ready' && (
        <Card>
          <form onSubmit={handleSubmit} noValidate className="flex flex-col gap-6">
            <Group title="Salud: tipo de sangre y alergias" attribute="sangre_alergias">
              <Select
                label="Tipo de sangre"
                value={values.tipoSangre}
                onChange={(event) =>
                  update('tipoSangre', event.target.value as BloodType | '')
                }
                error={errors.tipoSangre}
              >
                <option value="">Sin indicar</option>
                {BLOOD_TYPES.map((type) => (
                  <option key={type} value={type}>
                    {type}
                  </option>
                ))}
              </Select>
              <Input
                label="Alergias"
                hint="Por ejemplo: penicilina, maní."
                value={values.alergias}
                onChange={(event) => update('alergias', event.target.value)}
                error={errors.alergias}
              />
            </Group>

            <Group title="Contacto de emergencia" attribute="contacto_emergencia">
              <Input
                label="Nombre del contacto"
                autoComplete="off"
                value={values.contactoNombre}
                onChange={(event) => update('contactoNombre', event.target.value)}
                error={errors.contactoNombre}
              />
              <Input
                label="Teléfono del contacto"
                type="tel"
                autoComplete="off"
                placeholder="+56 9 1234 5678"
                hint="Con código de país, por ejemplo +56 9 1234 5678."
                value={values.contactoTelefono}
                onChange={(event) => update('contactoTelefono', event.target.value)}
                error={errors.contactoTelefono}
              />
            </Group>

            <Group title="Donación" attribute="donante_licencia">
              <Checkbox
                label="Soy donante de órganos"
                checked={values.donante}
                onChange={(event) => update('donante', event.target.checked)}
              />
            </Group>

            <div role="status">
              {saved && (
                <p className="rounded-xl border-2 border-approved-600 bg-approved-50 p-4 text-lg font-semibold text-approved-900">
                  Estos datos se escribirán en tu chip en la próxima emisión.
                </p>
              )}
            </div>

            <div>
              <Button type="submit" loading={saving}>
                {saving ? 'Guardando…' : 'Guardar datos'}
              </Button>
            </div>
          </form>
        </Card>
      )}
    </section>
  )
}

import { useState, type FormEvent } from 'react'
import { Button, Input } from '../ui'

type LinkFormProps = {
  busy: boolean
  /** Devuelve un mensaje de error en español, o null si se vinculó. */
  onLink: (code: string) => Promise<string | null>
}

const CODE_PATTERN = /^\d{6}$/

/** Vinculación: el ciudadano toca su cédula en el terminal y escribe el código de 6 dígitos que ve en pantalla. */
export function LinkForm({ busy, onLink }: LinkFormProps) {
  const [code, setCode] = useState('')
  const [error, setError] = useState<string | undefined>()

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    if (!CODE_PATTERN.test(code)) {
      setError('El código tiene 6 dígitos.')
      return
    }
    setError((await onLink(code)) ?? undefined)
  }

  return (
    <form onSubmit={handleSubmit} noValidate className="flex flex-col gap-4">
      <Input
        label="Código de vinculación"
        hint="Acerca tu cédula al terminal en modo vinculación y escribe el código que muestra (vale 2 minutos)."
        inputMode="numeric"
        autoComplete="one-time-code"
        maxLength={6}
        value={code}
        onChange={(event) => {
          setCode(event.target.value.replace(/\D/g, ''))
          setError(undefined)
        }}
        error={error}
      />
      <Button type="submit" loading={busy}>
        Vincular este celular
      </Button>
    </form>
  )
}

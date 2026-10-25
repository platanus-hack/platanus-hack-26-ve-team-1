import { useState, type FormEvent } from 'react'
import { Badge, Button, Input } from '../ui'

type TokenFormProps = {
  token: string | null
  onSave: (token: string | null) => Promise<void>
}

/**
 * Token del terminal (se crea en "Terminales"). Es opcional: sin él se verifica igual, pero las lecturas no
 * llegan al historial de la persona. Se guarda solo en este dispositivo (IndexedDB), nunca en localStorage.
 */
export function TokenForm({ token, onSave }: TokenFormProps) {
  const [value, setValue] = useState('')

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    if (!value.trim()) return
    await onSave(value)
    setValue('')
  }

  return (
    <details className="rounded-2xl border border-neutral-200 bg-white p-4">
      <summary className="cursor-pointer text-lg font-semibold text-neutral-900">Historial en vivo (token del terminal)</summary>
      <div className="mt-4 flex flex-col gap-4">
        {token ? (
          <div className="flex flex-wrap items-center gap-3">
            <Badge variant="ok">Token guardado</Badge>
            <Button variant="secondary" onClick={() => void onSave(null)}>
              Quitar token
            </Button>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <Input
              label="Token del terminal"
              hint="Créalo en Terminales. Sin él, las verificaciones funcionan pero no aparecen en el historial de la persona."
              type="password"
              autoComplete="off"
              value={value}
              onChange={(event) => setValue(event.target.value)}
            />
            <Button type="submit" disabled={!value.trim()}>
              Guardar token
            </Button>
          </form>
        )}
      </div>
    </details>
  )
}

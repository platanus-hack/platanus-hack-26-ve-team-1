import { useEffect, useState } from 'react'
import { Badge, Button } from '../ui'

type SyncStatusProps = {
  online: boolean
  issuers: number
  revoked: number
  lastSync: number | null
  pending: number
  onSync: () => void
}

function ago(ms: number): string {
  const s = Math.max(0, Math.round(ms / 1000))
  if (s < 60) return `hace ${s} s`
  if (s < 3600) return `hace ${Math.round(s / 60)} min`
  return `hace ${Math.round(s / 3600)} h`
}

export function SyncStatus({ online, issuers, revoked, lastSync, pending, onSync }: SyncStatusProps) {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [])

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-3">
        <Badge variant={online ? 'ok' : 'warning'}>{online ? 'En línea' : 'Sin red'}</Badge>
        {pending > 0 && <Badge variant="neutral">{pending} por enviar</Badge>}
      </div>
      {lastSync ? (
        <p className="text-base text-neutral-700">
          Emisores: {issuers} · Cédulas bloqueadas: {revoked} · Actualizado {ago(now - lastSync)}
        </p>
      ) : (
        <p role="alert" className="text-base font-medium text-denied-700">
          Aún no hay datos del emisor. Conéctate a internet una vez para poder verificar sin red.
        </p>
      )}
      <div>
        <Button variant="secondary" disabled={!online} onClick={onSync}>
          Actualizar ahora
        </Button>
      </div>
    </div>
  )
}

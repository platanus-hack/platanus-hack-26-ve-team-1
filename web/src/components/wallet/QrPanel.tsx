import { ROTATE_SECONDS } from '../../lib/slj/tables.ts'
import type { WalletCredential } from '../../lib/slj/wallet.ts'
import { Badge, Card, Spinner } from '../ui'
import { QrCode } from './QrCode'

type QrPanelProps = {
  cred: WalletCredential
  qr: string | null
  /** De 1 a 0: lo que queda para que el QR se renueve. */
  remaining: number
  error: boolean
  online: boolean
}

export function QrPanel({ cred, qr, remaining, error, online }: QrPanelProps) {
  const seconds = Math.ceil(remaining * ROTATE_SECONDS)
  return (
    <Card className="flex flex-col items-center gap-4">
      <div className="flex w-full flex-wrap items-center justify-between gap-2">
        <h2 className="text-2xl font-bold text-neutral-900">{cred.label}</h2>
        <Badge variant={online ? 'ok' : 'warning'}>{online ? 'En línea' : 'Sin red: sigue funcionando'}</Badge>
      </div>

      {error ? (
        <p role="alert" className="text-lg font-medium text-denied-700">
          No se pudo generar el QR. Cierra y abre la billetera.
        </p>
      ) : qr ? (
        <QrCode value={qr} label={`Código QR de ${cred.label}`} />
      ) : (
        <Spinner />
      )}

      <div className="w-full max-w-sm">
        <div
          role="progressbar"
          aria-label="Tiempo para que el QR se renueve"
          aria-valuemin={0}
          aria-valuemax={ROTATE_SECONDS}
          aria-valuenow={seconds}
          className="h-3 overflow-hidden rounded-full bg-neutral-200"
        >
          <div className="h-full bg-primary-600 transition-[width] duration-200" style={{ width: `${remaining * 100}%` }} />
        </div>
        <p className="mt-2 text-center text-base text-neutral-600">
          Se renueva cada {ROTATE_SECONDS} s. Una captura de pantalla deja de servir enseguida.
        </p>
      </div>
    </Card>
  )
}

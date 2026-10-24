import { attrLabel, ISSUER_NAME, RESULT } from '../../lib/slj/tables.ts'
import type { VerifyResult } from '../../lib/slj/verify.ts'
import { cn } from '../../lib/cn'

const FAILURES: Record<number, string> = {
  [RESULT.BAD_SIG]: 'Firma inválida',
  [RESULT.REVOKED]: 'Cédula bloqueada',
  [RESULT.EXPIRED]: 'Credencial vencida',
  [RESULT.NO_PERMISSION]: 'Este local no pide ese dato',
  [RESULT.STALE]: 'QR caducado o ya usado',
  [RESULT.BAD_FORMAT]: 'No es un QR válido de sIsA',
}

type ResultPanelProps = {
  result: VerifyResult
}

/**
 * Pantalla del comercio: solo "Mayor de 18: SÍ" y el emisor. Nunca nombre, handle ni ningún otro dato
 * de la persona, ni siquiera cuando se rechaza.
 */
export function ResultPanel({ result }: ResultPanelProps) {
  const approved = result.code === RESULT.OK
  const title = approved
    ? `${attrLabel(result.attr ?? 0, result.issuer ?? 0)}: SÍ`
    : (FAILURES[result.code] ?? 'Rechazado')
  return (
    <div
      role="status"
      aria-live="assertive"
      className={cn(
        'flex flex-col items-center gap-2 rounded-3xl px-6 py-12 text-center text-white',
        approved ? 'bg-approved-600' : 'bg-denied-600',
      )}
    >
      <span aria-hidden="true" className="text-7xl font-bold leading-none">
        {approved ? '✓' : '✕'}
      </span>
      <p className="text-4xl font-bold">{title}</p>
      {approved && <p className="text-xl">{ISSUER_NAME[result.issuer ?? 0] ?? 'Emisor reconocido'}</p>}
    </div>
  )
}

import { useEffect, useRef } from 'react'
import { getEmissionStatus } from '../../api/issuer'
import { Spinner } from '../ui'
import { getErrorMessage } from '../../lib/errors'

const POLL_INTERVAL_MS = 2000
const TIMEOUT_MS = 2 * 60 * 1000
const MAX_CONSECUTIVE_ERRORS = 3

export type EmissionResult = { ok: true } | { ok: false; message: string }

type EmissionStatusProps = {
  /** Se llama una sola vez: cuando la emisión termina, falla o se agota el tiempo. */
  onResult: (result: EmissionResult) => void
}

/**
 * Espera a que el terminal grabe la tarjeta, consultando el estado cada 2 s (máximo 2 min).
 * Tolera hasta 3 errores HTTP seguidos; una consulta exitosa reinicia la cuenta.
 */
export function EmissionStatus({ onResult }: EmissionStatusProps) {
  const onResultRef = useRef(onResult)

  useEffect(() => {
    onResultRef.current = onResult
  })

  useEffect(() => {
    let finished = false
    let consecutiveErrors = 0
    let pollTimer: ReturnType<typeof setTimeout> | undefined

    const finish = (result: EmissionResult) => {
      if (finished) return
      finished = true
      clearTimeout(pollTimer)
      clearTimeout(timeoutTimer)
      onResultRef.current(result)
    }

    const timeoutTimer = setTimeout(
      () =>
        finish({
          ok: false,
          message:
            'Se agotó el tiempo de espera (2 minutos). Revisa que el terminal esté en modo emisión e inténtalo de nuevo.',
        }),
      TIMEOUT_MS,
    )

    // Una consulta a la vez: la siguiente se programa cuando termina la anterior.
    const poll = async () => {
      try {
        const status = await getEmissionStatus()
        if (finished) return
        consecutiveErrors = 0
        if (status.estado === 'completado') return finish({ ok: true })
        if (status.estado === 'error') {
          return finish({
            ok: false,
            message: status.mensaje ?? 'La emisión falló. Inténtalo de nuevo.',
          })
        }
      } catch (error) {
        if (finished) return
        consecutiveErrors += 1
        if (consecutiveErrors >= MAX_CONSECUTIVE_ERRORS) {
          return finish({ ok: false, message: getErrorMessage(error) })
        }
      }
      pollTimer = setTimeout(poll, POLL_INTERVAL_MS)
    }

    void poll()

    return () => {
      finished = true
      clearTimeout(pollTimer)
      clearTimeout(timeoutTimer)
    }
  }, [])

  return (
    <div
      role="status"
      aria-live="polite"
      className="flex flex-col items-center gap-4 py-4 text-center"
    >
      <Spinner className="text-primary-700 [&>svg]:size-12" />
      <p className="text-2xl font-bold text-primary-900">
        Acerca una tarjeta virgen al terminal en modo emisión
      </p>
      <p className="text-lg text-neutral-700">
        Esperando la tarjeta… Si no se completa en 2 minutos, la emisión se cancela.
      </p>
    </div>
  )
}

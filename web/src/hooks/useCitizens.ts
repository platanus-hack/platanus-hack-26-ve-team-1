import { useCallback, useEffect, useState } from 'react'
import { getCitizens } from '../api/issuer'
import type { Citizen } from '../api/types'

type CitizensState =
  | { status: 'loading' }
  | { status: 'ready'; citizens: Citizen[] }
  | { status: 'error'; error: unknown }

export function useCitizens() {
  const [state, setState] = useState<CitizensState>({ status: 'loading' })
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    let cancelled = false
    getCitizens()
      .then((citizens) => {
        if (!cancelled) setState({ status: 'ready', citizens })
      })
      .catch((error: unknown) => {
        if (!cancelled) setState({ status: 'error', error })
      })
    return () => {
      cancelled = true
    }
  }, [attempt])

  /** Vuelve a pedir la lista mostrando el spinner. */
  const reload = useCallback(() => {
    setState({ status: 'loading' })
    setAttempt((current) => current + 1)
  }, [])

  /** Actualiza la lista en segundo plano, sin spinner (p. ej. tras emitir una cédula). */
  const refresh = useCallback(() => {
    getCitizens()
      .then((citizens) => setState({ status: 'ready', citizens }))
      .catch(() => {
        // Se conserva la lista actual; el usuario puede recargar manualmente.
      })
  }, [])

  /** Refleja un bloqueo exitoso sin volver a pedir la lista. */
  const markRevoked = useCallback((citizenId: string) => {
    setState((current) =>
      current.status === 'ready'
        ? {
            status: 'ready',
            citizens: current.citizens.map((citizen) =>
              citizen.id === citizenId ? { ...citizen, estadoCedula: 'revocada' } : citizen,
            ),
          }
        : current,
    )
  }, [])

  return { state, reload, refresh, markRevoked }
}

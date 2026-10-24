import { useCallback, useEffect, useState } from 'react'
import { getTerminals } from '../api/terminals'
import type { Terminal } from '../api/types'

type TerminalsState =
  | { status: 'loading' }
  | { status: 'ready'; terminals: Terminal[] }
  | { status: 'error'; error: unknown }

export function useTerminals() {
  const [state, setState] = useState<TerminalsState>({ status: 'loading' })
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    let cancelled = false
    getTerminals()
      .then((terminals) => {
        if (!cancelled) setState({ status: 'ready', terminals })
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

  /** Actualiza la lista en segundo plano, sin spinner (p. ej. tras crear o eliminar). */
  const refresh = useCallback(() => {
    getTerminals()
      .then((terminals) => setState({ status: 'ready', terminals }))
      .catch(() => {
        // Se conserva la lista actual; el usuario puede recargar manualmente.
      })
  }, [])

  return { state, reload, refresh }
}

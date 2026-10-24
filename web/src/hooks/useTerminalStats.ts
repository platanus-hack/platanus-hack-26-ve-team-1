import { useCallback, useEffect, useState } from 'react'
import { getTerminalStats } from '../api/terminals'
import type { TerminalStats } from '../api/types'

type StatsState =
  | { status: 'loading' }
  | { status: 'ready'; stats: TerminalStats[] }
  | { status: 'error'; error: unknown }

/** `version` cambia cuando la lista de terminales cambia, para volver a pedir los totales. */
export function useTerminalStats(version: number) {
  const [state, setState] = useState<StatsState>({ status: 'loading' })
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    let cancelled = false
    getTerminalStats()
      .then((stats) => {
        if (!cancelled) setState({ status: 'ready', stats })
      })
      .catch((error: unknown) => {
        if (!cancelled) setState({ status: 'error', error })
      })
    return () => {
      cancelled = true
    }
  }, [attempt, version])

  const reload = useCallback(() => {
    setState({ status: 'loading' })
    setAttempt((current) => current + 1)
  }, [])

  return { state, reload }
}

import { useCallback, useEffect, useState } from 'react'
import { getCardInfo } from '../api/card'
import type { CardInfo } from '../api/types'

type CardState =
  | { status: 'loading' }
  | { status: 'ready'; card: CardInfo }
  | { status: 'error'; error: unknown }

export function useCardInfo() {
  const [state, setState] = useState<CardState>({ status: 'loading' })
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    let cancelled = false
    getCardInfo()
      .then((card) => {
        if (!cancelled) setState({ status: 'ready', card })
      })
      .catch((error: unknown) => {
        if (!cancelled) setState({ status: 'error', error })
      })
    return () => {
      cancelled = true
    }
  }, [attempt])

  const reload = useCallback(() => {
    setState({ status: 'loading' })
    setAttempt((current) => current + 1)
  }, [])

  return { state, reload }
}

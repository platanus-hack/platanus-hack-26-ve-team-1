import { useCallback, useEffect, useState } from 'react'
import { getStudents } from '../api/university'
import type { Student } from '../api/types'

type StudentsState =
  | { status: 'loading' }
  | { status: 'ready'; students: Student[] }
  | { status: 'error'; error: unknown }

export function useStudents() {
  const [state, setState] = useState<StudentsState>({ status: 'loading' })
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    let cancelled = false
    getStudents()
      .then((students) => {
        if (!cancelled) setState({ status: 'ready', students })
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

  /** Actualiza la lista en segundo plano, sin spinner (p. ej. tras emitir un carnet). */
  const refresh = useCallback(() => {
    getStudents()
      .then((students) => setState({ status: 'ready', students }))
      .catch(() => {
        // Se conserva la lista actual; el usuario puede recargar manualmente.
      })
  }, [])

  return { state, reload, refresh }
}

import { Component, type ReactNode } from 'react'
import { focusRing } from '../lib/cn'

type ErrorBoundaryState = { failed: boolean }

/** Evita la pantalla en blanco si una vista falla al dibujarse. */
export class ErrorBoundary extends Component<{ children: ReactNode }, ErrorBoundaryState> {
  state: ErrorBoundaryState = { failed: false }

  static getDerivedStateFromError(): ErrorBoundaryState {
    return { failed: true }
  }

  render() {
    if (!this.state.failed) return this.props.children
    return (
      <main className="flex min-h-svh flex-col items-center justify-center gap-6 p-6 text-center">
        <h1 className="text-4xl font-bold text-primary-900">Algo salió mal</h1>
        <p className="max-w-prose text-xl text-neutral-700">
          Ocurrió un error inesperado en sIsA. Recarga la página para volver a intentarlo.
        </p>
        <button
          type="button"
          onClick={() => window.location.reload()}
          className={`min-h-12 rounded-xl bg-primary-700 px-6 py-3 text-lg font-semibold text-white hover:bg-primary-800 ${focusRing}`}
        >
          Recargar la página
        </button>
      </main>
    )
  }
}

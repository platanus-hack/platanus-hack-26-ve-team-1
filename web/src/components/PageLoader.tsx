import { Spinner } from './ui'

export function PageLoader() {
  return (
    <div className="flex min-h-svh items-center justify-center text-primary-700">
      <Spinner label="Cargando…" className="[&>svg]:size-12" />
    </div>
  )
}

import { Link } from 'react-router'
import { PageTitle } from '../components/PageTitle'
import { cn, focusRing } from '../lib/cn'

export default function NotFoundPage() {
  return (
    <div className="flex flex-col items-start gap-6">
      <PageTitle>Página no encontrada</PageTitle>
      <p className="text-lg text-neutral-700">
        La dirección que buscas no existe en sIsA.
      </p>
      <Link
        to="/"
        className={cn(
          'inline-flex min-h-12 items-center rounded-xl bg-primary-700 px-6 py-3 text-lg font-semibold text-white hover:bg-primary-800',
          focusRing,
        )}
      >
        Volver al inicio
      </Link>
    </div>
  )
}

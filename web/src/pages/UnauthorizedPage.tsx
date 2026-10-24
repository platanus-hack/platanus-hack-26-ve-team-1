import { Link } from 'react-router'
import { PageTitle } from '../components/PageTitle'
import { useAuth } from '../hooks/useAuth'
import { focusRing } from '../lib/cn'
import { HOME_BY_ROLE } from '../lib/roles'

export default function UnauthorizedPage() {
  const { user } = useAuth()

  return (
    <div className="flex flex-col items-start gap-6">
      <PageTitle>No autorizado</PageTitle>
      <p className="text-lg text-neutral-700">
        Tu cuenta no tiene permiso para ver esta sección de sIsA.
      </p>
      <Link
        to={user ? HOME_BY_ROLE[user.rol] : '/login'}
        className={`inline-flex min-h-12 items-center rounded-xl bg-primary-700 px-6 py-3 text-lg font-semibold text-white hover:bg-primary-800 ${focusRing}`}
      >
        Volver a mi inicio
      </Link>
    </div>
  )
}

import { Navigate, Outlet, useLocation } from 'react-router'
import type { Role } from '../lib/roles'
import { useAuth } from '../hooks/useAuth'
import UnauthorizedPage from '../pages/UnauthorizedPage'
import { PageLoader } from './PageLoader'

type ProtectedRouteProps = {
  /** Roles con acceso. Sin esta prop basta con tener sesión. */
  roles?: Role[]
}

export function ProtectedRoute({ roles }: ProtectedRouteProps) {
  const { user, loading } = useAuth()
  const location = useLocation()

  if (loading) return <PageLoader />
  if (!user) return <Navigate to="/login" replace state={{ from: location }} />
  if (roles && !roles.includes(user.rol)) return <UnauthorizedPage />

  return <Outlet />
}

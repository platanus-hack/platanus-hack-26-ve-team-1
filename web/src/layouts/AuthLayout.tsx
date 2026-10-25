import { Navigate, Outlet } from 'react-router'
import { Logo } from '../components/Logo'
import { PageLoader } from '../components/PageLoader'
import { Card } from '../components/ui'
import { useAuth } from '../hooks/useAuth'
import { HOME_BY_ROLE } from '../lib/roles'

export default function AuthLayout() {
  const { user, loading } = useAuth()

  if (loading) return <PageLoader />
  if (user) return <Navigate to={HOME_BY_ROLE[user.rol]} replace />

  return (
    <div className="flex min-h-svh flex-col items-center justify-center gap-8 bg-primary-900 p-6">
      <Logo className="text-5xl text-white" />
      <main className="w-full max-w-md">
        <Card className="p-8">
          <Outlet />
        </Card>
      </main>
    </div>
  )
}

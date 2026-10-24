import { Outlet } from 'react-router'
import { Logo } from '../components/Logo'
import { Card } from '../components/ui'

export default function AuthLayout() {
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

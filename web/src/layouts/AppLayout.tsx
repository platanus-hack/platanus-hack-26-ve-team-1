import { useState } from 'react'
import { NavLink, Outlet } from 'react-router'
import { Logo } from '../components/Logo'
import { Button } from '../components/ui'
import { useAuth } from '../hooks/useAuth'
import { useToast } from '../hooks/useToast'
import { cn, focusRing } from '../lib/cn'
import { getErrorMessage } from '../lib/errors'
import { NAV_BY_ROLE } from '../lib/roles'

export default function AppLayout() {
  const { user, logout } = useAuth()
  const { toast } = useToast()
  const [menuOpen, setMenuOpen] = useState(false)
  const [loggingOut, setLoggingOut] = useState(false)
  const items = user ? NAV_BY_ROLE[user.rol] : []

  // ProtectedRoute redirige a /login cuando el usuario queda en null.
  const handleLogout = async () => {
    setMenuOpen(false)
    setLoggingOut(true)
    try {
      await logout()
    } catch (error) {
      toast({
        title: 'No se pudo cerrar la sesión en el servidor',
        description: getErrorMessage(error),
        variant: 'error',
      })
    }
  }

  return (
    <div className="flex min-h-svh flex-col">
      <header className="bg-primary-900 text-white">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-x-6 gap-y-2 px-6 py-3">
          <Logo className="text-white" />

          <button
            type="button"
            aria-expanded={menuOpen}
            aria-controls="menu-principal"
            onClick={() => setMenuOpen((open) => !open)}
            className={cn(
              'min-h-12 rounded-xl border-2 border-white/60 px-4 text-lg font-semibold md:hidden',
              focusRing,
            )}
          >
            {menuOpen ? 'Cerrar menú' : 'Menú'}
          </button>

          <div
            id="menu-principal"
            className={cn(
              'w-full flex-col gap-4 pb-2 md:flex md:w-auto md:flex-1 md:flex-row md:items-center md:justify-between md:pb-0',
              menuOpen ? 'flex' : 'hidden',
            )}
          >
            <nav aria-label="Principal">
              <ul className="flex flex-col gap-1 md:flex-row md:gap-2">
                {items.map((item) => (
                  <li key={item.to}>
                    <NavLink
                      to={item.to}
                      end={item.end}
                      onClick={() => setMenuOpen(false)}
                      className={({ isActive }) =>
                        cn(
                          'flex min-h-12 items-center rounded-xl px-4 text-lg font-semibold',
                          focusRing,
                          isActive
                            ? 'bg-white text-primary-900'
                            : 'text-white hover:bg-primary-700',
                        )
                      }
                    >
                      {item.label}
                    </NavLink>
                  </li>
                ))}
              </ul>
            </nav>
            <Button
              variant="secondary"
              onClick={handleLogout}
              loading={loggingOut}
              className="self-start md:self-auto"
            >
              Cerrar sesión
            </Button>
          </div>
        </div>
      </header>

      <main className="mx-auto w-full max-w-6xl flex-1 p-6">
        <Outlet />
      </main>
    </div>
  )
}

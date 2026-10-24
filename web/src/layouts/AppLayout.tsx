import { useState } from 'react'
import { NavLink, Outlet, useNavigate } from 'react-router'
import { Logo } from '../components/Logo'
import { Button } from '../components/ui'
import { useCurrentRole } from '../hooks/useCurrentRole'
import { cn, focusRing } from '../lib/cn'
import { NAV_BY_ROLE } from '../lib/roles'

export default function AppLayout() {
  const role = useCurrentRole()
  const navigate = useNavigate()
  const [menuOpen, setMenuOpen] = useState(false)
  const items = NAV_BY_ROLE[role]

  // TODO: reemplazar por el cierre de sesión real cuando exista auth.
  const handleLogout = () => {
    setMenuOpen(false)
    navigate('/login')
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

import { Link } from 'react-router'
import { useAuth } from '../hooks/useAuth'
import { cn, focusRing } from '../lib/cn'
import { HOME_BY_ROLE } from '../lib/roles'
import { LogoMark } from './LogoMark'

export function Logo({ className }: { className?: string }) {
  const { user } = useAuth()

  return (
    <Link
      to={user ? HOME_BY_ROLE[user.rol] : '/login'}
      aria-label="sIsA, ir al inicio"
      className={cn(
        'inline-flex items-center gap-3 rounded-lg text-3xl font-extrabold tracking-tight',
        focusRing,
        className,
      )}
    >
      <LogoMark className="h-[1.3em] w-auto" />
      <span aria-hidden="true">sIsA</span>
    </Link>
  )
}

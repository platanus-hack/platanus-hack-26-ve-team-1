import { Link } from 'react-router'
import { cn, focusRing } from '../lib/cn'

export function Logo({ className }: { className?: string }) {
  return (
    <Link
      to="/"
      aria-label="sIsA, ir al inicio"
      className={cn(
        'rounded-lg text-3xl font-extrabold tracking-tight',
        focusRing,
        className,
      )}
    >
      sIsA
    </Link>
  )
}

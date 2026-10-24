import type { ComponentProps } from 'react'
import { cn, focusRing } from '../../lib/cn'
import { Spinner } from './Spinner'

export type ButtonVariant = 'primary' | 'secondary' | 'danger'

type ButtonProps = ComponentProps<'button'> & {
  variant?: ButtonVariant
  loading?: boolean
}

const variants: Record<ButtonVariant, string> = {
  primary: 'bg-primary-700 text-white hover:bg-primary-800',
  secondary:
    'border-2 border-primary-700 bg-white text-primary-800 hover:bg-primary-50',
  danger: 'bg-denied-600 text-white hover:bg-denied-700',
}

export function Button({
  variant = 'primary',
  loading = false,
  disabled,
  type = 'button',
  className,
  children,
  ...props
}: ButtonProps) {
  return (
    <button
      type={type}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      className={cn(
        'inline-flex min-h-12 items-center justify-center gap-3 rounded-xl px-6 py-3 text-lg font-semibold transition-colors',
        'disabled:cursor-not-allowed disabled:opacity-60',
        focusRing,
        variants[variant],
        className,
      )}
      {...props}
    >
      {loading && <Spinner />}
      {children}
    </button>
  )
}

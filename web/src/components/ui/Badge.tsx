import type { ComponentProps } from 'react'
import { cn } from '../../lib/cn'

export type BadgeVariant = 'ok' | 'error' | 'neutral' | 'warning'

type BadgeProps = ComponentProps<'span'> & {
  variant?: BadgeVariant
}

const variants: Record<BadgeVariant, string> = {
  ok: 'bg-approved-100 text-approved-900 ring-approved-600',
  error: 'bg-denied-100 text-denied-900 ring-denied-600',
  neutral: 'bg-neutral-100 text-neutral-800 ring-neutral-400',
  warning: 'bg-amber-100 text-amber-900 ring-amber-600',
}

export function Badge({ variant = 'neutral', className, ...props }: BadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full px-4 py-1 text-base font-semibold ring-1 ring-inset',
        variants[variant],
        className,
      )}
      {...props}
    />
  )
}

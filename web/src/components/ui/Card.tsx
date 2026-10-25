import type { ComponentProps } from 'react'
import { cn } from '../../lib/cn'

type CardProps = Omit<ComponentProps<'section'>, 'ref'> & {
  /** Elemento semántico: "article" para tarjetas independientes con su propio encabezado. */
  as?: 'section' | 'article' | 'div'
}

export function Card({ as: Component = 'section', className, ...props }: CardProps) {
  return (
    <Component
      className={cn(
        'rounded-2xl border border-neutral-200 bg-white p-6 shadow-sm',
        className,
      )}
      {...props}
    />
  )
}

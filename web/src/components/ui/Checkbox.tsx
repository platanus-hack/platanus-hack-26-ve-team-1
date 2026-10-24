import type { ComponentProps } from 'react'
import { cn, focusRing } from '../../lib/cn'

type CheckboxProps = Omit<ComponentProps<'input'>, 'type'> & {
  label: string
}

export function Checkbox({ label, className, ...props }: CheckboxProps) {
  return (
    <label className="flex min-h-12 cursor-pointer items-center gap-3 text-lg font-semibold text-neutral-900">
      <input
        type="checkbox"
        className={cn('size-6 shrink-0 accent-primary-700', focusRing, className)}
        {...props}
      />
      {label}
    </label>
  )
}

import { useId, type ComponentProps } from 'react'
import { cn, focusRing } from '../../lib/cn'

type SelectProps = Omit<ComponentProps<'select'>, 'id'> & {
  label: string
  error?: string
  hint?: string
  id?: string
}

export function Select({
  label,
  error,
  hint,
  id,
  className,
  children,
  ...props
}: SelectProps) {
  const generatedId = useId()
  const selectId = id ?? generatedId
  const hintId = `${selectId}-hint`
  const errorId = `${selectId}-error`
  const describedBy =
    [hint && hintId, error && errorId].filter(Boolean).join(' ') || undefined

  return (
    <div className="flex flex-col gap-2">
      <label htmlFor={selectId} className="text-lg font-semibold text-neutral-900">
        {label}
      </label>
      {hint && (
        <p id={hintId} className="text-base text-neutral-600">
          {hint}
        </p>
      )}
      <select
        id={selectId}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy}
        className={cn(
          'min-h-12 rounded-xl border-2 bg-white px-4 py-3 text-lg text-neutral-900',
          'disabled:cursor-not-allowed disabled:bg-neutral-100 disabled:text-neutral-600',
          error ? 'border-denied-600' : 'border-neutral-400',
          focusRing,
          className,
        )}
        {...props}
      >
        {children}
      </select>
      {error && (
        <p id={errorId} role="alert" className="text-base font-medium text-denied-700">
          {error}
        </p>
      )}
    </div>
  )
}

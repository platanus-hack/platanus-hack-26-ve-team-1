import { useId, type ComponentProps } from 'react'
import { cn, focusRing } from '../../lib/cn'

type InputProps = Omit<ComponentProps<'input'>, 'id'> & {
  label: string
  error?: string
  hint?: string
  id?: string
}

export function Input({
  label,
  error,
  hint,
  id,
  className,
  ...props
}: InputProps) {
  const generatedId = useId()
  const inputId = id ?? generatedId
  const hintId = `${inputId}-hint`
  const errorId = `${inputId}-error`
  const describedBy =
    [hint && hintId, error && errorId].filter(Boolean).join(' ') || undefined

  return (
    <div className="flex flex-col gap-2">
      <label htmlFor={inputId} className="text-lg font-semibold text-neutral-900">
        {label}
      </label>
      {hint && (
        <p id={hintId} className="text-base text-neutral-600">
          {hint}
        </p>
      )}
      <input
        id={inputId}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy}
        className={cn(
          'min-h-12 rounded-xl border-2 bg-white px-4 py-3 text-lg text-neutral-900 placeholder:text-neutral-500',
          'disabled:cursor-not-allowed disabled:bg-neutral-100 disabled:text-neutral-600',
          error ? 'border-denied-600' : 'border-neutral-400',
          focusRing,
          className,
        )}
        {...props}
      />
      {error && (
        <p id={errorId} role="alert" className="text-base font-medium text-denied-700">
          {error}
        </p>
      )}
    </div>
  )
}

import { cn } from '../../lib/cn'

type SpinnerProps = {
  /** Texto para lectores de pantalla. Sin él, el spinner es decorativo. */
  label?: string
  className?: string
}

export function Spinner({ label, className }: SpinnerProps) {
  return (
    <span
      role={label ? 'status' : undefined}
      aria-hidden={label ? undefined : true}
      className={cn('inline-flex', className)}
    >
      <svg
        className="size-6 animate-spin"
        viewBox="0 0 24 24"
        fill="none"
        aria-hidden="true"
      >
        <circle
          cx="12"
          cy="12"
          r="10"
          stroke="currentColor"
          strokeWidth="4"
          className="opacity-25"
        />
        <path
          d="M22 12a10 10 0 0 0-10-10"
          stroke="currentColor"
          strokeWidth="4"
          strokeLinecap="round"
        />
      </svg>
      {label && <span className="sr-only">{label}</span>}
    </span>
  )
}

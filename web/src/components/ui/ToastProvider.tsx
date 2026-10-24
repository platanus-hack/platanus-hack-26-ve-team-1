import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { cn, focusRing } from '../../lib/cn'
import {
  ToastContext,
  type ToastOptions,
  type ToastVariant,
} from './toast-context'

const AUTO_DISMISS_MS = 6000

type ToastItem = ToastOptions & { id: number }

const variants: Record<ToastVariant, string> = {
  ok: 'border-approved-600 bg-approved-50 text-approved-900',
  error: 'border-denied-600 bg-denied-50 text-denied-900',
  neutral: 'border-neutral-400 bg-white text-neutral-900',
}

let nextId = 0

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([])

  const dismiss = useCallback((id: number) => {
    setItems((current) => current.filter((item) => item.id !== id))
  }, [])

  const toast = useCallback((options: ToastOptions) => {
    setItems((current) => [...current, { ...options, id: nextId++ }])
  }, [])

  const value = useMemo(() => ({ toast }), [toast])

  return (
    <ToastContext value={value}>
      {children}
      <div className="fixed right-4 bottom-4 z-50 flex w-[min(92vw,26rem)] flex-col gap-3">
        {items.map((item) => (
          <ToastMessage key={item.id} item={item} onDismiss={dismiss} />
        ))}
      </div>
    </ToastContext>
  )
}

function ToastMessage({
  item,
  onDismiss,
}: {
  item: ToastItem
  onDismiss: (id: number) => void
}) {
  const { id, title, description, variant = 'neutral' } = item

  useEffect(() => {
    const timer = setTimeout(() => onDismiss(id), AUTO_DISMISS_MS)
    return () => clearTimeout(timer)
  }, [id, onDismiss])

  return (
    <div
      role={variant === 'error' ? 'alert' : 'status'}
      className={cn(
        'flex items-start gap-3 rounded-xl border-2 p-4 shadow-lg',
        variants[variant],
      )}
    >
      <div className="flex-1">
        <p className="text-lg font-bold">{title}</p>
        {description && <p className="text-base">{description}</p>}
      </div>
      <button
        type="button"
        onClick={() => onDismiss(id)}
        aria-label="Cerrar notificación"
        className={cn(
          '-m-1 flex size-10 shrink-0 items-center justify-center rounded-lg text-2xl leading-none hover:bg-black/10',
          focusRing,
        )}
      >
        <span aria-hidden="true">×</span>
      </button>
    </div>
  )
}

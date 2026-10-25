import { createContext } from 'react'

export type ToastVariant = 'ok' | 'error' | 'neutral'

export type ToastOptions = {
  title: string
  description?: string
  variant?: ToastVariant
}

export type ToastContextValue = {
  toast: (options: ToastOptions) => void
}

export const ToastContext = createContext<ToastContextValue | null>(null)

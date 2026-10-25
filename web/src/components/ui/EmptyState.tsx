import type { ReactNode } from 'react'

type EmptyStateProps = {
  title: string
  description?: string
  action?: ReactNode
}

export function EmptyState({ title, description, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center gap-4 rounded-2xl border-2 border-dashed border-neutral-300 px-6 py-12 text-center">
      <h2 className="text-2xl font-bold text-neutral-900">{title}</h2>
      {description && (
        <p className="max-w-prose text-lg text-neutral-600">{description}</p>
      )}
      {action}
    </div>
  )
}

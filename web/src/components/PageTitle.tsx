import type { ReactNode } from 'react'

export function PageTitle({ children }: { children: ReactNode }) {
  return <h1 className="text-4xl font-bold text-primary-900">{children}</h1>
}

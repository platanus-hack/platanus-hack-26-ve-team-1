import type { Role } from '../api/types'

export type { Role }

export type NavItem = {
  to: string
  label: string
  end?: boolean
}

export const NAV_BY_ROLE: Record<Role, NavItem[]> = {
  ciudadano: [
    { to: '/', label: 'Inicio', end: true },
    { to: '/billetera', label: 'Billetera' },
  ],
  emisor: [{ to: '/emisor', label: 'Panel del emisor' }],
  comercio: [{ to: '/comercio', label: 'Terminales' }],
}

export function isRole(value: string | null): value is Role {
  return value === 'ciudadano' || value === 'emisor' || value === 'comercio'
}

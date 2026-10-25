export const ROLES = {
  CIUDADANO: 'ciudadano',
  EMISOR: 'emisor',
  COMERCIO: 'comercio',
} as const

export type Role = (typeof ROLES)[keyof typeof ROLES]

export type NavItem = {
  to: string
  label: string
  end?: boolean
}

export const NAV_BY_ROLE: Record<Role, NavItem[]> = {
  [ROLES.CIUDADANO]: [
    { to: '/', label: 'Inicio', end: true },
    { to: '/billetera', label: 'Billetera' },
  ],
  [ROLES.EMISOR]: [{ to: '/emisor', label: 'Panel del emisor' }],
  [ROLES.COMERCIO]: [{ to: '/comercio', label: 'Terminales' }],
}

export const HOME_BY_ROLE: Record<Role, string> = {
  [ROLES.CIUDADANO]: '/',
  [ROLES.EMISOR]: '/emisor',
  [ROLES.COMERCIO]: '/comercio',
}

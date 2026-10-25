import type { Attribute, TerminalRole } from '../api/types'

export type TerminalRoleInfo = {
  label: string
  description: string
  /** Si hay varias opciones, el usuario elige una; si no, los datos son fijos. */
  atributos: Attribute[]
  choice: boolean
  /** Aviso que se muestra al configurar o ver el terminal. */
  warning?: string
}

export const TERMINAL_ROLES: TerminalRole[] = ['comercio', 'registro', 'salud', 'emergencia']

// El dato que pide el terminal depende de su rol.
export const TERMINAL_ROLE_INFO: Record<TerminalRole, TerminalRoleInfo> = {
  comercio: {
    label: 'Comercio',
    description: 'Verifica si la persona es mayor de edad.',
    atributos: ['mayor_de_18', 'mayor_de_21'],
    choice: true,
  },
  registro: {
    label: 'Registro',
    description: 'Hoteles, bancos y hospitales que necesitan identificar a la persona.',
    atributos: ['identidad'],
    choice: false,
  },
  salud: {
    label: 'Salud',
    description: 'Atención de salud y tránsito.',
    atributos: ['sangre_alergias', 'contacto_emergencia', 'donante_licencia'],
    choice: false,
  },
  emergencia: {
    label: 'Emergencia',
    description: 'Lee los datos vitales cuando la persona no puede dar su consentimiento.',
    atributos: ['sangre_alergias', 'contacto_emergencia'],
    choice: false,
    warning:
      'Los terminales de emergencia leen estos datos sin pedir consentimiento a la persona. Úsalos solo en emergencias reales.',
  },
}

/** Datos que pide un terminal al crearlo con ese rol (el primero, si hay opciones). */
export function defaultAtributos(role: TerminalRole): Attribute[] {
  const info = TERMINAL_ROLE_INFO[role]
  return info.choice ? [info.atributos[0]] : info.atributos
}

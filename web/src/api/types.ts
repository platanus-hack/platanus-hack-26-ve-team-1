export type Role = 'ciudadano' | 'emisor' | 'comercio'

export type Attribute = 'mayor_de_18' | 'mayor_de_21' | 'cedula_vigente'

/**
 * Contrato JSON que el frontend espera del backend (P1) en register, login y GET /me:
 * `{ id, email, nombre, rol, telefono, avisosActivados }`
 * con `rol` en "ciudadano" | "emisor" | "comercio" y `telefono` string o null.
 */
export type User = {
  id: string
  email: string
  nombre: string
  rol: Role
  telefono: string | null
  avisosActivados: boolean
}

export type CardStatus = 'activa' | 'revocada'

export type CardInfo = {
  id: string
  estado: CardStatus
  emitidaEn: string
  atributos: Attribute[]
}

export type VerificationResult = 'aprobado' | 'denegado'

export type EventItem = {
  id: string
  local: string
  atributo: Attribute
  resultado: VerificationResult
  /** Fecha y hora en formato ISO 8601. */
  hora: string
  emergencia: boolean
}

export type Terminal = {
  id: string
  nombre: string
  rol: Exclude<Role, 'ciudadano'>
  atributo: Attribute
  /** Solo se entrega al crear o regenerar el terminal. */
  token?: string
}

/** Ciudadano tal como lo ve el emisor. */
export type Citizen = {
  id: string
  nombre: string
  email: string
  edad: number
  cedula: CardStatus
}

export type LoginInput = {
  email: string
  password: string
}

export type RegisterInput = LoginInput & {
  nombre: string
  telefono?: string
}

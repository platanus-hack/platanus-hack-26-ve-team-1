import type { BloodType } from '../lib/optionalData'
import type { ROLES, Role } from '../lib/roles'

/** Valores exactos del contrato con P1: 'ciudadano' | 'emisor' | 'comercio' (minúscula, sin tilde). */
export type { Role }

export type Attribute =
  | 'mayor_de_18'
  | 'mayor_de_21'
  | 'identidad'
  | 'sangre_alergias'
  | 'contacto_emergencia'
  | 'donante_licencia'

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
  rol: Exclude<Role, typeof ROLES.CIUDADANO>
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

/** Datos opcionales que el ciudadano pide escribir en su chip. */
export type OptionalData = {
  tipoSangre: BloodType | null
  alergias: string
  /** Teléfono en formato internacional (E.164), p. ej. "+56912345678". */
  contactoEmergencia: { nombre: string; telefono: string }
  donante: boolean
}

export type LoginInput = {
  email: string
  password: string
}

export type RegisterInput = LoginInput & {
  nombre: string
  telefono?: string
}

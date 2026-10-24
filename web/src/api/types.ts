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

/** Código de resultado del backend (1–6); ver RESULT_INFO en lib/events.ts. */
export type EventResult = 1 | 2 | 3 | 4 | 5 | 6

/** Código de atributo del backend (1–6); ver EVENT_ATTRIBUTE_LABELS en lib/events.ts. */
export type EventAttribute = 1 | 2 | 3 | 4 | 5 | 6

/** Evento ya normalizado (ver normalizeEvent en lib/events.ts). */
export type EventItem = {
  id: number
  local: string
  /** null si el backend envió un código no válido. */
  atributo: EventAttribute | null
  /** null si el backend envió un código no válido. */
  resultado: EventResult | null
  /** Fecha y hora en formato ISO 8601; vacío si no vino o no es válida. */
  hora: string
  /** true en accesos de emergencia (no es un código de resultado). */
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

/** Estado de la cédula de un ciudadano, visto por el emisor. */
export type CitizenCardStatus = CardStatus | 'sin_emitir'

/** Ciudadano tal como lo ve el emisor. `id` es interno: nunca se muestra. */
export type Citizen = {
  id: string
  nombre: string
  email: string
  estadoCedula: CitizenCardStatus
  /** Fecha de emisión en ISO 8601; null si aún no tiene cédula. */
  emitidaEn: string | null
}

export type EmissionState = 'pendiente' | 'completado' | 'error'

/** Estado de la emisión en curso en el terminal. */
export type EmissionStatus = {
  estado: EmissionState
  /** Detalle en español cuando estado es "error". */
  mensaje?: string
}

/** Datos opcionales que el ciudadano pide escribir en su chip. */
export type OptionalData = {
  tipoSangre: BloodType | null
  alergias: string
  /** Teléfono en formato internacional (E.164), p. ej. "+56912345678". */
  contactoEmergencia: { nombre: string; telefono: string }
  donante: boolean
}

/** Avisos por WhatsApp: el teléfono es User.telefono (distinto del contacto de emergencia). */
export type NotificationSettings = {
  /** Formato internacional sin espacios, p. ej. "+56912345678"; null si no hay. */
  telefono: string | null
  avisosActivados: boolean
}

export type LoginInput = {
  email: string
  password: string
}

export type RegisterInput = LoginInput & {
  nombre: string
  telefono?: string
}

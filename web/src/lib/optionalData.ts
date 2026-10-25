import type { OptionalData } from '../api/types'
import { normalizePhone } from './validation'

export const BLOOD_TYPES = ['A+', 'A-', 'B+', 'B-', 'AB+', 'AB-', 'O+', 'O-'] as const

export type BloodType = (typeof BLOOD_TYPES)[number]

export const EMPTY_OPTIONAL_DATA: OptionalData = {
  tipoSangre: null,
  alergias: '',
  contactoEmergencia: { nombre: '', telefono: '' },
  donante: false,
}

/**
 * Cómo se compone el texto del contacto de emergencia (A5) que se escribe en el chip:
 * "<nombre><separador><teléfono sin '+'>", por ejemplo "Mamá 04141234567".
 * SUPUESTO pendiente de confirmar con P1 (ver docs/PENDIENTES_P1.md): si el chip guarda
 * nombre y teléfono juntos o por separado, y si el teléfono lleva o no código de país.
 */
export const EMERGENCY_CONTACT_CHIP_SEPARATOR = ' '

export function composeEmergencyContactChipText(contact: {
  nombre: string
  telefono: string
}): string {
  const nombre = contact.nombre.trim()
  const telefono = normalizePhone(contact.telefono).replace(/^\+/, '')
  return [nombre, telefono].filter(Boolean).join(EMERGENCY_CONTACT_CHIP_SEPARATOR)
}

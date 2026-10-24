import type { OptionalData } from '../api/types'

export const BLOOD_TYPES = ['A+', 'A-', 'B+', 'B-', 'AB+', 'AB-', 'O+', 'O-'] as const

export type BloodType = (typeof BLOOD_TYPES)[number]

export const EMPTY_OPTIONAL_DATA: OptionalData = {
  tipoSangre: null,
  alergias: '',
  contactoEmergencia: { nombre: '', telefono: '' },
  donante: false,
}

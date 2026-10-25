import type { Attribute } from '../api/types'

export type AttributeInfo = {
  nombre: string
  descripcion: string
  /** Quién puede leer este atributo. */
  lector: string
}

export const ATTRIBUTE_INFO: Record<Attribute, AttributeInfo> = {
  mayor_de_18: {
    nombre: 'Mayor de 18',
    descripcion: 'Responde solo sí o no. No revela tu fecha de nacimiento.',
    lector: 'Comercio',
  },
  mayor_de_21: {
    nombre: 'Mayor de 21',
    descripcion: 'Responde solo sí o no. No revela tu fecha de nacimiento.',
    lector: 'Comercio',
  },
  identidad: {
    nombre: 'Identidad',
    descripcion: 'Nombre, número V y vigencia de tu cédula.',
    lector: 'Registro (hotel, banco, hospital)',
  },
  sangre_alergias: {
    nombre: 'Tipo de sangre y alergias',
    descripcion: 'Para que te atiendan de forma segura en una urgencia.',
    lector: 'Salud',
  },
  contacto_emergencia: {
    nombre: 'Contacto de emergencia',
    descripcion: 'La persona a quien avisar si algo te ocurre.',
    lector: 'Salud',
  },
  donante_licencia: {
    nombre: 'Donante y licencia de conducir',
    descripcion: 'Si eres donante y si tu licencia de conducir está vigente.',
    lector: 'Salud y tránsito',
  },
}

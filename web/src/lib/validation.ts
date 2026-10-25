const PHONE_PATTERN = /^\+[1-9]\d{7,14}$/

/** Límite de los textos que se escriben en el chip: alergias (A4) y contacto de emergencia (A5). */
export const CHIP_TEXT_MAX_BYTES = 16

const textEncoder = new TextEncoder()

/** Cuenta bytes UTF-8, no caracteres: "á" y "ñ" ocupan 2 y un emoji hasta 4. */
export function countBytes(text: string): number {
  return textEncoder.encode(text).length
}

export function validateMaxBytes(text: string, maxBytes: number): string | undefined {
  const excess = countBytes(text) - maxBytes
  if (excess > 0) {
    return `Máximo ${maxBytes} bytes (te pasaste por ${excess}). Usa un nombre corto.`
  }
}

/** Deja "+" y dígitos (E.164): quita espacios, guiones, puntos y paréntesis. "+56 9 1234-5678" → "+56912345678". */
export function normalizePhone(value: string): string {
  return value.replace(/[\s\-().]/g, '')
}

/**
 * La misma validación se reutiliza para dos campos independientes, sin estado compartido:
 * el teléfono de contacto de emergencia (OptionalData) y el WhatsApp del usuario (User.telefono).
 */
export function validatePhone(value: string): string | undefined {
  const phone = normalizePhone(value)
  if (!phone) return 'Ingresa un teléfono.'
  if (!PHONE_PATTERN.test(phone)) {
    return 'Usa el formato internacional, por ejemplo +56 9 1234 5678.'
  }
}

const MAX_AGE_YEARS = 120

/** Fecha de nacimiento en formato YYYY-MM-DD (valor de un input date). */
export function validateBirthDate(value: string): string | undefined {
  if (!value) return 'Ingresa la fecha de nacimiento.'
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
  if (!match) return 'Usa una fecha válida.'
  const [year, month, day] = match.slice(1).map(Number)
  const date = new Date(year, month - 1, day)
  if (date.getFullYear() !== year || date.getMonth() !== month - 1 || date.getDate() !== day) {
    return 'Usa una fecha válida.'
  }
  const now = new Date()
  if (date > now) return 'La fecha de nacimiento no puede ser futura.'
  const oldest = new Date(now.getFullYear() - MAX_AGE_YEARS, now.getMonth(), now.getDate())
  if (date < oldest) return `La fecha no es plausible (más de ${MAX_AGE_YEARS} años).`
}

export const CEDULA_MIN_DIGITS = 6
export const CEDULA_MAX_DIGITS = 9

/** Número de cédula: solo dígitos, sin letras, puntos ni guiones. */
export function validateCedulaNumber(value: string): string | undefined {
  const cedula = value.trim()
  if (!cedula) return 'Ingresa el número de cédula.'
  if (!new RegExp(`^\\d{${CEDULA_MIN_DIGITS},${CEDULA_MAX_DIGITS}}$`).test(cedula)) {
    return `Usa solo dígitos (de ${CEDULA_MIN_DIGITS} a ${CEDULA_MAX_DIGITS}), sin puntos ni letras.`
  }
}

export const MAX_TERMINAL_NAME_LENGTH = 60

export function validateTerminalName(value: string): string | undefined {
  const name = value.trim()
  if (!name) return 'Ingresa un nombre para el terminal.'
  if (name.length > MAX_TERMINAL_NAME_LENGTH) {
    return `El nombre no puede superar los ${MAX_TERMINAL_NAME_LENGTH} caracteres.`
  }
}

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

export const MIN_PASSWORD_LENGTH = 8

export function validateEmail(value: string): string | undefined {
  const email = value.trim()
  if (!email) return 'Ingresa tu correo.'
  if (!EMAIL_PATTERN.test(email)) return 'Ingresa un correo válido.'
}

export function validateName(value: string): string | undefined {
  if (!value.trim()) return 'Ingresa tu nombre.'
}

export function validatePassword(
  value: string,
  { checkLength = false }: { checkLength?: boolean } = {},
): string | undefined {
  if (!value) return 'Ingresa tu contraseña.'
  if (checkLength && value.length < MIN_PASSWORD_LENGTH) {
    return `La contraseña debe tener al menos ${MIN_PASSWORD_LENGTH} caracteres.`
  }
}

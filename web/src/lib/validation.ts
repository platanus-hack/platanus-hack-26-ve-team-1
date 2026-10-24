const PHONE_PATTERN = /^\+[1-9]\d{7,14}$/

export const MAX_ALLERGIES_LENGTH = 200

/** Quita espacios, guiones, puntos y paréntesis: "+56 9 1234-5678" → "+56912345678". */
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

export function validateAllergies(value: string): string | undefined {
  if (value.trim().length > MAX_ALLERGIES_LENGTH) {
    return `Las alergias no pueden superar los ${MAX_ALLERGIES_LENGTH} caracteres.`
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

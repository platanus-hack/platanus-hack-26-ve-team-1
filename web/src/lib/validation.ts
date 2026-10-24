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

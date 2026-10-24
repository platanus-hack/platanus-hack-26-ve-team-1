import { ApiError } from '../api/client'

export function getErrorMessage(error: unknown): string {
  if (error instanceof ApiError) return error.message
  return 'Ocurrió un error inesperado. Inténtalo de nuevo.'
}

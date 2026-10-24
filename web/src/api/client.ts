export const UNAUTHORIZED_EVENT = 'sisa:unauthorized'

export const API_BASE = '/api'

export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

export function emitUnauthorized(): void {
  window.dispatchEvent(new Event(UNAUTHORIZED_EVENT))
}

type RequestOptions = {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  body?: unknown
  /** Desactívalo en endpoints donde un 401 es una respuesta esperada (p. ej. login). */
  signalUnauthorized?: boolean
  /** Cabeceras extra, p. ej. `Authorization: Bearer <token>` del terminal en POST /events. */
  headers?: Record<string, string>
}

const DEFAULT_MESSAGES: Record<number, string> = {
  400: 'La solicitud no es válida.',
  401: 'Tu sesión no es válida. Inicia sesión de nuevo.',
  403: 'No tienes permiso para hacer esto.',
  404: 'No encontramos lo que buscas.',
  409: 'Ya existe un registro con esos datos.',
}

async function readErrorMessage(response: Response): Promise<string> {
  try {
    const data: unknown = await response.json()
    if (typeof data === 'object' && data !== null) {
      const { error, message } = data as Record<string, unknown>
      if (typeof error === 'string') return error
      if (typeof message === 'string') return message
    }
  } catch {
    // El cuerpo no es JSON: se usa un mensaje genérico.
  }
  return (
    DEFAULT_MESSAGES[response.status] ??
    'Ocurrió un error en el servidor. Inténtalo de nuevo.'
  )
}

export async function request<T>(
  path: string,
  { method = 'GET', body, signalUnauthorized = true, headers }: RequestOptions = {},
): Promise<T> {
  let response: Response
  try {
    response = await fetch(`${API_BASE}${path}`, {
      method,
      credentials: 'include',
      headers: {
        Accept: 'application/json',
        ...(body !== undefined && { 'Content-Type': 'application/json' }),
        ...headers,
      },
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch {
    throw new ApiError(0, 'No se pudo conectar con el servidor.')
  }

  if (!response.ok) {
    if (response.status === 401 && signalUnauthorized) emitUnauthorized()
    throw new ApiError(response.status, await readErrorMessage(response))
  }

  if (response.status === 204) return undefined as T
  return (await response.json()) as T
}

import { request } from './client'
import {
  mockCreateTerminal,
  mockDeleteTerminal,
  mockGetStats,
  mockGetTerminals,
  mockUpdateTerminal,
} from './mock'
import type { Terminal, TerminalInput, TerminalStats } from './types'

const useMock = import.meta.env.VITE_USE_MOCK === 'true'

// Rutas relativas a la base "/api" que añade client.ts.
// Todas son supuestos del frontend: pendientes de confirmar con P1 (ver docs/PENDIENTES_P1.md).
export const TERMINAL_ENDPOINTS = {
  // GET → lista de terminales del comercio (sin token). POST → crea uno y devuelve su token una vez.
  terminals: '/terminals',
  // PUT → edita un terminal (sin token). DELETE → lo elimina e invalida su token.
  terminal: (id: string) => `/terminals/${encodeURIComponent(id)}`,
  // GET → totales de uso por terminal: { terminalId, hoy, aprobadas, rechazadas }.
  stats: '/stats',
} as const

export function getTerminals(): Promise<Terminal[]> {
  if (useMock) return mockGetTerminals()
  return request<Terminal[]>(TERMINAL_ENDPOINTS.terminals)
}

/** La respuesta trae el token en claro: quien llama debe mostrarlo una vez y no conservarlo. */
export function createTerminal(input: TerminalInput): Promise<Terminal> {
  if (useMock) return mockCreateTerminal(input)
  return request<Terminal>(TERMINAL_ENDPOINTS.terminals, { method: 'POST', body: input })
}

export function updateTerminal(id: string, input: TerminalInput): Promise<Terminal> {
  if (useMock) return mockUpdateTerminal(id, input)
  return request<Terminal>(TERMINAL_ENDPOINTS.terminal(id), { method: 'PUT', body: input })
}

export function deleteTerminal(id: string): Promise<void> {
  if (useMock) return mockDeleteTerminal(id)
  return request<void>(TERMINAL_ENDPOINTS.terminal(id), { method: 'DELETE' })
}

export function getTerminalStats(): Promise<TerminalStats[]> {
  if (useMock) return mockGetStats()
  return request<TerminalStats[]>(TERMINAL_ENDPOINTS.stats)
}

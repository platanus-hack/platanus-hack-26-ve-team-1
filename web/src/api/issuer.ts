import { request } from './client'
import { mockGetCitizens, mockGetEmissionStatus, mockOrderEmission } from './mock'
import type { Citizen, EmissionStatus } from './types'

const useMock = import.meta.env.VITE_USE_MOCK === 'true'

// Rutas relativas a la base "/api" que añade client.ts.
// Todas son supuestos del frontend: pendientes de confirmar con P1 (ver docs/PENDIENTES_P1.md).
export const ISSUER_ENDPOINTS = {
  // GET → lista de ciudadanos (solo rol emisor).
  citizens: '/citizens',
  // POST { ciudadanoId } → inicia una emisión; el terminal queda esperando una tarjeta virgen.
  emissionOrder: '/emision/order',
  // GET → estado de la emisión en curso del emisor: { estado, mensaje? }.
  emissionStatus: '/emision/status',
} as const

export function getCitizens(): Promise<Citizen[]> {
  if (useMock) return mockGetCitizens()
  return request<Citizen[]>(ISSUER_ENDPOINTS.citizens)
}

export function orderEmission(citizenId: string): Promise<void> {
  if (useMock) return mockOrderEmission(citizenId)
  return request<void>(ISSUER_ENDPOINTS.emissionOrder, {
    method: 'POST',
    body: { ciudadanoId: citizenId },
  })
}

export function getEmissionStatus(): Promise<EmissionStatus> {
  if (useMock) return mockGetEmissionStatus()
  return request<EmissionStatus>(ISSUER_ENDPOINTS.emissionStatus)
}

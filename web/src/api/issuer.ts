import { request } from './client'
import {
  mockGetCitizens,
  mockGetEmissionStatus,
  mockOrderEmission,
  mockUpdateCitizenProfile,
} from './mock'
import type { Citizen, CitizenProfileInput, EmissionStatus } from './types'

const useMock = import.meta.env.VITE_USE_MOCK === 'true'

// Rutas relativas a la base "/api" que añade client.ts.
// Todas son supuestos del frontend: pendientes de confirmar con P1 (ver docs/PENDIENTES_P1.md).
export const ISSUER_ENDPOINTS = {
  // GET → lista de ciudadanos (solo rol emisor).
  citizens: '/citizens',
  // PUT { fechaNacimiento, cedula } → completa el perfil del ciudadano (solo rol emisor).
  citizenProfile: (id: string) => `/citizens/${encodeURIComponent(id)}/profile`,
  // POST { ciudadanoId } → inicia una emisión; el terminal queda esperando una tarjeta virgen.
  emissionOrder: '/emision/order',
  // GET → estado de la emisión en curso del emisor: { estado, mensaje? }.
  emissionStatus: '/emision/status',
} as const

export function getCitizens(): Promise<Citizen[]> {
  if (useMock) return mockGetCitizens()
  return request<Citizen[]>(ISSUER_ENDPOINTS.citizens)
}

/** Datos sensibles: solo viajan en el cuerpo de la petición; nunca se registran ni se guardan en el cliente. */
export function updateCitizenProfile(id: string, profile: CitizenProfileInput): Promise<void> {
  if (useMock) return mockUpdateCitizenProfile(id, profile)
  return request<void>(ISSUER_ENDPOINTS.citizenProfile(id), { method: 'PUT', body: profile })
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

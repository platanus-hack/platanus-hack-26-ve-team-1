import { request } from './client'
import { mockGetOptionalData, mockSaveOptionalData } from './mock'
import type { OptionalData } from './types'

const useMock = import.meta.env.VITE_USE_MOCK === 'true'

// Rutas relativas a la base "/api" que añade client.ts.
export const OPTIONAL_DATA_ENDPOINTS = {
  // Contrato: PUT acordado; el GET para precargar el formulario está pendiente de confirmar con P1.
  optionalData: '/me/optional-data',
} as const

export function getOptionalData(): Promise<OptionalData> {
  if (useMock) return mockGetOptionalData()
  return request<OptionalData>(OPTIONAL_DATA_ENDPOINTS.optionalData)
}

export function saveOptionalData(data: OptionalData): Promise<OptionalData> {
  if (useMock) return mockSaveOptionalData(data)
  return request<OptionalData>(OPTIONAL_DATA_ENDPOINTS.optionalData, {
    method: 'PUT',
    body: data,
  })
}

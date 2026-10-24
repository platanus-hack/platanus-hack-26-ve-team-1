import { request } from './client'
import { mockGetCardInfo } from './mock'
import type { CardInfo } from './types'

const useMock = import.meta.env.VITE_USE_MOCK === 'true'

// Rutas relativas a la base "/api" que añade client.ts.
export const CARD_ENDPOINTS = {
  // Contrato: pendiente de confirmar con P1.
  info: '/card',
} as const

export function getCardInfo(): Promise<CardInfo> {
  if (useMock) return mockGetCardInfo()
  return request<CardInfo>(CARD_ENDPOINTS.info)
}

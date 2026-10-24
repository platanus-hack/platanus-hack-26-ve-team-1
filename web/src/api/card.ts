import { request } from './client'
import { mockGetCardInfo, mockRevokeCard } from './mock'
import type { CardInfo } from './types'

const useMock = import.meta.env.VITE_USE_MOCK === 'true'

// Rutas relativas a la base "/api" que añade client.ts.
export const CARD_ENDPOINTS = {
  // Contrato: pendiente de confirmar con P1.
  info: '/card',
  // Contrato acordado: bloquea (revoca) la cédula del usuario autenticado.
  revoke: '/revoke',
} as const

export function getCardInfo(): Promise<CardInfo> {
  if (useMock) return mockGetCardInfo()
  return request<CardInfo>(CARD_ENDPOINTS.info)
}

// Contrato: sin cuerpo. La API identifica la cédula por la sesión (cookie), así que el front
// nunca envía handle ni UID.
export function revokeCard(): Promise<void> {
  if (useMock) return mockRevokeCard()
  return request<void>(CARD_ENDPOINTS.revoke, { method: 'POST' })
}

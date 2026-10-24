import { request } from './client'
import { mockGetCardInfo, mockRevokeCard } from './mock'
import type { CardInfo } from './types'

const useMock = import.meta.env.VITE_USE_MOCK === 'true'

// Rutas relativas a la base "/api" que añade client.ts.
export const CARD_ENDPOINTS = {
  // Contrato: pendiente de confirmar con P1.
  info: '/card',
  // Contrato acordado: bloquea (revoca) una cédula.
  revoke: '/revoke',
} as const

export function getCardInfo(): Promise<CardInfo> {
  if (useMock) return mockGetCardInfo()
  return request<CardInfo>(CARD_ENDPOINTS.info)
}

/**
 * Sin argumentos, el ciudadano bloquea su propia cédula: sin cuerpo, la API la identifica por la
 * sesión y el front nunca envía handle ni UID. Con `citizenId`, el emisor revoca la de un
 * ciudadano (cuerpo { ciudadanoId }; supuesto pendiente de confirmar con P1).
 */
export function revokeCard(citizenId?: string): Promise<void> {
  if (useMock) return mockRevokeCard(citizenId)
  return request<void>(CARD_ENDPOINTS.revoke, {
    method: 'POST',
    body: citizenId === undefined ? undefined : { ciudadanoId: citizenId },
  })
}

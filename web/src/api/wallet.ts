import type { VerifEvent } from '../lib/slj/events.ts'
import type { SendResult } from '../lib/slj/outbox.ts'
import { ApiError, request } from './client'
import {
  mockConfirmLink,
  mockGetIssuers,
  mockGetRevocations,
  mockGetWallet,
  mockPostEvent,
} from './walletMock'

const useMock = import.meta.env.VITE_USE_MOCK === 'true'

// Rutas relativas a la base "/api" que añade client.ts. Contrato de P1 (docs/contrato.md, rama p1-backend).
export const WALLET_ENDPOINTS = {
  // POST { code, public_key } → { credentials: [{ a, blob_hex, expires }] }. Con sesión de ciudadano.
  linkConfirm: '/link/confirm',
  // GET ?public_key=<hex66> → { credentials: [{ a, issuer, blob_hex, expires }] }. Con sesión.
  wallet: '/wallet',
  // GET público → { "0x01": JWK, "0x02": JWK }.
  issuers: '/issuers',
  // GET público → { hs: ["<handle de 16 hex>", ...] }.
  revocations: '/revocations',
  // POST { t: "verif", h, a, res, ts, emg } con `Authorization: Bearer <token del terminal>`.
  events: '/events',
} as const

/** Lista cruda de credenciales; `credentialsFromResponse` valida cada blob. */
export async function confirmLink(code: string, publicKeyHex: string): Promise<unknown> {
  const response = useMock
    ? await mockConfirmLink(code, publicKeyHex)
    : await request<{ credentials?: unknown }>(WALLET_ENDPOINTS.linkConfirm, {
        method: 'POST',
        body: { code, public_key: publicKeyHex },
      })
  return response.credentials
}

export async function getWalletCredentials(publicKeyHex: string): Promise<unknown> {
  const response = useMock
    ? await mockGetWallet(publicKeyHex)
    : await request<{ credentials?: unknown }>(
        `${WALLET_ENDPOINTS.wallet}?public_key=${encodeURIComponent(publicKeyHex)}`,
      )
  return response.credentials
}

export function getIssuers(): Promise<Record<string, JsonWebKey>> {
  if (useMock) return mockGetIssuers()
  return request<Record<string, JsonWebKey>>(WALLET_ENDPOINTS.issuers, { signalUnauthorized: false })
}

/** Handles revocados, cada uno de 16 caracteres hex. */
export async function getRevocations(): Promise<string[]> {
  const response = useMock
    ? await mockGetRevocations()
    : await request<{ hs?: unknown }>(WALLET_ENDPOINTS.revocations, { signalUnauthorized: false })
  return Array.isArray(response.hs) ? response.hs.filter((h): h is string => typeof h === 'string') : []
}

/**
 * Envía un evento del verificador. Un 401 aquí es un token de terminal inválido, no una sesión caída:
 * no debe cerrar la sesión del comercio.
 */
export async function postVerifEvent(event: VerifEvent, token: string): Promise<void> {
  if (useMock) return mockPostEvent()
  await request<void>(WALLET_ENDPOINTS.events, {
    method: 'POST',
    body: event,
    headers: { Authorization: `Bearer ${token}` },
    signalUnauthorized: false,
  })
}

/**
 * Traduce el resultado de enviar un evento al contrato del outbox. 400 y 404 son rechazos que no
 * cambian si se reintenta (se descartan); 401/403 (token mal o revocado), 429, 5xx y los errores de
 * red se reintentan: el comercio puede corregir el token y no se pierde el historial.
 */
export async function sendQueuedEvent(event: unknown, token: string): Promise<SendResult> {
  try {
    await postVerifEvent(event as VerifEvent, token)
    return 'sent'
  } catch (error) {
    if (error instanceof ApiError && (error.status === 400 || error.status === 404)) return 'drop'
    return 'retry'
  }
}

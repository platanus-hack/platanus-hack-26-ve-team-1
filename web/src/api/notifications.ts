import { request } from './client'
import { mockSaveNotifications } from './mock'
import type { NotificationSettings } from './types'

const useMock = import.meta.env.VITE_USE_MOCK === 'true'

// Rutas relativas a la base "/api" que añade client.ts.
export const NOTIFICATIONS_ENDPOINTS = {
  // Contrato acordado: guarda el WhatsApp (User.telefono) y el interruptor de avisos.
  settings: '/me/notifications',
} as const

export function saveNotifications(settings: NotificationSettings): Promise<NotificationSettings> {
  if (useMock) return mockSaveNotifications(settings)
  return request<NotificationSettings>(NOTIFICATIONS_ENDPOINTS.settings, {
    method: 'PUT',
    body: settings,
  })
}

import { request } from './client'
import { mockGetMe, mockLogin, mockLogout, mockRegister } from './mock'
import type { LoginInput, RegisterInput, User } from './types'

const useMock = import.meta.env.VITE_USE_MOCK === 'true'

// Rutas relativas a la base "/api" que añade client.ts.
export const AUTH_ENDPOINTS = {
  // Contrato acordado.
  register: '/auth/register',
  // Contrato acordado.
  login: '/auth/login',
  // Contrato: pendiente de confirmar con P1.
  logout: '/auth/logout',
  // Contrato: pendiente de confirmar con P1.
  me: '/me',
} as const

export function register(input: RegisterInput): Promise<User> {
  if (useMock) return mockRegister(input)
  return request<User>(AUTH_ENDPOINTS.register, { method: 'POST', body: input })
}

export function login(input: LoginInput): Promise<User> {
  if (useMock) return mockLogin(input)
  return request<User>(AUTH_ENDPOINTS.login, {
    method: 'POST',
    body: input,
    signalUnauthorized: false,
  })
}

export function logout(): Promise<void> {
  if (useMock) return mockLogout()
  return request<void>(AUTH_ENDPOINTS.logout, { method: 'POST' })
}

export function getMe(): Promise<User> {
  if (useMock) return mockGetMe()
  return request<User>(AUTH_ENDPOINTS.me)
}

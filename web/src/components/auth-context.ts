import { createContext } from 'react'
import type { LoginInput, RegisterInput, User } from '../api/types'

export type AuthContextValue = {
  user: User | null
  /** true mientras se restaura la sesión al cargar la app. */
  loading: boolean
  login: (input: LoginInput) => Promise<void>
  register: (input: RegisterInput) => Promise<void>
  logout: () => Promise<void>
  /** Actualiza campos del usuario ya autenticado sin volver a consultar la sesión. */
  updateUser: (changes: Partial<User>) => void
}

export const AuthContext = createContext<AuthContextValue | null>(null)

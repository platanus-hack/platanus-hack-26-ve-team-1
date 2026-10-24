import { useSearchParams } from 'react-router'
import { isRole, type Role } from '../lib/roles'

const STORAGE_KEY = 'sisa:rol-temporal'

// TODO: Se reemplaza por el rol del usuario autenticado en el prompt 1.5.
// Solo en desarrollo: ?rol=emisor se guarda en sessionStorage y persiste al navegar.
export function useCurrentRole(): Role {
  const [params] = useSearchParams()
  if (!import.meta.env.DEV) return 'ciudadano'

  const fromUrl = params.get('rol')
  if (isRole(fromUrl)) {
    sessionStorage.setItem(STORAGE_KEY, fromUrl)
    return fromUrl
  }

  const stored = sessionStorage.getItem(STORAGE_KEY)
  return isRole(stored) ? stored : 'ciudadano'
}

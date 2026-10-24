import { useEffect, useRef, useState } from 'react'
import { createPresentation } from '../lib/slj/present.ts'
import { generateDeviceKey } from '../lib/slj/p256.ts'
import { ROTATE_SECONDS } from '../lib/slj/tables.ts'
import { getDeviceKeyPair } from '../lib/slj/store.ts'
import type { WalletCredential } from '../lib/slj/wallet.ts'

const ROTATE_MS = ROTATE_SECONDS * 1000
const TICK_MS = 250

/**
 * Cadena `SLJ1:` de la credencial elegida, regenerada cada 20 s con la hora actual (una captura
 * de pantalla deja de servir en menos de 30 s). `remaining` va de 1 a 0 para la barra de tiempo.
 */
export function useRotatingQr(cred: WalletCredential | null) {
  // El estado lleva la credencial a la que pertenece: al cambiar de credencial el QR anterior deja de mostrarse
  // sin tener que reiniciarlo con un setState dentro del efecto.
  const [state, setState] = useState<{ cred: WalletCredential; qr: string | null; error: boolean } | null>(null)
  const [remaining, setRemaining] = useState(1)
  const generatedAt = useRef(0)
  const generating = useRef(false)

  useEffect(() => {
    generatedAt.current = 0
    if (!cred) return

    let cancelled = false
    const generate = async () => {
      if (generating.current) return
      generating.current = true
      try {
        const pair = await getDeviceKeyPair(generateDeviceKey)
        const now = Date.now()
        const next = await createPresentation(cred.blob, pair.privateKey, Math.floor(now / 1000))
        if (cancelled) return
        generatedAt.current = now
        setState({ cred, qr: next, error: false })
      } catch {
        if (!cancelled) setState({ cred, qr: null, error: true })
      } finally {
        generating.current = false
      }
    }

    void generate()
    const timer = setInterval(() => {
      const age = Date.now() - generatedAt.current
      if (generatedAt.current === 0 || age >= ROTATE_MS) void generate()
      setRemaining(generatedAt.current === 0 ? 1 : Math.max(0, 1 - age / ROTATE_MS))
    }, TICK_MS)
    return () => {
      cancelled = true
      clearInterval(timer)
    }
  }, [cred])

  const current = cred && state?.cred === cred ? state : null
  return { qr: current?.qr ?? null, remaining, error: current?.error ?? false }
}

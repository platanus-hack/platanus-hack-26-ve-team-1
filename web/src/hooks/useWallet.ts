import { useCallback, useEffect, useState } from 'react'
import { confirmLink, getWalletCredentials } from '../api/wallet'
import { exportCompressed, generateDeviceKey } from '../lib/slj/p256.ts'
import { toHex } from '../lib/slj/bytes.ts'
import { getDeviceKeyPair, kvGet, kvSet, requestPersist } from '../lib/slj/store.ts'
import { credentialsFromResponse, toWalletCredential, type WalletCredential } from '../lib/slj/wallet.ts'
import { useOnline } from './useOnline'

const BLOBS_KEY = 'wallet.blobs'

async function loadStored(): Promise<WalletCredential[]> {
  const blobs = (await kvGet<Uint8Array[]>(BLOBS_KEY)) ?? []
  return blobs.flatMap((blob) => toWalletCredential(blob) ?? [])
}

async function save(creds: WalletCredential[]): Promise<void> {
  await kvSet(
    BLOBS_KEY,
    creds.map((c) => c.blob),
  )
}

/** Clave pública comprimida (hex de 66 caracteres) de la llave no extraíble de este celular. */
async function devicePublicKeyHex(): Promise<string> {
  const pair = await getDeviceKeyPair(generateDeviceKey)
  return toHex(await exportCompressed(pair.publicKey))
}

/**
 * Credenciales de este celular. Todo lo necesario para presentar el QR está guardado en el dispositivo
 * (IndexedDB), así que la billetera sigue mostrando el QR sin red. La clave privada nunca sale de WebCrypto.
 */
export function useWallet() {
  const online = useOnline()
  const [creds, setCreds] = useState<WalletCredential[]>([])
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)

  const replace = useCallback(async (next: WalletCredential[]) => {
    await save(next)
    setCreds(next)
  }, [])

  const refresh = useCallback(async () => {
    // Sin llave no hay nada vinculado que renovar.
    if (!navigator.onLine || !(await kvGet('devkey'))) return
    const list = credentialsFromResponse(await getWalletCredentials(await devicePublicKeyHex()))
    await replace(list)
  }, [replace])

  useEffect(() => {
    let cancelled = false
    void (async () => {
      const stored = await loadStored().catch(() => [])
      if (cancelled) return
      setCreds(stored)
      setLoading(false)
      // Renueva en segundo plano; si falla se conservan las guardadas.
      await refresh().catch(() => undefined)
    })()
    return () => {
      cancelled = true
    }
  }, [refresh])

  const link = useCallback(
    async (code: string) => {
      setBusy(true)
      try {
        await requestPersist()
        const list = credentialsFromResponse(await confirmLink(code, await devicePublicKeyHex()))
        if (list.length === 0) throw new Error('La cédula no tiene credenciales para la billetera.')
        await replace(list)
      } finally {
        setBusy(false)
      }
    },
    [replace],
  )

  const renew = useCallback(async () => {
    setBusy(true)
    try {
      await refresh()
    } finally {
      setBusy(false)
    }
  }, [refresh])

  return { creds, loading, busy, online, link, renew }
}

import { useCallback, useEffect, useRef, useState } from 'react'
import { getIssuers, getRevocations, sendQueuedEvent } from '../api/wallet'
import { revocationsFromHex, parseRevocations } from '../lib/slj/codec.ts'
import { toVerifEvent } from '../lib/slj/events.ts'
import { flushOutbox } from '../lib/slj/outbox.ts'
import { importIssuersJwk } from '../lib/slj/p256.ts'
import {
  kvDel,
  kvGet,
  kvSet,
  loadRevocations,
  pushOutbox,
  saveRevocations,
  takeOutbox,
} from '../lib/slj/store.ts'
import { verifyPresentation, type VerifyResult } from '../lib/slj/verify.ts'
import { useOnline } from './useOnline'

const SYNC_EVERY_MS = 30_000

/**
 * Verificador sin red. Los emisores, las revocaciones y el outbox viven en IndexedDB: tras la primera
 * sincronización todo el flujo de verificar funciona en modo avión, y los eventos se envían al volver la red.
 */
export function useVerifier() {
  const online = useOnline()
  const [issuers, setIssuers] = useState<Map<number, CryptoKey>>(new Map())
  const [revoked, setRevoked] = useState<Set<bigint>>(new Set())
  const [lastSync, setLastSync] = useState<number | null>(null)
  const [pending, setPending] = useState(0)
  const [token, setTokenState] = useState<string | null>(null)
  const [ready, setReady] = useState(false)
  const seen = useRef(new Map<string, number>())
  const tokenRef = useRef<string | null>(null)
  const syncing = useRef(false)

  const loadLocal = useCallback(async () => {
    const jwks = await kvGet<Record<string, JsonWebKey>>('issuers')
    if (jwks) setIssuers(await importIssuersJwk(jwks))
    const blob = await loadRevocations()
    if (blob) setRevoked(parseRevocations(blob))
    setLastSync((await kvGet<number>('lastSync')) ?? null)
    setPending((await takeOutbox()).length)
  }, [])

  const flush = useCallback(async () => {
    const current = tokenRef.current
    if (!current || !navigator.onLine) return
    await flushOutbox((event) => sendQueuedEvent(event, current))
    setPending((await takeOutbox()).length)
  }, [])

  const sync = useCallback(async () => {
    if (syncing.current || !navigator.onLine) return
    syncing.current = true
    try {
      const jwks = await getIssuers()
      const hs = await getRevocations()
      // Una respuesta vacía o rota no debe borrar lo que ya se tenía para verificar sin red.
      if (Object.keys(jwks).length > 0) await kvSet('issuers', jwks)
      await saveRevocations(revocationsFromHex(hs))
      await kvSet('lastSync', Date.now())
      await loadLocal()
    } catch {
      // Sin conexión útil: se sigue con los datos guardados y se reintenta luego.
    }
    try {
      await flush()
    } finally {
      syncing.current = false
    }
  }, [flush, loadLocal])

  useEffect(() => {
    let cancelled = false
    void (async () => {
      const saved = (await kvGet<string>('terminalToken')) ?? null
      tokenRef.current = saved
      if (cancelled) return
      setTokenState(saved)
      await loadLocal().catch(() => undefined)
      if (cancelled) return
      setReady(true)
      await sync()
    })()
    return () => {
      cancelled = true
    }
  }, [loadLocal, sync])

  // Al volver la red se sincroniza y se envían los eventos pendientes.
  useEffect(() => {
    const onOnline = () => void sync()
    window.addEventListener('online', onOnline)
    return () => window.removeEventListener('online', onOnline)
  }, [sync])

  useEffect(() => {
    const timer = setInterval(() => {
      if (document.visibilityState === 'visible') void sync()
    }, SYNC_EVERY_MS)
    return () => clearInterval(timer)
  }, [sync])

  const verify = useCallback(
    async (text: string, accept?: Set<number>): Promise<VerifyResult> => {
      const nowSec = Math.floor(Date.now() / 1000)
      const result = await verifyPresentation(text.trim(), { issuers, revoked, now: nowSec, accept, seen: seen.current })
      // Sin token del terminal no hay a dónde enviar el evento: no se encola.
      const event = tokenRef.current ? toVerifEvent(result, nowSec) : null
      if (event) {
        await pushOutbox(event)
        setPending((await takeOutbox()).length)
        void flush()
      }
      return result
    },
    [issuers, revoked, flush],
  )

  const setToken = useCallback(
    async (next: string | null) => {
      const clean = next?.trim() || null
      tokenRef.current = clean
      setTokenState(clean)
      if (clean) {
        await kvSet('terminalToken', clean)
        void flush()
      } else {
        await kvDel('terminalToken')
      }
    },
    [flush],
  )

  return { online, ready, issuers: issuers.size, revoked: revoked.size, lastSync, pending, token, setToken, sync, verify }
}

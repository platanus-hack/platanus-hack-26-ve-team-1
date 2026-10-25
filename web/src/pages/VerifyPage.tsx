import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { PageTitle } from '../components/PageTitle'
import { Button, Card, Select } from '../components/ui'
import { ResultPanel } from '../components/verify/ResultPanel'
import { Scanner } from '../components/verify/Scanner'
import { SyncStatus } from '../components/verify/SyncStatus'
import { TokenForm } from '../components/verify/TokenForm'
import { useVerifier } from '../hooks/useVerifier'
import { setupPwa } from '../lib/pwa'
import type { VerifyResult } from '../lib/slj/verify.ts'

const RESULT_VISIBLE_MS = 5000
/** El mismo QR frente a la cámara no se vuelve a procesar durante este tiempo. */
const SAME_QR_COOLDOWN_MS = 5000

// Qué dato pide este local. Los valores son los códigos de atributo del contrato (A3 del emisor 2 = carnet).
const ACCEPT_OPTIONS = [
  { value: '', label: 'Cualquier dato' },
  { value: '1', label: 'Mayor de 18' },
  { value: '2', label: 'Mayor de 21' },
  { value: '6', label: 'Donante y licencia' },
  { value: '3', label: 'Carnet universitario' },
] as const

export default function VerifyPage() {
  const { online, issuers, revoked, lastSync, pending, token, setToken, sync, verify } = useVerifier()
  const [scanning, setScanning] = useState(false)
  const [result, setResult] = useState<VerifyResult | null>(null)
  const [accept, setAccept] = useState('')
  const last = useRef({ text: '', at: 0 })
  const hideTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

  useEffect(() => setupPwa('verificar'), [])
  useEffect(() => () => clearTimeout(hideTimer.current), [])
  // Sin lector nativo de QR se usará jsQR: se descarga ya (con red) para que luego funcione sin red.
  useEffect(() => {
    if (!('BarcodeDetector' in window)) void import('jsqr')
  }, [])

  const acceptSet = useMemo(() => (accept ? new Set([Number(accept)]) : undefined), [accept])

  const handleCode = useCallback(
    async (text: string) => {
      const now = Date.now()
      if (text === last.current.text && now - last.current.at < SAME_QR_COOLDOWN_MS) return
      last.current = { text, at: now }
      setResult(await verify(text, acceptSet))
      clearTimeout(hideTimer.current)
      hideTimer.current = setTimeout(() => setResult(null), RESULT_VISIBLE_MS)
    },
    [verify, acceptSet],
  )

  const noData = lastSync === null

  return (
    <div className="flex flex-col gap-6">
      <PageTitle>Verificar</PageTitle>

      {result && <ResultPanel result={result} />}

      <Card className="flex flex-col gap-4">
        <Select label="Este local pide" value={accept} onChange={(event) => setAccept(event.target.value)}>
          {ACCEPT_OPTIONS.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </Select>

        {scanning ? (
          <>
            <Scanner onCode={(text) => void handleCode(text)} />
            <Button variant="secondary" onClick={() => setScanning(false)}>
              Detener cámara
            </Button>
          </>
        ) : (
          <Button disabled={noData} onClick={() => setScanning(true)}>
            Abrir cámara
          </Button>
        )}
      </Card>

      <Card>
        <SyncStatus
          online={online}
          issuers={issuers}
          revoked={revoked}
          lastSync={lastSync}
          pending={pending}
          onSync={() => void sync()}
        />
      </Card>

      <TokenForm token={token} onSave={setToken} />

      {import.meta.env.DEV && <DevPaste onCode={handleCode} />}
    </div>
  )
}

/** Solo en desarrollo: probar sin cámara pegando la cadena SLJ1. No existe en producción. */
function DevPaste({ onCode }: { onCode: (text: string) => Promise<void> }) {
  const [text, setText] = useState('')
  return (
    <details className="rounded-2xl border border-dashed border-neutral-400 p-4">
      <summary className="cursor-pointer text-base text-neutral-600">Desarrollo: pegar cadena SLJ1</summary>
      <div className="mt-3 flex flex-col gap-3">
        <textarea
          aria-label="Cadena SLJ1"
          className="min-h-24 rounded-xl border-2 border-neutral-400 p-3 font-mono text-sm"
          value={text}
          onChange={(event) => setText(event.target.value)}
        />
        <Button variant="secondary" onClick={() => void onCode(text)}>
          Verificar
        </Button>
      </div>
    </details>
  )
}

import { useEffect, useMemo, useState } from 'react'
import { ApiError } from '../api/client'
import { CredentialPicker } from '../components/wallet/CredentialPicker'
import { LinkForm } from '../components/wallet/LinkForm'
import { QrPanel } from '../components/wallet/QrPanel'
import { PageTitle } from '../components/PageTitle'
import { Button, Card, EmptyState, Spinner } from '../components/ui'
import { useRotatingQr } from '../hooks/useRotatingQr'
import { useToast } from '../hooks/useToast'
import { useWallet } from '../hooks/useWallet'
import { setupPwa } from '../lib/pwa'
import { usable } from '../lib/slj/wallet.ts'

function messageOf(error: unknown): string {
  if (error instanceof ApiError || error instanceof Error) return error.message
  return 'Ocurrió un error inesperado. Inténtalo de nuevo.'
}

export default function WalletPage() {
  const { creds, loading, busy, online, link, renew } = useWallet()
  const { toast } = useToast()
  const [selected, setSelected] = useState(0)
  const [relinking, setRelinking] = useState(false)
  const [nowSec] = useState(() => Math.floor(Date.now() / 1000))

  useEffect(() => setupPwa('billetera'), [])

  const valid = useMemo(() => usable(creds, nowSec), [creds, nowSec])
  const current = valid[Math.min(selected, valid.length - 1)] ?? null
  const { qr, remaining, error } = useRotatingQr(current)

  async function handleLink(code: string): Promise<string | null> {
    try {
      await link(code)
      setSelected(0)
      setRelinking(false)
      toast({ title: 'Celular vinculado', description: 'Ya puedes mostrar tus credenciales.', variant: 'ok' })
      return null
    } catch (err) {
      return messageOf(err)
    }
  }

  async function handleRenew() {
    try {
      await renew()
      toast({ title: 'Credenciales actualizadas', variant: 'ok' })
    } catch (err) {
      toast({ title: 'No se pudo actualizar', description: messageOf(err), variant: 'error' })
    }
  }

  if (loading) {
    return (
      <div className="flex flex-col gap-6">
        <PageTitle>Billetera</PageTitle>
        <Spinner />
      </div>
    )
  }

  if (creds.length === 0 || relinking) {
    return (
      <div className="flex flex-col gap-6">
        <PageTitle>Billetera</PageTitle>
        <Card className="flex flex-col gap-4">
          <h2 className="text-2xl font-bold text-neutral-900">Vincula tu cédula a este celular</h2>
          <p className="text-lg text-neutral-700">
            Se hace una sola vez. Después podrás mostrar un QR que el comercio verifica sin internet.
          </p>
          <LinkForm busy={busy} onLink={handleLink} />
          {relinking && (
            <Button variant="secondary" onClick={() => setRelinking(false)}>
              Cancelar
            </Button>
          )}
        </Card>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-6">
      <PageTitle>Billetera</PageTitle>

      {current ? (
        <>
          <CredentialPicker creds={valid} selected={Math.min(selected, valid.length - 1)} onSelect={setSelected} />
          <QrPanel cred={current} qr={qr} remaining={remaining} error={error} online={online} />
        </>
      ) : (
        <EmptyState
          title="No tienes credenciales vigentes"
          description="Tu cédula puede estar bloqueada o vencida. Actualiza para pedir las vigentes."
        />
      )}

      <div className="flex flex-wrap gap-3">
        <Button variant="secondary" loading={busy} disabled={!online} onClick={() => void handleRenew()}>
          Actualizar credenciales
        </Button>
        <Button variant="secondary" onClick={() => setRelinking(true)}>
          Vincular otra cédula
        </Button>
      </div>
      {!online && (
        <p className="text-base text-neutral-600">Sin red no se pueden actualizar, pero el QR sigue funcionando.</p>
      )}
    </div>
  )
}

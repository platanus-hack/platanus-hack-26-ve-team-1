import { useToast } from '../../hooks/useToast'
import { Button, Modal } from '../ui'

type TokenModalProps = {
  terminalName: string
  /** Token en claro: solo vive en el estado de quien monta este modal y se descarta al cerrar. */
  token: string
  onClose: () => void
}

// El token se muestra una única vez. Este componente no lo guarda en ningún lado.
export function TokenModal({ terminalName, token, onClose }: TokenModalProps) {
  const { toast } = useToast()

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(token)
      toast({ title: 'Token copiado', description: 'Ya está en tu portapapeles.', variant: 'ok' })
    } catch {
      toast({
        title: 'No se pudo copiar el token',
        description: 'Selecciónalo con el mouse y cópialo a mano.',
        variant: 'error',
      })
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      title="Token del terminal"
      footer={
        <>
          <Button variant="secondary" onClick={handleCopy}>
            Copiar token
          </Button>
          <Button onClick={onClose} data-autofocus>
            Ya lo guardé
          </Button>
        </>
      }
    >
      <p>
        Este es el token de <strong>{terminalName}</strong>. Configúralo en el terminal.
      </p>
      <p
        role="note"
        className="mt-3 rounded-xl border-2 border-amber-600 bg-amber-50 p-4 text-base font-semibold text-amber-900"
      >
        Se muestra una sola vez. Cuando cierres esta ventana no podrás verlo de nuevo.
      </p>
      <code className="mt-3 block rounded-xl bg-neutral-100 p-4 font-mono text-base break-all text-neutral-900 select-all">
        {token}
      </code>
    </Modal>
  )
}

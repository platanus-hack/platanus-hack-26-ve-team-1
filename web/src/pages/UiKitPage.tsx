import { useState, type ReactNode } from 'react'
import {
  Badge,
  Button,
  Card,
  EmptyState,
  Input,
  Modal,
  Spinner,
} from '../components/ui'
import { useToast } from '../hooks/useToast'

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Card aria-label={title} className="flex flex-col gap-4">
      <h2 className="text-2xl font-bold text-primary-900">{title}</h2>
      {children}
    </Card>
  )
}

export default function UiKitPage() {
  const [modalOpen, setModalOpen] = useState(false)
  const [name, setName] = useState('')
  const { toast } = useToast()

  return (
    <main className="mx-auto flex max-w-4xl flex-col gap-6 p-6">
      <h1 className="text-4xl font-bold text-primary-900">UI Kit (temporal)</h1>

      <Section title="Button">
        <div className="flex flex-wrap gap-4">
          <Button>Primario</Button>
          <Button variant="secondary">Secundario</Button>
          <Button variant="danger">Peligro</Button>
          <Button loading>Cargando</Button>
          <Button variant="secondary" loading>
            Cargando
          </Button>
          <Button disabled>Deshabilitado</Button>
        </div>
      </Section>

      <Section title="Badge">
        <div className="flex flex-wrap gap-4">
          <Badge variant="ok">Mayor de 18</Badge>
          <Badge variant="error">Denegado</Badge>
          <Badge variant="warning">Pendiente</Badge>
          <Badge variant="neutral">Sin verificar</Badge>
        </div>
      </Section>

      <Section title="Input">
        <Input
          label="Nombre"
          hint="Como aparece en tu cédula."
          value={name}
          onChange={(event) => setName(event.target.value)}
        />
        <Input
          label="Correo"
          type="email"
          defaultValue="correo-invalido"
          error="Ingresa un correo válido."
        />
        <Input label="Deshabilitado" disabled defaultValue="No editable" />
      </Section>

      <Section title="Spinner">
        <Spinner label="Cargando datos" className="text-primary-700" />
      </Section>

      <Section title="Modal">
        <div>
          <Button onClick={() => setModalOpen(true)}>Abrir modal</Button>
        </div>
        <Modal
          open={modalOpen}
          onClose={() => setModalOpen(false)}
          title="¿Revocar credencial?"
          footer={
            <>
              <Button variant="secondary" onClick={() => setModalOpen(false)}>
                Cancelar
              </Button>
              <Button variant="danger" onClick={() => setModalOpen(false)}>
                Revocar
              </Button>
            </>
          }
        >
          Esta acción no se puede deshacer.
        </Modal>
      </Section>

      <Section title="Toast">
        <div className="flex flex-wrap gap-4">
          <Button onClick={() => toast({ title: 'Guardado', variant: 'ok' })}>
            Éxito
          </Button>
          <Button
            variant="danger"
            onClick={() =>
              toast({
                title: 'Error',
                description: 'No se pudo completar la operación.',
                variant: 'error',
              })
            }
          >
            Error
          </Button>
          <Button
            variant="secondary"
            onClick={() => toast({ title: 'Información' })}
          >
            Neutro
          </Button>
        </div>
      </Section>

      <Section title="EmptyState">
        <EmptyState
          title="Aún no hay registros"
          description="Cuando se emita una credencial aparecerá aquí."
          action={<Button variant="secondary">Actualizar</Button>}
        />
      </Section>
    </main>
  )
}

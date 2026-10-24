import { useEffect } from 'react'
import { ApiError } from '../api/client'
import type { CardInfo } from '../api/types'
import { AttributeIcon } from '../components/AttributeIcon'
import { EventHistory } from '../components/EventHistory'
import { OptionalDataForm } from '../components/OptionalDataForm'
import { PageTitle } from '../components/PageTitle'
import { Badge, Button, Card, EmptyState, Spinner } from '../components/ui'
import { useAuth } from '../hooks/useAuth'
import { useCardInfo } from '../hooks/useCardInfo'
import { useToast } from '../hooks/useToast'
import { ATTRIBUTE_INFO } from '../lib/attributes'
import { getErrorMessage } from '../lib/errors'
import { formatDate } from '../lib/format'

function isNotFound(error: unknown): boolean {
  return error instanceof ApiError && error.status === 404
}

function CardStatus({ card }: { card: CardInfo }) {
  const active = card.estado === 'activa'
  return (
    <Card className="flex flex-wrap items-center gap-4" aria-label="Estado de la cédula">
      <Badge variant={active ? 'ok' : 'error'} className="px-5 py-2 text-xl">
        {active ? 'Cédula activa' : 'Cédula revocada'}
      </Badge>
      <p className="text-lg text-neutral-700">
        Emitida el {formatDate(card.emitidaEn)}
      </p>
    </Card>
  )
}

function AttributeGrid({ card }: { card: CardInfo }) {
  if (card.estado === 'revocada' || card.atributos.length === 0) {
    return (
      <EmptyState
        title="Tu cédula está revocada"
        description="Ningún lector puede consultar tus datos mientras esté revocada. Si no fuiste tú, contacta al emisor para reactivarla."
      />
    )
  }

  return (
    <ul className="grid gap-4 md:grid-cols-2">
      {card.atributos.map((attribute) => {
        const info = ATTRIBUTE_INFO[attribute]
        return (
          <li key={attribute}>
            <Card
              as="article"
              aria-labelledby={`atributo-${attribute}`}
              className="flex h-full gap-4"
            >
              <span className="flex size-14 shrink-0 items-center justify-center rounded-xl bg-primary-100 text-primary-800">
                <AttributeIcon attribute={attribute} className="size-8" />
              </span>
              <div className="flex flex-col gap-2">
                <h3
                  id={`atributo-${attribute}`}
                  className="text-xl font-bold text-neutral-900"
                >
                  {info.nombre}
                </h3>
                <p className="text-lg text-neutral-700">{info.descripcion}</p>
                <p className="text-base font-semibold text-neutral-600">
                  Puede leerlo:{' '}
                  <Badge variant="neutral" className="ml-1">
                    {info.lector}
                  </Badge>
                </p>
              </div>
            </Card>
          </li>
        )
      })}
    </ul>
  )
}

export default function HomePage() {
  const { user } = useAuth()
  const { toast } = useToast()
  const { state, reload } = useCardInfo()

  useEffect(() => {
    if (state.status === 'error' && !isNotFound(state.error)) {
      toast({
        title: 'No se pudo cargar tu cédula',
        description: getErrorMessage(state.error),
        variant: 'error',
      })
    }
  }, [state, toast])

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col gap-1">
        <PageTitle>Mi cédula</PageTitle>
        {user && <p className="text-xl text-neutral-700">Hola, {user.nombre}.</p>}
      </header>

      <Card className="border-primary-200 bg-primary-50">
        <h2 className="text-2xl font-bold text-primary-900">
          Sabe más, muestra menos
        </h2>
        <p className="mt-2 text-lg text-primary-900">
          sIsA guarda tus datos, pero cada lector recibe solo la respuesta que
          necesita. Un comercio ve «mayor de 18: sí» y nada más.
        </p>
      </Card>

      {state.status === 'loading' && (
        <div className="flex items-center gap-3 py-8 text-xl text-primary-800">
          <Spinner label="Cargando tu cédula" className="[&>svg]:size-8" />
          <span aria-hidden="true">Cargando tu cédula…</span>
        </div>
      )}

      {state.status === 'error' &&
        (isNotFound(state.error) ? (
          <EmptyState
            title="Aún no tienes una cédula"
            description="Cuando el Registro emita tu cédula sIsA, aparecerá aquí."
          />
        ) : (
          <EmptyState
            title="No pudimos cargar tu cédula"
            description={getErrorMessage(state.error)}
            action={<Button onClick={reload}>Reintentar</Button>}
          />
        ))}

      {state.status === 'ready' && (
        <>
          <CardStatus card={state.card} />
          <section aria-labelledby="datos-chip" className="flex flex-col gap-4">
            <h2 id="datos-chip" className="text-2xl font-bold text-primary-900">
              Qué guarda tu chip y quién puede leerlo
            </h2>
            <AttributeGrid card={state.card} />
          </section>
        </>
      )}

      <OptionalDataForm />

      <EventHistory />
    </div>
  )
}

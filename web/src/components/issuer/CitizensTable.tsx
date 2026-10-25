import { useId } from 'react'
import type { Citizen, CitizenCardStatus } from '../../api/types'
import type { BadgeVariant } from '../ui'
import { formatDate } from '../../lib/format'
import { Badge, Button, Card } from '../ui'

const STATUS_INFO: Record<CitizenCardStatus, { label: string; variant: BadgeVariant }> = {
  activa: { label: 'Activa', variant: 'ok' },
  revocada: { label: 'Revocada', variant: 'error' },
  sin_emitir: { label: 'Sin emitir', variant: 'neutral' },
}

type CitizensTableProps = {
  citizens: Citizen[]
  onEmit: (citizen: Citizen) => void
  onRevoke: (citizen: Citizen) => void
  onCompleteProfile: (citizen: Citizen) => void
  /** Deshabilita todas las acciones mientras hay una emisión o una revocación en curso. */
  disabled?: boolean
}

function RowActions({
  citizen,
  onEmit,
  onRevoke,
  onCompleteProfile,
  disabled,
}: {
  citizen: Citizen
} & Pick<CitizensTableProps, 'onEmit' | 'onRevoke' | 'onCompleteProfile' | 'disabled'>) {
  const missingProfileId = useId()

  // Una cédula activa solo se puede revocar; sin cédula o revocada, se puede emitir una nueva.
  if (citizen.estadoCedula === 'activa') {
    return (
      <Button
        variant="danger"
        onClick={() => onRevoke(citizen)}
        disabled={disabled}
        aria-label={`Revocar la cédula de ${citizen.nombre}`}
      >
        Revocar
      </Button>
    )
  }

  // Sin fecha de nacimiento y cédula no se puede emitir: primero se completa el perfil.
  if (!citizen.perfilCompleto) {
    return (
      <div className="flex flex-col items-start gap-2">
        <div className="flex flex-wrap gap-2">
          <Button
            onClick={() => onCompleteProfile(citizen)}
            disabled={disabled}
            aria-label={`Completar el perfil de ${citizen.nombre}`}
          >
            Completar perfil
          </Button>
          <Button
            variant="secondary"
            disabled
            aria-describedby={missingProfileId}
            aria-label={`Emitir cédula a ${citizen.nombre}`}
          >
            Emitir cédula
          </Button>
        </div>
        <p id={missingProfileId} className="text-base text-neutral-600">
          Falta fecha de nacimiento y cédula
        </p>
      </div>
    )
  }

  return (
    <Button
      onClick={() => onEmit(citizen)}
      disabled={disabled}
      aria-label={`Emitir cédula a ${citizen.nombre}`}
    >
      Emitir cédula
    </Button>
  )
}

function StatusBadge({ status }: { status: CitizenCardStatus }) {
  const info = STATUS_INFO[status]
  return <Badge variant={info.variant}>{info.label}</Badge>
}

function issuedOn(citizen: Citizen): string {
  return citizen.emitidaEn ? formatDate(citizen.emitidaEn) : '—'
}

/** Tabla en pantallas anchas; en móvil se degrada a una tarjeta por ciudadano. */
export function CitizensTable({
  citizens,
  onEmit,
  onRevoke,
  onCompleteProfile,
  disabled,
}: CitizensTableProps) {
  return (
    <>
      <div className="hidden overflow-hidden rounded-2xl border border-neutral-200 bg-white shadow-sm md:block">
        <table className="w-full text-left text-lg">
          <caption className="sr-only">Ciudadanos y estado de su cédula</caption>
          <thead className="bg-primary-50 text-base text-primary-900">
            <tr>
              <th scope="col" className="px-4 py-3 font-bold">Nombre</th>
              <th scope="col" className="px-4 py-3 font-bold">Correo</th>
              <th scope="col" className="px-4 py-3 font-bold">Estado de la cédula</th>
              <th scope="col" className="px-4 py-3 font-bold">Emisión</th>
              <th scope="col" className="px-4 py-3 font-bold">Acciones</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-neutral-200">
            {citizens.map((citizen) => (
              <tr key={citizen.id}>
                <th scope="row" className="px-4 py-4 font-semibold text-neutral-900">
                  {citizen.nombre}
                </th>
                <td className="px-4 py-4 break-all text-neutral-700">{citizen.email}</td>
                <td className="px-4 py-4">
                  <StatusBadge status={citizen.estadoCedula} />
                </td>
                <td className="px-4 py-4 text-neutral-700">{issuedOn(citizen)}</td>
                <td className="px-4 py-4">
                  <RowActions
                    citizen={citizen}
                    onEmit={onEmit}
                    onRevoke={onRevoke}
                    onCompleteProfile={onCompleteProfile}
                    disabled={disabled}
                  />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <ul className="flex flex-col gap-4 md:hidden">
        {citizens.map((citizen) => (
          <li key={citizen.id}>
            <Card as="article" aria-label={citizen.nombre} className="flex flex-col gap-3">
              <div className="flex flex-wrap items-start justify-between gap-2">
                <h3 className="text-xl font-bold text-neutral-900">{citizen.nombre}</h3>
                <StatusBadge status={citizen.estadoCedula} />
              </div>
              <p className="text-lg break-all text-neutral-700">{citizen.email}</p>
              <p className="text-base text-neutral-600">Emisión: {issuedOn(citizen)}</p>
              <div>
                <RowActions
                  citizen={citizen}
                  onEmit={onEmit}
                  onRevoke={onRevoke}
                  onCompleteProfile={onCompleteProfile}
                  disabled={disabled}
                />
              </div>
            </Card>
          </li>
        ))}
      </ul>
    </>
  )
}

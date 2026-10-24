import type { Terminal } from '../../api/types'
import { ATTRIBUTE_INFO } from '../../lib/attributes'
import { TERMINAL_ROLE_INFO } from '../../lib/terminals'
import { Badge, Button, Card } from '../ui'

type TerminalsListProps = {
  terminals: Terminal[]
  onEdit: (terminal: Terminal) => void
  onDelete: (terminal: Terminal) => void
  /** Deshabilita las acciones mientras hay un formulario o una confirmación abiertos. */
  disabled?: boolean
}

function dataLabel(terminal: Terminal): string {
  return terminal.atributos.map((attribute) => ATTRIBUTE_INFO[attribute].nombre).join(', ')
}

function RoleBadges({ terminal }: { terminal: Terminal }) {
  const info = TERMINAL_ROLE_INFO[terminal.rol]
  return (
    <span className="flex flex-wrap items-center gap-2">
      <Badge variant="neutral">{info.label}</Badge>
      {terminal.rol === 'emergencia' && <Badge variant="warning">Sin consentimiento</Badge>}
    </span>
  )
}

function RowActions({
  terminal,
  onEdit,
  onDelete,
  disabled,
}: { terminal: Terminal } & Pick<TerminalsListProps, 'onEdit' | 'onDelete' | 'disabled'>) {
  return (
    <div className="flex flex-wrap gap-2">
      <Button
        variant="secondary"
        onClick={() => onEdit(terminal)}
        disabled={disabled}
        aria-label={`Editar el terminal ${terminal.nombre}`}
      >
        Editar
      </Button>
      <Button
        variant="danger"
        onClick={() => onDelete(terminal)}
        disabled={disabled}
        aria-label={`Eliminar el terminal ${terminal.nombre}`}
      >
        Eliminar
      </Button>
    </div>
  )
}

/** Tabla en pantallas anchas; en móvil se degrada a una tarjeta por terminal. */
export function TerminalsList({ terminals, onEdit, onDelete, disabled }: TerminalsListProps) {
  return (
    <>
      <div className="hidden overflow-hidden rounded-2xl border border-neutral-200 bg-white shadow-sm md:block">
        <table className="w-full text-left text-lg">
          <caption className="sr-only">Terminales configurados</caption>
          <thead className="bg-primary-50 text-base text-primary-900">
            <tr>
              <th scope="col" className="px-4 py-3 font-bold">Nombre</th>
              <th scope="col" className="px-4 py-3 font-bold">Tipo</th>
              <th scope="col" className="px-4 py-3 font-bold">Datos que pide</th>
              <th scope="col" className="px-4 py-3 font-bold">Acciones</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-neutral-200">
            {terminals.map((terminal) => (
              <tr key={terminal.id}>
                <th scope="row" className="px-4 py-4 font-semibold text-neutral-900">
                  {terminal.nombre}
                </th>
                <td className="px-4 py-4">
                  <RoleBadges terminal={terminal} />
                </td>
                <td className="px-4 py-4 text-neutral-700">{dataLabel(terminal)}</td>
                <td className="px-4 py-4">
                  <RowActions
                    terminal={terminal}
                    onEdit={onEdit}
                    onDelete={onDelete}
                    disabled={disabled}
                  />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <ul className="flex flex-col gap-4 md:hidden">
        {terminals.map((terminal) => (
          <li key={terminal.id}>
            <Card as="article" aria-label={terminal.nombre} className="flex flex-col gap-3">
              <div className="flex flex-wrap items-start justify-between gap-2">
                <h3 className="text-xl font-bold text-neutral-900">{terminal.nombre}</h3>
                <RoleBadges terminal={terminal} />
              </div>
              <p className="text-lg text-neutral-700">Pide: {dataLabel(terminal)}</p>
              <RowActions
                terminal={terminal}
                onEdit={onEdit}
                onDelete={onDelete}
                disabled={disabled}
              />
            </Card>
          </li>
        ))}
      </ul>
    </>
  )
}

import type { Student, StudentCredentialStatus } from '../../api/types'
import type { BadgeVariant } from '../ui'
import { formatDate } from '../../lib/format'
import { Badge, Button, Card } from '../ui'

const STATUS_INFO: Record<StudentCredentialStatus, { label: string; variant: BadgeVariant }> = {
  sin_carnet: { label: 'Sin carnet', variant: 'neutral' },
  emitido: { label: 'Carnet emitido', variant: 'ok' },
}

type StudentsTableProps = {
  students: Student[]
  onIssue: (student: Student) => void
  /** Deshabilita las acciones mientras hay una emisión en curso. */
  disabled?: boolean
}

function StatusBadge({ status }: { status: StudentCredentialStatus }) {
  const info = STATUS_INFO[status]
  return <Badge variant={info.variant}>{info.label}</Badge>
}

function issuedOn(student: Student): string {
  return student.emitidoEn ? formatDate(student.emitidoEn) : '—'
}

function RowAction({
  student,
  onIssue,
  disabled,
}: { student: Student } & Pick<StudentsTableProps, 'onIssue' | 'disabled'>) {
  // Con carnet ya emitido no hay acción disponible.
  if (student.estadoCarnet === 'emitido') {
    return (
      <span className="text-neutral-600">
        <span aria-hidden="true">—</span>
        <span className="sr-only">Sin acciones disponibles</span>
      </span>
    )
  }
  return (
    <Button
      onClick={() => onIssue(student)}
      disabled={disabled}
      aria-label={`Emitir carnet universitario a ${student.nombre}`}
    >
      Emitir carnet universitario
    </Button>
  )
}

/** Tabla en pantallas anchas; en móvil se degrada a una tarjeta por estudiante. */
export function StudentsTable({ students, onIssue, disabled }: StudentsTableProps) {
  return (
    <>
      <div className="hidden overflow-hidden rounded-2xl border border-neutral-200 bg-white shadow-sm md:block">
        <table className="w-full text-left text-lg">
          <caption className="sr-only">Estudiantes y estado de su carnet universitario</caption>
          <thead className="bg-primary-50 text-base text-primary-900">
            <tr>
              <th scope="col" className="px-4 py-3 font-bold">Nombre</th>
              <th scope="col" className="px-4 py-3 font-bold">Correo</th>
              <th scope="col" className="px-4 py-3 font-bold">Estado del carnet</th>
              <th scope="col" className="px-4 py-3 font-bold">Emisión</th>
              <th scope="col" className="px-4 py-3 font-bold">Acciones</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-neutral-200">
            {students.map((student) => (
              <tr key={student.id}>
                <th scope="row" className="px-4 py-4 font-semibold text-neutral-900">
                  {student.nombre}
                </th>
                <td className="px-4 py-4 break-all text-neutral-700">{student.email}</td>
                <td className="px-4 py-4">
                  <StatusBadge status={student.estadoCarnet} />
                </td>
                <td className="px-4 py-4 text-neutral-700">{issuedOn(student)}</td>
                <td className="px-4 py-4">
                  <RowAction student={student} onIssue={onIssue} disabled={disabled} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <ul className="flex flex-col gap-4 md:hidden">
        {students.map((student) => (
          <li key={student.id}>
            <Card as="article" aria-label={student.nombre} className="flex flex-col gap-3">
              <div className="flex flex-wrap items-start justify-between gap-2">
                <h3 className="text-xl font-bold text-neutral-900">{student.nombre}</h3>
                <StatusBadge status={student.estadoCarnet} />
              </div>
              <p className="text-lg break-all text-neutral-700">{student.email}</p>
              <p className="text-base text-neutral-600">Emisión: {issuedOn(student)}</p>
              {student.estadoCarnet === 'sin_carnet' && (
                <div>
                  <RowAction student={student} onIssue={onIssue} disabled={disabled} />
                </div>
              )}
            </Card>
          </li>
        ))}
      </ul>
    </>
  )
}

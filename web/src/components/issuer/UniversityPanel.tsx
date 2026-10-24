import { useEffect, useState } from 'react'
import type { Student } from '../../api/types'
import { useStudents } from '../../hooks/useStudents'
import { useToast } from '../../hooks/useToast'
import { getErrorMessage } from '../../lib/errors'
import { Button, EmptyState, Spinner } from '../ui'
import { IssueCredentialModal } from './IssueCredentialModal'
import { StudentsTable } from './StudentsTable'

export function UniversityPanel() {
  const { toast } = useToast()
  const { state, reload, refresh } = useStudents()
  const [issueTarget, setIssueTarget] = useState<Student | null>(null)

  useEffect(() => {
    if (state.status === 'error') {
      toast({
        title: 'No se pudo cargar la lista de estudiantes',
        description: getErrorMessage(state.error),
        variant: 'error',
      })
    }
  }, [state, toast])

  return (
    <div className="flex flex-col gap-6">
      <p className="text-xl text-neutral-700">
        Emite el carnet universitario sIsA de cada estudiante.
      </p>

      {state.status === 'loading' && (
        <div className="flex items-center gap-3 py-8 text-xl text-primary-800">
          <Spinner label="Cargando estudiantes" className="[&>svg]:size-8" />
          <span aria-hidden="true">Cargando estudiantes…</span>
        </div>
      )}

      {state.status === 'error' && (
        <EmptyState
          title="No pudimos cargar los estudiantes"
          description={getErrorMessage(state.error)}
          action={<Button onClick={reload}>Reintentar</Button>}
        />
      )}

      {state.status === 'ready' && state.students.length === 0 && (
        <EmptyState
          title="Aún no hay estudiantes"
          description="Cuando la universidad registre estudiantes, aparecerán aquí para emitirles su carnet."
        />
      )}

      {state.status === 'ready' && state.students.length > 0 && (
        <StudentsTable
          students={state.students}
          onIssue={setIssueTarget}
          disabled={issueTarget !== null}
        />
      )}

      {issueTarget && (
        <IssueCredentialModal
          student={issueTarget}
          onClose={() => setIssueTarget(null)}
          onIssued={refresh}
        />
      )}
    </div>
  )
}

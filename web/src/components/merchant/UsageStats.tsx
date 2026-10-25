import { useEffect } from 'react'
import type { Terminal } from '../../api/types'
import { useTerminalStats } from '../../hooks/useTerminalStats'
import { useToast } from '../../hooks/useToast'
import { getErrorMessage } from '../../lib/errors'
import { Button, EmptyState, Spinner } from '../ui'

type UsageStatsProps = {
  terminals: Terminal[]
  /** Cambia cuando la lista de terminales cambia, para volver a pedir los totales. */
  version: number
}

/** Solo totales por terminal (hoy, aprobadas, rechazadas): nunca handles, UID ni datos personales. */
export function UsageStats({ terminals, version }: UsageStatsProps) {
  const { toast } = useToast()
  const { state, reload } = useTerminalStats(version)

  useEffect(() => {
    if (state.status === 'error') {
      toast({
        title: 'No se pudo cargar el uso de los terminales',
        description: getErrorMessage(state.error),
        variant: 'error',
      })
    }
  }, [state, toast])

  return (
    <section aria-labelledby="uso-terminales" className="flex flex-col gap-4">
      <h2 id="uso-terminales" className="text-2xl font-bold text-primary-900">
        Uso de los terminales
      </h2>

      {state.status === 'loading' && (
        <div className="flex items-center gap-3 py-8 text-xl text-primary-800">
          <Spinner label="Cargando el uso" className="[&>svg]:size-8" />
          <span aria-hidden="true">Cargando el uso…</span>
        </div>
      )}

      {state.status === 'error' && (
        <EmptyState
          title="No pudimos cargar el uso"
          description={getErrorMessage(state.error)}
          action={<Button onClick={reload}>Reintentar</Button>}
        />
      )}

      {state.status === 'ready' && terminals.length === 0 && (
        <EmptyState
          title="Aún no hay uso que mostrar"
          description="Cuando crees un terminal, verás aquí cuántas lecturas hace."
        />
      )}

      {state.status === 'ready' && terminals.length > 0 && (
        <div className="overflow-x-auto rounded-2xl border border-neutral-200 bg-white shadow-sm">
          <table className="w-full text-left text-lg">
            <caption className="sr-only">Totales de uso por terminal</caption>
            <thead className="bg-primary-50 text-base text-primary-900">
              <tr>
                <th scope="col" className="px-4 py-3 font-bold">Terminal</th>
                <th scope="col" className="px-4 py-3 text-right font-bold">Hoy</th>
                <th scope="col" className="px-4 py-3 text-right font-bold">Aprobadas</th>
                <th scope="col" className="px-4 py-3 text-right font-bold">Rechazadas</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-200">
              {terminals.map((terminal) => {
                const stats = state.stats.find((item) => item.terminalId === terminal.id)
                return (
                  <tr key={terminal.id}>
                    <th scope="row" className="px-4 py-4 font-semibold text-neutral-900">
                      {terminal.nombre}
                    </th>
                    <td className="px-4 py-4 text-right tabular-nums">{stats?.hoy ?? 0}</td>
                    <td className="px-4 py-4 text-right tabular-nums">{stats?.aprobadas ?? 0}</td>
                    <td className="px-4 py-4 text-right tabular-nums">{stats?.rechazadas ?? 0}</td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}

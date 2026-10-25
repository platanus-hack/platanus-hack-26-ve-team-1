import { useId, useRef, type KeyboardEvent, type ReactNode } from 'react'
import { cn, focusRing } from '../../lib/cn'

export type TabItem = {
  id: string
  label: string
  panel: ReactNode
  /** Si se define, muestra un punto de aviso en la pestaña; el texto es para lectores de pantalla. */
  notice?: string
}

type TabsProps = {
  items: TabItem[]
  activeId: string
  onChange: (id: string) => void
  /** Nombre accesible de la lista de pestañas. */
  label: string
}

// Todos los paneles quedan montados (solo se ocultan) para conservar su estado.
export function Tabs({ items, activeId, onChange, label }: TabsProps) {
  const baseId = useId()
  const tabRefs = useRef<Array<HTMLButtonElement | null>>([])

  const tabId = (id: string) => `${baseId}-tab-${id}`
  const panelId = (id: string) => `${baseId}-panel-${id}`

  function handleKeyDown(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    const last = items.length - 1
    const target =
      event.key === 'ArrowRight'
        ? index === last
          ? 0
          : index + 1
        : event.key === 'ArrowLeft'
          ? index === 0
            ? last
            : index - 1
          : event.key === 'Home'
            ? 0
            : event.key === 'End'
              ? last
              : null
    if (target === null) return
    event.preventDefault()
    tabRefs.current[target]?.focus()
    onChange(items[target].id)
  }

  return (
    <div className="flex flex-col gap-6">
      <div
        role="tablist"
        aria-label={label}
        className="-mx-6 flex gap-2 overflow-x-auto px-6 py-1 md:mx-0 md:px-0"
      >
        {items.map((item, index) => {
          const selected = item.id === activeId
          return (
            <button
              key={item.id}
              ref={(element) => {
                tabRefs.current[index] = element
              }}
              id={tabId(item.id)}
              type="button"
              role="tab"
              aria-selected={selected}
              aria-controls={panelId(item.id)}
              tabIndex={selected ? 0 : -1}
              onClick={() => onChange(item.id)}
              onKeyDown={(event) => handleKeyDown(event, index)}
              className={cn(
                'inline-flex min-h-12 shrink-0 items-center gap-2 rounded-xl border-2 px-5 py-2 text-lg font-semibold whitespace-nowrap transition-colors',
                focusRing,
                selected
                  ? 'border-primary-700 bg-primary-700 text-white'
                  : 'border-neutral-300 bg-white text-primary-900 hover:bg-primary-50',
              )}
            >
              {item.label}
              {item.notice && (
                <>
                  <span
                    aria-hidden="true"
                    className="size-3 rounded-full bg-approved-500 ring-2 ring-white"
                  />
                  <span className="sr-only">{item.notice}</span>
                </>
              )}
            </button>
          )
        })}
      </div>

      {items.map((item) => (
        <div
          key={item.id}
          id={panelId(item.id)}
          role="tabpanel"
          aria-labelledby={tabId(item.id)}
          hidden={item.id !== activeId}
          tabIndex={0}
          className={cn('flex flex-col gap-6 rounded-xl', focusRing)}
        >
          {item.panel}
        </div>
      ))}
    </div>
  )
}

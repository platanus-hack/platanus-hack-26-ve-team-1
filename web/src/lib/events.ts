import type { BadgeVariant } from '../components/ui'
import type { EventAttribute, EventItem, EventResult } from '../api/types'

/**
 * Códigos de resultado del backend (1–6):
 * 1 Aprobado · 2 Firma inválida · 3 Cédula revocada · 4 Cédula vencida ·
 * 5 Sin permiso para ese dato · 6 Rechazado por ti.
 * La emergencia no es un código: llega aparte como `emergencia: true`.
 */
export const RESULT_INFO: Record<EventResult, { label: string; variant: BadgeVariant }> = {
  1: { label: 'Aprobado', variant: 'ok' },
  2: { label: 'Firma inválida', variant: 'error' },
  3: { label: 'Cédula revocada', variant: 'error' },
  4: { label: 'Cédula vencida', variant: 'warning' },
  5: { label: 'Sin permiso para ese dato', variant: 'warning' },
  6: { label: 'Rechazado por ti', variant: 'neutral' },
}

const UNKNOWN_RESULT = { label: 'Desconocido', variant: 'neutral' } as const

export function getResultInfo(code: EventResult | null): {
  label: string
  variant: BadgeVariant
} {
  return code === null ? UNKNOWN_RESULT : RESULT_INFO[code]
}

/** Códigos de atributo del backend (1–6). */
export const EVENT_ATTRIBUTE_LABELS: Record<EventAttribute, string> = {
  1: 'Mayor de 18',
  2: 'Mayor de 21',
  3: 'Identidad',
  4: 'Tipo de sangre y alergias',
  5: 'Contacto de emergencia',
  6: 'Donante y licencia',
}

export function getAttributeLabel(code: EventAttribute | null): string {
  return code === null ? 'Dato desconocido' : EVENT_ATTRIBUTE_LABELS[code]
}

function toCode(value: number): EventResult | null {
  return Number.isInteger(value) && value >= 1 && value <= 6 ? (value as EventResult) : null
}

/** Acepta 1–6 o "01"–"06". */
function parseResult(value: unknown): EventResult | null {
  if (typeof value === 'number') return toCode(value)
  if (typeof value === 'string' && /^0?[1-6]$/.test(value)) return toCode(Number(value))
  return null
}

/** Acepta 1–6 o "A1"–"A6". */
function parseAttribute(value: unknown): EventAttribute | null {
  if (typeof value === 'number') return toCode(value)
  const match = typeof value === 'string' ? /^A([1-6])$/i.exec(value) : null
  return match ? toCode(Number(match[1])) : null
}

function parseId(value: unknown): number | null {
  if (typeof value === 'number') return Number.isSafeInteger(value) && value >= 0 ? value : null
  if (typeof value === 'string' && /^\d+$/.test(value)) {
    const id = Number(value)
    return Number.isSafeInteger(id) ? id : null
  }
  return null
}

/**
 * Convierte la forma del backend ({ id, local, atributo, resultado, hora, emergencia }) al
 * EventItem interno, aceptando atributo "A1"–"A6" o 1–6 y resultado 1–6 o "01"–"06".
 * Devuelve null solo si el mensaje no es un objeto o no trae id; los códigos
 * inválidos quedan en null y se muestran como "Desconocido".
 */
export function normalizeEvent(raw: unknown): EventItem | null {
  if (typeof raw !== 'object' || raw === null) return null
  const data = raw as Record<string, unknown>
  const id = parseId(data.id)
  if (id === null) return null

  const hora =
    typeof data.hora === 'string' && !Number.isNaN(Date.parse(data.hora)) ? data.hora : ''

  return {
    id,
    local:
      typeof data.local === 'string' && data.local.trim()
        ? data.local.trim()
        : 'Lugar desconocido',
    atributo: parseAttribute(data.atributo),
    resultado: parseResult(data.resultado),
    hora,
    emergencia: data.emergencia === true,
  }
}

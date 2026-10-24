const dateFormatter = new Intl.DateTimeFormat('es-CL', { dateStyle: 'long' })
const dateTimeFormatter = new Intl.DateTimeFormat('es-CL', {
  dateStyle: 'medium',
  timeStyle: 'medium',
})

export function formatDate(iso: string): string {
  return dateFormatter.format(new Date(iso))
}

/** Fecha y hora en la zona horaria local del navegador. */
export function formatDateTime(iso: string): string {
  const date = new Date(iso)
  return Number.isNaN(date.getTime()) ? 'Hora desconocida' : dateTimeFormatter.format(date)
}

const dateFormatter = new Intl.DateTimeFormat('es-CL', { dateStyle: 'long' })

export function formatDate(iso: string): string {
  return dateFormatter.format(new Date(iso))
}

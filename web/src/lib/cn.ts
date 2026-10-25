export function cn(...classes: Array<string | false | null | undefined>): string {
  return classes.filter(Boolean).join(' ')
}

export const focusRing =
  'focus-visible:outline-4 focus-visible:outline-offset-2 focus-visible:outline-primary-500'

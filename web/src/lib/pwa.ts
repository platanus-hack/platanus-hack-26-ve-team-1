// Instalación y modo sin red de la billetera y el verificador. Sin dependencias: un service worker propio
// (public/sw-p4.js) y un manifiesto por pantalla. El service worker NUNCA toca /api/*.

export type PwaKind = 'billetera' | 'verificar'

const MANIFEST: Record<PwaKind, string> = {
  billetera: '/billetera.webmanifest',
  verificar: '/verificar.webmanifest',
}

const THEME_COLOR = '#16358a'

function whenActive(registration: ServiceWorkerRegistration): Promise<ServiceWorker | null> {
  const worker = registration.installing ?? registration.waiting
  if (!worker) return Promise.resolve(registration.active)
  return new Promise((resolve) => {
    worker.addEventListener('statechange', () => {
      if (worker.state === 'activated') resolve(worker)
      else if (worker.state === 'redundant') resolve(null)
    })
  })
}

/** Archivos que la página ya cargó (JS, CSS, fuentes): el service worker los guarda para abrir sin red. */
function loadedResources(): string[] {
  return (performance.getEntriesByType('resource') as PerformanceResourceTiming[])
    .map((entry) => new URL(entry.name, location.href))
    .filter((url) => url.origin === location.origin && !url.pathname.startsWith('/api/') && !url.pathname.endsWith('.map'))
    .map((url) => url.pathname + url.search)
}

async function registerWorker(kind: PwaKind): Promise<void> {
  if (import.meta.env.DEV || !('serviceWorker' in navigator)) return
  try {
    const registration = await navigator.serviceWorker.register('/sw-p4.js', { scope: `/${kind}` })
    const worker = registration.active ?? (await whenActive(registration))
    worker?.postMessage({ type: 'cache-urls', urls: loadedResources() })
  } catch {
    // Sin service worker la pantalla funciona igual, solo que no abre sin red.
  }
}

/**
 * Prepara la pantalla como app instalable y la deja disponible sin red. Devuelve la función que quita lo
 * que se añadió al salir de la pantalla, para no dejar el manifiesto de una en las demás.
 */
export function setupPwa(kind: PwaKind): () => void {
  const added: Element[] = []
  const add = (tag: 'link' | 'meta', attributes: Record<string, string>) => {
    const element = document.createElement(tag)
    for (const [name, value] of Object.entries(attributes)) element.setAttribute(name, value)
    document.head.appendChild(element)
    added.push(element)
  }
  add('link', { rel: 'manifest', href: MANIFEST[kind] })
  add('link', { rel: 'apple-touch-icon', href: '/apple-touch-icon.png' })
  add('meta', { name: 'theme-color', content: THEME_COLOR })
  add('meta', { name: 'mobile-web-app-capable', content: 'yes' })
  add('meta', { name: 'apple-mobile-web-app-capable', content: 'yes' })
  add('meta', { name: 'apple-mobile-web-app-title', content: 'sIsA' })
  void registerWorker(kind)
  return () => added.forEach((element) => element.remove())
}

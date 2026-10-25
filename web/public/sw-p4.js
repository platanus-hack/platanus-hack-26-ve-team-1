// Service worker de la billetera y el verificador de sIsA.
//
// Objetivo: que /billetera y /verificar abran y funcionen SIN internet tras la primera carga.
// Reglas de seguridad: nunca intercepta /api/* (credenciales, revocaciones y eventos siempre van por la red
// y no se guardan aquí), solo responde a GET del mismo origen y no guarda cabeceras ni cookies a mano.

const CACHE = 'sisa-p4-v1'
const SHELL = '/index.html'

self.addEventListener('install', () => {
  self.skipWaiting()
})

self.addEventListener('activate', (event) => {
  event.waitUntil(
    (async () => {
      for (const key of await caches.keys()) {
        if (key.startsWith('sisa-p4-') && key !== CACHE) await caches.delete(key)
      }
      await self.clients.claim()
    })(),
  )
})

function cacheable(rawUrl) {
  const url = new URL(rawUrl, self.location.origin)
  return url.origin === self.location.origin && !url.pathname.startsWith('/api/')
}

// La página pide guardar los archivos que ya cargó: así se puede abrir sin red desde la primera visita.
self.addEventListener('message', (event) => {
  if (!event.data || event.data.type !== 'cache-urls' || !Array.isArray(event.data.urls)) return
  event.waitUntil(
    (async () => {
      const cache = await caches.open(CACHE)
      const urls = [SHELL, ...event.data.urls].filter((url) => typeof url === 'string' && cacheable(url))
      await Promise.allSettled(urls.map((url) => cache.add(url)))
    })(),
  )
})

// Navegación: red primero (siempre la versión nueva) y, sin red, la última página guardada.
async function navigate(request) {
  try {
    const response = await fetch(request)
    const type = response.headers.get('content-type') || ''
    if (response.ok && type.includes('text/html')) {
      const cache = await caches.open(CACHE)
      await cache.put(SHELL, response.clone())
    }
    return response
  } catch {
    return (await caches.match(SHELL)) || Response.error()
  }
}

// /assets/* tiene el hash en el nombre: nunca cambia, así que se sirve del caché sin preguntar a la red.
async function cacheFirst(request) {
  const cached = await caches.match(request)
  if (cached) return cached
  const response = await fetch(request)
  if (response.ok) {
    const cache = await caches.open(CACHE)
    await cache.put(request, response.clone())
  }
  return response
}

// Íconos, manifiestos y demás: se muestra lo guardado y se actualiza en segundo plano.
async function staleWhileRevalidate(request) {
  const cache = await caches.open(CACHE)
  const cached = await cache.match(request)
  const refresh = fetch(request)
    .then((response) => {
      if (response.ok) cache.put(request, response.clone())
      return response
    })
    .catch(() => cached)
  return cached || refresh
}

self.addEventListener('fetch', (event) => {
  const request = event.request
  if (request.method !== 'GET' || !cacheable(request.url)) return
  const url = new URL(request.url)
  if (request.mode === 'navigate') event.respondWith(navigate(request))
  else if (url.pathname.startsWith('/assets/')) event.respondWith(cacheFirst(request))
  else event.respondWith(staleWhileRevalidate(request))
})

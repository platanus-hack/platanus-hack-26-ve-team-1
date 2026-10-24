# Despliegue en Render (P4 → P5)

Solo se puede desplegar en Render (no en Vercel). Render solo se conecta a repos propios: espejar el repo a uno personal como indica el README y conectar ese.

## Ya disponible: laboratorio de P4

`render.yaml` define un Static Site (`slj-p4-lab`) que publica `app/public/p4-lab`. En Render: **New → Blueprint**, elegir el repo y la rama, y aplicar. La URL HTTPS resultante sirve para probar en iPhone y Android (WebCrypto, IndexedDB y cámara exigen contexto seguro).

## Para cuando exista el scaffold de app/ (P5)

Un solo **Web Service** de Node para portal, PWAs y API, de modo que todo comparta origen (la llave del celular vive por origen):

- Build: `npm ci && npm run build` · Start: `npm run start` (con `rootDir: app`).
- Pinnear Node con `NODE_VERSION` y `engines` en `package.json`.
- `healthCheckPath` a una ruta liviana, por ejemplo `/api/health`.
- Las `NEXT_PUBLIC_*` se fijan en el build: definirlas en Render antes del primer despliegue.
- Claves privadas del emisor y secretos de Supabase y Kapso solo en el panel de Render, nunca en el repo.
- URL pública del servicio = URL del webhook de Kapso y base de `/api` para el gateway de P3.

### Cabeceras del service worker (`next.config`)

```js
async headers() {
  return [
    { source: "/sw-p4.js", headers: [
      { key: "Cache-Control", value: "no-cache" },
      { key: "Service-Worker-Allowed", value: "/" },
    ]},
    { source: "/:path(billetera|verificar).webmanifest", headers: [
      { key: "Cache-Control", value: "public, max-age=3600" },
    ]},
  ];
},
```

### Arranque en frío

En el plan gratuito el servicio se duerme tras un rato sin tráfico. Afecta a `/api/link/confirm`, `/api/wallet` y `/api/events`, no a la verificación, que es local. Con créditos, usar un plan que no se duerma para la demo.

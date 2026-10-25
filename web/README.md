# sIsA · portal web

Frontend de sIsA, un prototipo de cédula electrónica que muestra solo el dato necesario (por ejemplo, «mayor de 18») y no los datos completos. React + TypeScript + Vite + Tailwind CSS v4.

## Comandos

Usa Node 22.

```bash
npm install
npm run dev     # servidor de desarrollo
npm run build   # comprobación de tipos + build de producción
npm run lint    # Oxlint
```

## Variables de entorno

Copia `.env.example` a `.env.local` (ignorado por git).

- `VITE_USE_MOCK=true`: toda la capa `src/api` usa datos de demostración, sin backend. Cuentas demo (contraseña `demo1234`): `ana@sisa.demo`, `luis@sisa.demo`, `maria@sisa.demo` (ciudadanos), `emisor@sisa.demo` y `comercio@sisa.demo`.
- `VITE_USE_MOCK=false` (o sin definir): se llama a la API real con la cookie de sesión (`credentials: "include"`). En producción debe ir en `false`: así el código del mock no entra al bundle.
- `VITE_API_URL`: origen del backend, p. ej. `https://api.sisa.example.com` (sin `/api` ni barra final). Todas las llamadas van a `${VITE_API_URL}/api/*`. Vite la incorpora al **compilar**, así que hay que definirla antes de `npm run build`. Vacía: usa `/api` en el mismo origen.

## Despliegue

El front es un sitio estático (carpeta `dist/`) para una CDN; el backend corre aparte, en otra instancia.

1. Compila con `VITE_API_URL` apuntando al backend y `VITE_USE_MOCK` sin definir.
2. Publica `dist/` y configura el *fallback* de SPA: toda ruta desconocida debe servir `index.html` (para `/emisor`, `/comercio`, etc. al recargar).
3. El backend debe permitir el origen del front (CORS con credenciales) y emitir la cookie de sesión de forma que viaje entre ambos sitios. Detalle en `docs/PENDIENTES_P1.md`, sección "Despliegue: front en CDN, back externo".

## Rutas

| Ruta | Rol | Contenido |
|------|-----|-----------|
| `/` | ciudadano | Portal ciudadano |
| `/emisor` | emisor | Cédulas y carnets universitarios |
| `/comercio` | comercio | Terminales y uso |
| `/login`, `/registro` | público | Acceso |
| `/billetera`, `/verificar` | (P4) | Placeholders |
| `/ui-kit` | solo en desarrollo | Catálogo de componentes |

## Estructura de `src/`

`api/` (cliente, tipos y mock) · `components/` (`ui/` genéricos, `issuer/`, `merchant/`) · `hooks/` · `layouts/` · `lib/` · `pages/`.

Los endpoints que el backend aún no confirma están en `docs/PENDIENTES_P1.md`.

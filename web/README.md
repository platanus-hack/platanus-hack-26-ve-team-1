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
- `VITE_USE_MOCK=false` (o sin definir): se llama a la API real en `/api/*` con la cookie de sesión (`credentials: "include"`). En producción debe ir en `false`: así el código del mock no entra al bundle.

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

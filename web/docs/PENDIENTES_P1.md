# Pendientes con P1 (backend)

Endpoints y formas JSON que el frontend de sIsA necesita y que **no están en el contrato** acordado. Hay que confirmarlos o ajustar el frontend.

> Las rutas de auth están definidas en `AUTH_ENDPOINTS` (`api/auth.ts`). Todas son relativas a la base `${VITE_API_URL}/api` y se llaman con `credentials: "include"` (JWT en cookie HttpOnly).

## Despliegue: front en CDN, back externo

El front es estático (CDN) y llama directamente al backend en otra instancia: `fetch` y `EventSource` van a `${VITE_API_URL}/api/*` (cross-origin, con credenciales). Lo que el backend debe hacer:

1. **CORS con credenciales** en todas las rutas `/api/*`, incluido el stream SSE y las respuestas de error:
   - `Access-Control-Allow-Origin: <origen exacto del front>` (no `*`: con credenciales el comodín no vale).
   - `Access-Control-Allow-Credentials: true`.
   - `Access-Control-Allow-Headers: Content-Type, Accept` y `Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS`.
   - Responder el *preflight* `OPTIONS` (el front envía `Content-Type: application/json` en POST/PUT).
   - `Vary: Origin`.
2. **Cookie de sesión** que viaje entre ambos sitios (`HttpOnly` siempre, y `Secure`):
   - Si front y back comparten dominio registrable (p. ej. `app.sisa.cl` y `api.sisa.cl`): `SameSite=Lax` funciona y es lo preferible. Conviene fijar `Domain=.sisa.cl` solo si hace falta.
   - Si son dominios distintos (p. ej. `*.onrender.com` y otro): hace falta `SameSite=None; Secure`, y algunos navegadores bloquean cookies de terceros: en ese caso el login no se mantiene. **Recomendado: poner ambos bajo el mismo dominio.**
3. **SSE** (`/api/events/stream`) con las mismas cabeceras CORS y `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `X-Accel-Buffering: no`, keep-alive cada 25 s.
4. **HTTPS** en ambos (la cookie `Secure` lo exige).
5. Datos a pasarle al front: la URL pública del backend para `VITE_API_URL`, y el origen del front para configurar CORS.

Preguntas para P1: ¿bajo qué dominio quedará el backend? ¿el front y el back compartirán dominio registrable? ¿cuál es el origen exacto del front para la lista de CORS?

## Endpoints por confirmar

| Método | Ruta (con `/api`) | Uso | Respuesta esperada |
|--------|-------------------|-----|--------------------|
| `GET` | `/api/me` | Restaurar la sesión al cargar la app. | `200` con un `User`; `401` si no hay sesión. |
| `POST` | `/api/auth/logout` | Cerrar sesión (el backend debe borrar la cookie). | `204` sin cuerpo, o `200`. |
| `GET` | `/api/card` | Cédula del usuario autenticado (portal ciudadano). Ruta en `CARD_ENDPOINTS` (`api/card.ts`). | `200` con un `CardInfo`; `404` si aún no tiene cédula. |
| `GET` | `/api/me/optional-data` | Precargar el formulario "Mis datos opcionales". Ruta en `OPTIONAL_DATA_ENDPOINTS` (`api/optionalData.ts`). | `200` con un `OptionalData` (valores vacíos si no hay datos guardados). |

Ya están en el contrato: `POST /api/auth/register`, `POST /api/auth/login` (ambos responden con un `User`), `PUT /api/me/optional-data` (recibe y devuelve un `OptionalData`), `GET /api/events` y `GET /api/events/stream` (SSE con cookie, ver más abajo). Rutas en `EVENT_ENDPOINTS` (`api/events.ts`).

## Forma JSON de `EventItem` (confirmada por P1, commit 988d3ac)

`GET /api/events` responde `{ "ok": true, "events": [...] }`, del más reciente al más antiguo. Admite `?limit=50&before=<id>` (máximo 200); el frontend usa `limit=50` en la carga inicial. Cada elemento de `events` y el `data` de cada evento del stream:

```json
{
  "id": 123,
  "local": "Bar El Faro",
  "atributo": "A1",
  "resultado": 1,
  "hora": "2026-10-24T22:15:00Z",
  "emergencia": false
}
```

- `id`: número único (obligatorio; los mensajes sin `id` se ignoran). Un `id` repetido se ignora.
- `resultado`: número 1–6 (el frontend también acepta `"01"`–`"06"`). Un valor fuera de rango se muestra como "Desconocido".

  | Código | Significado |
  |--------|-------------|
  | 1 | Aprobado |
  | 2 | Firma inválida |
  | 3 | Cédula revocada |
  | 4 | Cédula vencida |
  | 5 | Sin permiso para ese dato |
  | 6 | No compartiste |

- `atributo`: `"A1"`–`"A6"` (el frontend también acepta 1–6).

  | Código | Dato |
  |--------|------|
  | 1 / A1 | Mayor de 18 |
  | 2 / A2 | Mayor de 21 |
  | 3 / A3 | Identidad |
  | 4 / A4 | Tipo de sangre y alergias |
  | 5 / A5 | Contacto de emergencia |
  | 6 / A6 | Donante y licencia |

- `emergencia`: boolean. No es un código de resultado: es `true` en accesos de emergencia (típicamente con `resultado` 1).
- `hora`: RFC 3339 en UTC (el frontend la muestra en hora local).
- `local`: nombre del lugar.
- No debe incluir handles ni UID: el frontend solo muestra lugar, dato, hora y resultado.
- **Los eventos de tipo tamper no deben llegar al ciudadano**, ni por `GET /api/events` ni por el stream.

### Stream SSE (`GET /api/events/stream`)

- Envía eventos con nombre `evento` (`event: evento`) y un `EventItem` en JSON en `data:`. Solo envía eventos nuevos; el historial viene del GET. El frontend escucha con `addEventListener("evento", ...)`, ignora JSON inválido o sin `id` sin cerrar la conexión y reconecta solo (1 s, 2 s, 4 s… máximo 15 s), enviando `Last-Event-ID` por su cuenta.
- Requisitos del servidor: `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `X-Accel-Buffering: no` y un comentario keep-alive (`: keep-alive`) cada 25 s.

## Forma JSON de `OptionalData`

```json
{
  "tipoSangre": "O+",
  "alergias": "penicilina, maní",
  "contactoEmergencia": { "nombre": "Rosa Pérez", "telefono": "+56912345678" },
  "donante": true
}
```

- `tipoSangre`: `"A+"`, `"A-"`, `"B+"`, `"B-"`, `"AB+"`, `"AB-"`, `"O+"`, `"O-"` o `null`.
- `alergias` (A4): texto UTF-8 de **máximo 16 bytes** (puede ser vacío). Cuenta bytes, no caracteres: `á`, `ñ` y los emojis ocupan más de 1.
- `contactoEmergencia` (A5): el texto que se escribe en el chip es **nombre + teléfono juntos y debe caber en 16 bytes**. El frontend lo compone como `"<nombre> <teléfono sin '+'>"` (constante `EMERGENCY_CONTACT_CHIP_SEPARATOR` y función `composeEmergencyContactChipText`, `lib/optionalData.ts`). Ejemplo: `"Mamá 04141234567"`.
- `contactoEmergencia.telefono`: formato internacional sin espacios (`+` y 8 a 15 dígitos), o vacío si no hay contacto. Nombre y teléfono van juntos o ambos vacíos.
- `donante`: booleano.
- **Límite de 16 bytes en el servidor:** `PUT /api/me/optional-data` debe responder `400` con un mensaje **en español** (el frontend lo muestra tal cual en un toast y conserva lo escrito) cuando `alergias` o el texto combinado del contacto superen 16 bytes UTF-8. El frontend ya lo valida antes de enviar (contador "X/16 bytes").

### Preguntas abiertas para P1 (datos opcionales)

1. ¿El chip guarda **nombre y teléfono juntos** (un solo texto de 16 bytes) o **por separado**? Si es por separado, ¿cuántos bytes tiene cada uno?
2. ¿El teléfono se guarda **sin `+` ni código de país**? Hoy el frontend pide formato internacional (`+56912345678`) y, para el chip, solo quita el `+` (queda `56912345678`, 11 bytes). Con un teléfono así quedan ~4 bytes para el nombre.
3. El ejemplo `"Mamá 04141234567"` tiene 16 **caracteres** pero **17 bytes** UTF-8 (la `á` ocupa 2): ¿el límite es de bytes (como se asume) o de caracteres?

## Bloqueo de cédula y avisos por WhatsApp

Endpoints que usa el frontend (rutas en `CARD_ENDPOINTS` y `NOTIFICATIONS_ENDPOINTS`); confirmar la forma de las respuestas:

| Método | Ruta (con `/api`) | Cuerpo | Respuesta esperada |
|--------|-------------------|--------|--------------------|
| `POST` | `/api/revoke` | sin cuerpo | `2xx`. Tras el éxito el frontend marca la cédula como revocada (`estado: "revocada"`, `atributos: []`) sin volver a pedirla. |
| `PUT` | `/api/me/notifications` | `{ "telefono": "+56912345678", "avisosActivados": true }` | `200` con el mismo JSON guardado. `telefono` en formato internacional sin espacios, o `null`. |

- El teléfono de WhatsApp es `User.telefono`, distinto del contacto de emergencia (`OptionalData`).
- Con `avisosActivados: true` el teléfono es obligatorio (el frontend ya lo valida).
- El usuario puede responder `BLOQUEAR` al aviso de WhatsApp para bloquear su cédula (lo gestiona el backend).

### Preguntas abiertas para P1

1. **`POST /api/revoke`**: ¿confirmas que va **sin body**, que la cédula se identifica **por la sesión** (cookie; el front no envía handle ni UID) y que responde `2xx` (el frontend no lee el cuerpo)?
2. **Formato de teléfono**: ¿Kapso espera **E.164**? El frontend lo envía normalizado: `+` seguido de 8 a 15 dígitos, sin espacios ni guiones (p. ej. `+56912345678`).
3. **`PUT /api/me/notifications`**: ¿confirmas el body `{ "telefono": "+56912345678", "avisosActivados": true }` (con `telefono: null` si no hay) y que responde `200` con **el mismo JSON** guardado?

## Panel del emisor (`/emisor`): endpoints supuestos

Todo lo siguiente es un **supuesto del frontend**, pendiente de confirmar con P1 (rutas en `ISSUER_ENDPOINTS`, `api/issuer.ts`, y `CARD_ENDPOINTS`, `api/card.ts`). Todos exigen rol `emisor`.

| Método | Ruta (con `/api`) | Cuerpo | Respuesta esperada |
|--------|-------------------|--------|--------------------|
| `GET` | `/api/citizens` | — | `200` con una lista de `Citizen` (ver abajo). |
| `POST` | `/api/emision/order` | `{ "ciudadanoId": "u-123" }` | `2xx`. Deja al terminal en modo emisión esperando una tarjeta virgen. |
| `GET` | `/api/emision/status` | — | `200` con `{ "estado": "pendiente" \| "completado" \| "error", "mensaje"?: "..." }`: estado de la emisión en curso del emisor. |
| `POST` | `/api/revoke` | `{ "ciudadanoId": "u-123" }` | `2xx`. El emisor revoca la cédula de otro ciudadano (distinto del bloqueo propio, que va sin cuerpo). |
| `PUT` | `/api/citizens/{id}/profile` | `{ "fechaNacimiento": "1999-04-30", "cedula": "12345678" }` | `2xx`. Completa el perfil del ciudadano. **Solo lo acepta el rol `emisor`** (otro rol: `403`). `fechaNacimiento` en `YYYY-MM-DD`; `cedula` solo dígitos (el frontend valida 6 a 9). Errores `400` con mensaje en español. |

```json
{
  "id": "u-123",
  "nombre": "Ana Pérez",
  "email": "ana@sisa.demo",
  "estadoCedula": "activa",
  "emitidaEn": "2026-03-12T10:00:00Z",
  "perfilCompleto": true
}
```

- `estadoCedula`: `"activa"`, `"revocada"` o `"sin_emitir"` (en ese caso `emitidaEn` es `null`).
- **`GET /api/citizens` debe devolver `perfilCompleto`** (`true` si el ciudadano ya tiene fecha de nacimiento y cédula). Sin perfil completo el frontend deshabilita "Emitir cédula" y ofrece "Completar perfil". **No debe devolver la fecha de nacimiento ni la cédula**: el frontend nunca las muestra ni las guarda tras enviarlas. Además, `POST /api/emision/order` debería rechazar (`409`/`400`) a un ciudadano sin perfil completo.
- `id` es interno: el frontend lo usa solo para llamar a la API y nunca lo muestra (tampoco handles ni UID).
- El frontend consulta `GET /api/emision/status` cada **2 s** y se rinde a los **2 minutos** (muestra un error al emisor). Deja de consultar si el emisor cierra el panel.
- Preguntas abiertas para P1:
  1. ¿`POST /api/emision/order` devuelve un id de orden para consultar `?order=<id>`?
  2. ¿La lista de ciudadanos admite paginación o búsqueda en servidor?
  3. **¿Qué terminal atiende `POST /api/emision/order`?** Si hay varios terminales en modo emisión, ¿cómo se elige uno (por emisor, por sesión, por id de terminal)? El frontend no envía ningún id de terminal.
  4. **Validación en servidor de `{ "ciudadanoId" }` en `POST /api/revoke`:** solo debe aceptarse si la sesión tiene rol `emisor`; un ciudadano que envíe ese campo debe recibir `403` (el ciudadano solo puede bloquear su propia cédula, sin cuerpo).
  5. **Confirmar que `GET /api/citizens` devuelve únicamente** `id` (interno), `nombre`, `email`, `estadoCedula`, `emitidaEn` y `perfilCompleto`, **sin handle ni UID** y sin fecha de nacimiento ni cédula.
  6. ¿Cuántos dígitos tiene un número de cédula válido? El frontend acepta de 6 a 9 (`CEDULA_MIN_DIGITS` / `CEDULA_MAX_DIGITS`, `lib/validation.ts`).
- El frontend tolera hasta 3 errores HTTP seguidos en `GET /api/emision/status` antes de marcar fallo; una respuesta correcta reinicia la cuenta. Si el emisor cierra el panel con la emisión pendiente (tras confirmar), el frontend solo deja de consultar: **no cancela** la orden en el servidor (no hay endpoint de cancelación).

## Panel del emisor, pestaña "Universidad": endpoints supuestos

Supuestos del frontend, pendientes de confirmar con P1 (rutas en `UNIVERSITY_ENDPOINTS`, `api/university.ts`). Ambos exigen rol `emisor`. El frontend solo muestra la UI y hace la llamada: no hay lógica criptográfica en el cliente.

| Método | Ruta (con `/api`) | Cuerpo | Respuesta esperada |
|--------|-------------------|--------|--------------------|
| `GET` | `/api/students` | — | `200` con una lista de `Student` (ver abajo). |
| `POST` | `/api/credentials/university` | `{ "estudianteId": "u-123" }` | `2xx`. Tras el éxito el frontend vuelve a pedir la lista. Un `409` se muestra como "ya tiene carnet". |

```json
{
  "id": "u-123",
  "nombre": "Ana Pérez",
  "email": "ana@sisa.demo",
  "estadoCarnet": "sin_carnet",
  "emitidoEn": null
}
```

- `estadoCarnet`: `"sin_carnet"` o `"emitido"` (en este caso `emitidoEn` es una fecha ISO 8601).
- `id` es interno: el frontend lo usa solo para llamar a la API y nunca lo muestra (tampoco handles ni UID).
- Preguntas abiertas: ¿el `POST` devuelve el carnet emitido o solo `2xx`? ¿Un estudiante con carnet emitido puede volver a recibir uno (re-emisión)? El frontend hoy no ofrece esa acción.

## Terminales del comercio (`/comercio`): endpoints supuestos

Supuestos del frontend, pendientes de confirmar con P1 (rutas en `TERMINAL_ENDPOINTS`, `api/terminals.ts`). Todos exigen rol `comercio`.

| Método | Ruta (con `/api`) | Cuerpo | Respuesta esperada |
|--------|-------------------|--------|--------------------|
| `GET` | `/api/terminals` | — | `200` con una lista de `Terminal` **sin token**. |
| `POST` | `/api/terminals` | `TerminalInput` | `2xx` con el `Terminal` creado **incluyendo `token`** (única vez que se entrega). |
| `PUT` | `/api/terminals/:id` | `TerminalInput` | `200` con el `Terminal` actualizado, sin token. |
| `DELETE` | `/api/terminals/:id` | — | `2xx`. El terminal deja de funcionar y su token queda inválido. |
| `GET` | `/api/stats` | — | `200` con una lista de `{ "terminalId", "hoy", "aprobadas", "rechazadas" }`. |

```json
{ "id": "t-1", "nombre": "Caja principal", "rol": "comercio", "atributos": ["mayor_de_18"] }
```

```json
{ "nombre": "Caja principal", "rol": "comercio", "atributos": ["mayor_de_18"] }
```

- `rol` del terminal (no confundir con el rol del usuario): `"comercio"`, `"registro"`, `"salud"` o `"emergencia"`. `atributos` usa los valores de `CardInfo.atributos`.
- Datos que pide cada tipo (el frontend los fija; el servidor debe validarlos):
  - `comercio` → `mayor_de_18` **o** `mayor_de_21` (uno solo).
  - `registro` → `identidad`.
  - `salud` → `sangre_alergias`, `contacto_emergencia`, `donante_licencia`.
  - `emergencia` → `sangre_alergias`, `contacto_emergencia`. **Lee sin consentimiento del ciudadano**: la UI lo avisa.
- El token solo se muestra una vez en un Modal; el frontend no lo guarda en estado global, `localStorage` ni `sessionStorage`, y lo descarta al cerrar el Modal.
- `GET /api/stats` devuelve solo totales por terminal: nunca handles, UID ni datos personales. `terminalId` es interno y no se muestra.
- Preguntas abiertas: ¿`PUT` permite cambiar el `rol` de un terminal existente? ¿Hace falta un endpoint para regenerar el token? ¿`stats` admite filtrar por fecha?

## Forma JSON de `CardInfo`

```json
{
  "id": "c-ana",
  "estado": "activa",
  "emitidaEn": "2026-03-12T10:00:00Z",
  "atributos": ["mayor_de_18", "mayor_de_21", "identidad", "sangre_alergias", "contacto_emergencia", "donante_licencia"]
}
```

- `estado`: `"activa"` o `"revocada"`. Una cédula revocada devuelve `atributos: []`.
- `atributos`: valores exactos (minúscula, con guion bajo) de la lista del ejemplo.

## Forma JSON de `User`

Respuesta de `register`, `login` y `GET /me`:

```json
{
  "id": "u-123",
  "email": "ana@sisa.demo",
  "nombre": "Ana Pérez",
  "rol": "ciudadano",
  "telefono": "+56 9 1111 1111",
  "avisosActivados": true
}
```

- `id`: string.
- `rol`: exactamente `"ciudadano"`, `"emisor"` o `"comercio"` (minúscula, sin tilde).
- `telefono`: string o `null` si no hay.
- `avisosActivados`: booleano.

## Errores

El frontend lee el mensaje de `{ "error": "..." }` o `{ "message": "..." }` y lo muestra al usuario, por lo que debe venir **en español**. Un `401` en cualquier endpoint (salvo `login`) cierra la sesión localmente.

| Caso | Estado esperado |
|------|-----------------|
| Credenciales incorrectas en `login` | `401` |
| Correo ya registrado en `register` | `409` |
| Datos inválidos (p. ej. contraseña corta) | `400` |

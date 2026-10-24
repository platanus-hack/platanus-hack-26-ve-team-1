# Pendientes con P1 (backend)

Endpoints y formas JSON que el frontend de sIsA necesita y que **no están en el contrato** acordado. Hay que confirmarlos o ajustar el frontend.

> Las rutas de auth están definidas en `AUTH_ENDPOINTS` (`api/auth.ts`). Todas son relativas a la base `/api` y se llaman con `credentials: "include"` (JWT en cookie HttpOnly).

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
  | 6 | Rechazado por ti |

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
- `alergias`: string de hasta 200 caracteres (puede ser vacío).
- `contactoEmergencia.telefono`: formato internacional sin espacios (`+` y 8 a 15 dígitos), o vacío si no hay contacto. Nombre y teléfono van juntos o ambos vacíos.
- `donante`: booleano.

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

```json
{
  "id": "u-123",
  "nombre": "Ana Pérez",
  "email": "ana@sisa.demo",
  "estadoCedula": "activa",
  "emitidaEn": "2026-03-12T10:00:00Z"
}
```

- `estadoCedula`: `"activa"`, `"revocada"` o `"sin_emitir"` (en ese caso `emitidaEn` es `null`).
- `id` es interno: el frontend lo usa solo para llamar a la API y nunca lo muestra (tampoco handles ni UID).
- El frontend consulta `GET /api/emision/status` cada **2 s** y se rinde a los **2 minutos** (muestra un error al emisor). Deja de consultar si el emisor cierra el panel.
- Preguntas abiertas para P1:
  1. ¿`POST /api/emision/order` devuelve un id de orden para consultar `?order=<id>`?
  2. ¿La lista de ciudadanos admite paginación o búsqueda en servidor?
  3. **¿Qué terminal atiende `POST /api/emision/order`?** Si hay varios terminales en modo emisión, ¿cómo se elige uno (por emisor, por sesión, por id de terminal)? El frontend no envía ningún id de terminal.
  4. **Validación en servidor de `{ "ciudadanoId" }` en `POST /api/revoke`:** solo debe aceptarse si la sesión tiene rol `emisor`; un ciudadano que envíe ese campo debe recibir `403` (el ciudadano solo puede bloquear su propia cédula, sin cuerpo).
  5. **Confirmar que `GET /api/citizens` devuelve únicamente** `id` (interno), `nombre`, `email`, `estadoCedula` y `emitidaEn`, **sin handle ni UID**.
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

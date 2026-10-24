# Contrato de bytes de SoloLoJusto

Fuente de verdad para la API (Go), el firmware del lector (ESP32 con micro-ecc) y las PWAs (WebCrypto). Los enlaces entre lector, gateway, PIC y API están en [`contrato-terminal.md`](contrato-terminal.md).
Cualquier cambio se anuncia al equipo antes de hacerlo. Implementación de referencia:
`api/internal/sign`; vectores: `docs/test-vectors.json`; verificador independiente: `node tools/verify.mjs`.

## Reglas generales

- Multibyte en **big-endian**.
- Hash **SHA-256**. Firma **ECDSA P-256** de 64 B en formato crudo `r‖s` (IEEE P1363), cada mitad con relleno a 32 B.
- En Go se usa `ecdsa.Sign`, no `SignASN1`.

## Cabecera (16 B)

| Offset | Tamaño | Campo |
|---|---|---|
| 0 | 1 | version = `0x01` |
| 1 | 1 | emisor: `0x01` nacional, `0x02` universidad |
| 2 | 1 | atributo |
| 3 | 1 | largo = bytes de valor |
| 4 | 4 | vence (Unix, big-endian) |
| 8 | 8 | handle (aleatorio) |

## Atributos

| Código | Atributo | Valor máx. |
|---|---|---|
| `0x01` | Mayor de 18 | 16 B |
| `0x02` | Mayor de 21 | 16 B |
| `0x03` | Identidad (nombre, número V, vigencia) | 64 B |
| `0x04` | Sangre y alergias | 16 B |
| `0x05` | Contacto de emergencia | 16 B |
| `0x06` | Donante y licencia | 16 B |

## Firma en el chip

- Mensaje firmado: `UID ‖ cabecera ‖ valor[0:largo]`. **El relleno con ceros no se firma.**
- Registro en el chip: `cabecera(16) ‖ valor relleno a 16 (64 en identidad) ‖ firma(64)` = **96 B** (2 sectores) o **144 B** (3 sectores).
- Se parte en bloques de 16 B, 3 bloques de datos por sector (el cuarto, el del trailer, no se usa).

## Mapa de sectores

| Sector | Contenido |
|---|---|
| 1 | Índice: `version 0x01 ‖ handle (8 B)`, relleno a 16 B, sin firma (los otros 2 bloques en cero) |
| 2–3 | A1 Mayor de 18 |
| 4–5 | A2 Mayor de 21 |
| 6–8 | A3 Identidad |
| 9–10 | A4 Sangre y alergias |
| 11–12 | A5 Contacto de emergencia |
| 13–14 | A6 Donante y licencia |
| 15 | Reserva |

## Credencial de billetera

- Mensaje firmado: `cabecera ‖ valor[0:largo] ‖ clave pública del celular (33 B comprimida)`. **Sin UID.**
- Blob: `cabecera ‖ valor[0:largo] ‖ pubkey33 ‖ firma64`.

## QR (lo implementa P4)

```
"SLJ1:" + base64url( blob ‖ hora(4 B) ‖ firma del celular(64 B) sobre (hora ‖ atributo) ‖ atributo(1 B) )
```

Ventana de ±30 s respecto de la hora del verificador.

## Claves de los emisores

- `keys/issuers.json`: JWK (`kty`, `crv`, `x`, `y`) con llaves `"0x01"` y `"0x02"`.
- `keys/issuers.h`: `ISSUER_NACIONAL_PUB[64]` e `ISSUER_UNIVERSIDAD_PUB[64]`, `X‖Y` sin el prefijo `0x04` (formato de micro-ecc).
- Privadas: PKCS#8 DER en base64 (`secrets/<nombre>.key.b64`, fuera de git). En producción van en `ISSUER_NACIONAL_KEY` e `ISSUER_UNIVERSIDAD_KEY` con el mismo formato.

## Códigos de resultado

| Código | Significado |
|---|---|
| `01` | Aprobado |
| `02` | Firma inválida |
| `03` | Revocada |
| `04` | Vencida |
| `05` | Sin permiso |
| `06` | Rechazado por el ciudadano |

## Tablas (Postgres, `db/migrations/000001_init.up.sql`)

- **citizens**: correo, hash de contraseña, rol (`ciudadano`/`emisor`/`comercio`), nombre, teléfono de WhatsApp, avisos.
- **cards**: handle (BYTEA 8 B), UID, ciudadano, estado (`activa`/`revocada`), fechas.
- **attributes**: handle, código, emisor, valor, vencimiento; único por (handle, código).
- **devices**: ciudadano, handle, clave pública del celular (BYTEA 33 B).
- **wallet_credentials**: dispositivo, handle, código, blob, vencimiento.
- **terminals**: nombre, rol (`comercio`/`registro`/`salud`/`transito`/`emergencia`), atributo que pide, `modo` (`verificar`/`emergencia`/`emitir`/`vincular`), hash del token.
- **events**: `kind` (`verif`/`tamper`), handle, terminal, atributo, resultado (1–6), `emergency` (bool), hora. Handle, atributo y resultado son nulos en `tamper`. Retención de 30 días.
- **link_codes**: código de 6 dígitos, handle, vencimiento.

## Nota para P4 (WebCrypto)

`crypto.subtle.exportKey('raw', pub)` entrega la clave en **65 B** (`0x04 ‖ X ‖ Y`). Hay que comprimirla a 33 B:

```js
const raw = new Uint8Array(await crypto.subtle.exportKey('raw', pub)); // 65 B
const pub33 = new Uint8Array(33);
pub33[0] = (raw[64] & 1) === 0 ? 0x02 : 0x03; // paridad de Y
pub33.set(raw.subarray(1, 33), 1);            // X
```

WebCrypto firma y verifica `ECDSA` con `{ name: 'ECDSA', hash: 'SHA-256' }` directamente en `r‖s` de 64 B, sin conversión.

## API (Go)

Sesión: cookie `slj_session` (JWT HS256, HttpOnly, 7 días). Terminales: `Authorization: Bearer <token>`.
Los errores son `{"ok":false,"error":"..."}`. Binarios en hex (handle 16 caracteres, clave del celular 66 caracteres).

| Ruta | Quién | Cuerpo → respuesta |
|---|---|---|
| `POST /api/auth/register` | público | `{email,password,name,whatsapp?}` → `{ok,user}`; siempre rol ciudadano |
| `POST /api/auth/login` · `/logout` | público | `{email,password}` → `{ok,user}` |
| `GET /api/me` | sesión | `{ok,user}` |
| `GET /api/citizens` | emisor | `{citizens:[{id,email,name,profile}]}` |
| `PUT /api/citizens/{id}/profile` | emisor | perfil (abajo) → `{attributes:["A1",...]}` |
| `POST /api/emision/request` | emisor | `{citizen_id}`; queda pendiente 10 min |
| `POST /api/emision/start` | terminal registro, modo emitir | `{uid}` → `{h,blocks:[{s,b,d}]}` (índice primero) |
| `POST /api/link/start` | terminal en modo vincular | `{h}` → `{c}` (6 dígitos, 2 min) |
| `POST /api/link/confirm` | sesión | `{code,public_key}` → `{credentials:[{a,blob_hex,expires}]}`; la cédula debe ser de esa cuenta |
| `GET /api/wallet[?public_key=hex]` | sesión | `{credentials:[{a,issuer,blob_hex,expires}]}` vigentes, de cédulas activas |
| `GET /api/events[?limit=50&before=<id>]` | sesión | `{events:[{id,local,atributo,resultado,hora,emergencia}]}`, más recientes primero |
| `GET /api/events/stream` | sesión | SSE: un evento `evento` por lectura nueva, con el mismo JSON; admite `Last-Event-ID` |
| `POST /api/events` | terminal | `{t:"verif",h,a,res,ts,emg}` o `{t:"tamper",ts}` → `{ok}` |
| `GET /api/revocations` | público | `{hs:[...]}` |
| `GET /api/issuers` | público | `{"0x01":JWK,"0x02":JWK}` |
| `POST /api/revoke` | emisor o dueño | `{h}`; borra las credenciales de billetera |
| `POST /api/reactivate` | emisor o dueño | `{h}`; reactiva la cédula y vuelve a emitir las credenciales de sus celulares |
| `GET /api/cards` | sesión | `{cards:[{h,estado,emitida,revocada?,atributos,dispositivos}]}` de la cuenta |
| `POST /api/carnet/issue` | emisor | `{citizen_id}`; emite o renueva el carnet universitario en sus celulares vinculados |
| `GET /api/terminals` | emisor | `{terminals:[{id,local,rol,a,modo}]}` |
| `PATCH /api/terminals/{id}` | emisor | `{modo?,a?}`; el terminal lo lee en `/api/terminal/config` |
| `GET /api/commerce/summary[?days=7]` | comercio o emisor | totales de sus terminales, sin handles |
| `GET /api/terminal/config` | terminal | `{rol,a,local,modo}` |

Credenciales de billetera derivadas: A1, A2 y A6 (si existen y no han vencido). Identidad y datos de salud no salen del chip.

### Perfil del ciudadano y valores de los atributos

```json
{"birthdate":"2000-03-04","identity":{"name":"ANA PEREZ","id":"V-12345678","valid_until":"2030-01-01"},
 "blood":"O+ penicilina","contact":"Mama 04141234567","donor_license":"D;LIC-B2"}
```

| Atributo | Valor (UTF-8) | Se emite si |
|---|---|---|
| A1 / A2 | `0x01` (1 B) | cumple 18 / 21 años |
| A3 | `NOMBRE\|V-12345678\|AAAA-MM-DD` (≤ 64 B) | hay nombre y número |
| A4 | `blood` tal cual (≤ 16 B) | no vacío |
| A5 | `contact` tal cual (≤ 16 B) | no vacío |
| A6 | `donor_license` tal cual (≤ 16 B) | no vacío |

Todo lo firmado vence a los 365 días de la emisión.

## Avisos de WhatsApp (Kapso)

Variables: `KAPSO_API_KEY`, `KAPSO_PHONE_NUMBER_ID`, `KAPSO_WEBHOOK_SECRET`. Teléfonos guardados solo con dígitos y código de país (`0424...` se toma como Venezuela: `58424...`). El ciudadano los cambia con `PATCH /api/me` `{whatsapp, notifications}`.

| Disparador | Mensaje | A quién |
|---|---|---|
| `verif` con `res = 01` | "Tu cédula compartió 'Mayor de 18' con Bar La Esquina hoy a las 21:40. ¿No fuiste tú?" + botón **Bloquear cédula** | Dueño de la cédula, si tiene teléfono y avisos activos |
| `verif` con `res = 01` y `emg = true` | "Un terminal de emergencia (X) leyó tu tipo de sangre y contacto hoy a las 21:40. ¿No fuiste tú?" + botón | Igual |
| `tamper` | "Alerta: el terminal 'X' detectó manipulación hoy a las 21:40. Revisa el equipo." | Dueño del terminal (`terminals.owner_id`) |

Si el envío con botones falla, se manda el mismo texto con "Responde BLOQUEAR." Las lecturas con otro resultado (firma inválida, revocada, vencida, sin permiso, rechazada) no generan aviso. Las horas van en hora de Venezuela (UTC-4).

**Webhook** `POST /api/webhooks/kapso` (evento `whatsapp.message.received`): se valida `X-Webhook-Signature` (HMAC-SHA256 hex del cuerpo crudo con `KAPSO_WEBHOOK_SECRET`); sin secreto configurado responde 503, con firma mala 401. Si quien escribe envía BLOQUEAR o toca el botón (`id = bloquear`), se revocan todas las cédulas activas de la cuenta con ese teléfono, se borran sus credenciales de billetera y se responde "Tu cédula fue bloqueada. Puedes reactivarla desde el portal." Cualquier otro mensaje se ignora. `X-Idempotency-Key` evita responder dos veces ante un reintento.

### Historial del ciudadano

`GET /api/events` y `GET /api/events/stream` devuelven solo los eventos de las cédulas de la cuenta con sesión, sin handle ni UID:

```json
{"id":42,"local":"Bar La Esquina","atributo":"A1","resultado":1,"hora":"2026-10-24T21:40:03Z","emergencia":false}
```

- `atributo`: `"A1"`..`"A6"`. `resultado`: número 1–6 (ver códigos de resultado). `hora`: RFC 3339 en UTC. `local`: nombre del terminal.
- Solo lecturas (`verif`); las alarmas de manipulación no aparecen. Se incluyen todos los resultados, no solo los aprobados.
- El stream envía solo lo nuevo; el historial se pide con `GET /api/events`. Con `Last-Event-ID` recupera lo perdido. Manda un latido cada 25 s y sondea cada 10 s como respaldo del `LISTEN/NOTIFY` de Postgres (canal `slj_events`).
- Con `EventSource` basta la cookie de sesión del mismo origen.

### Carnet universitario (emisor 0x02)

No se agregó un código de atributo nuevo: el carnet es un **A3 (Identidad) firmado por la universidad** (cabecera con emisor `0x02`), solo en la billetera, nunca en el chip. Valor: `NOMBRE|N°-CARNET|AAAA-MM-DD` (≤ 64 B), igual que A3. Sale del campo `carnet` del perfil (`{name, id, valid_until}`) y se emite al vincular el celular o con `POST /api/carnet/issue`. Un celular puede tener a la vez un A3 nacional y uno universitario: se distinguen por `issuer` (`"0x01"`/`"0x02"`) y por el byte de emisor de la cabecera. El verificador debe validar con la clave que corresponda a ese byte (`/api/issuers`).

### Comercio

`GET /api/commerce/summary` devuelve para los terminales cuyo `owner_id` es la cuenta (todos, si es emisor):

```json
{"ok":true,"days":7,"totals":{"total":4,"aprobadas":3,"rechazadas":1,"manipulaciones":1},
 "por_resultado":{"1":3,"2":1},"por_atributo":{"A1":2,"A4":1,"A2":1},
 "por_dia":[{"dia":"2026-10-24","total":4,"aprobadas":3}],
 "terminales":[{"id":2,"local":"Bar La Esquina","total":4,"aprobadas":3,"rechazadas":1,"manipulaciones":1}]}
```

Nunca incluye handles ni datos de personas. Los días van en hora de Venezuela.

### Datos de demo y retención

- `go -C api run ./cmd/admin demo [-ana +58.. -luis +58.. -maria +58..]` siembra o restaura: cuentas `emisor@`, `comercio@`, `ana@`, `luis@`, `maria@slj.test` (misma contraseña) y los terminales Registro (emitir), Bar La Esquina (A1), Clinica Salud (A4), Ambulancia 1 (emergencia) y Transito (A6). Borra cédulas, credenciales, eventos y solicitudes de las cuentas demo y deja los modos en su valor inicial. La contraseña y los tokens quedan en `secrets/demo.json` y no cambian entre restauraciones.
- `cmd/retention` borra eventos de más de 30 días (`RETENTION_DAYS`), códigos vencidos y solicitudes viejas. Va como Cron Job en Render.

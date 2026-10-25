# Contrato de bytes de sIsA

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
Los errores son `{"ok":false,"error":"mensaje en español"}`. Binarios en hex (handle 16 caracteres, clave del celular 66 caracteres).
Los `id` que ve el front (usuarios, terminales, estudiantes) son **texto** (`"17"`); en los cuerpos se aceptan como texto o número.
Las respuestas del front son JSON suelto (`Citizen[]`, `Terminal[]`...), sin envoltorio, salvo `GET /api/events` (`{ok, events}`).

### Front (portal, panel del emisor, comercio)

| Ruta | Quién | Cuerpo → respuesta |
|---|---|---|
| `POST /api/auth/register` | público | `{nombre,email,password,telefono?}` → usuario; siempre rol ciudadano; 409 si el correo existe |
| `POST /api/auth/login` · `/logout` | público | `{email,password}` → usuario; 401 si falla |
| `GET /api/me` | sesión | usuario |
| `PUT /api/me/notifications` (también `PATCH /api/me`) | sesión | `{telefono:"+58…"\|null, avisosActivados}` → mismo JSON; 400 si activas avisos sin teléfono |
| `GET /api/card` | sesión | `{id, estado:"activa"\|"revocada", emitidaEn, atributos:["mayor_de_18",…]}`; 404 si no tiene cédula; revocada → `atributos: []` |
| `GET` · `PUT /api/me/optional-data` | sesión | `{tipoSangre\|null, alergias, contactoEmergencia:{nombre,telefono}, donante}`; 400 en español si no caben en el chip (abajo) |
| `GET /api/events[?limit=50&before=<id>]` | sesión | `{ok,events:[{id,local,atributo,resultado,hora,emergencia}]}`, más recientes primero |
| `GET /api/events/stream` | sesión | SSE: evento `evento` por lectura nueva, mismo JSON |
| `POST /api/revoke` | sesión | **sin cuerpo**: revoca las cédulas activas de la cuenta. El emisor manda `{ciudadanoId}`; un ciudadano que lo mande recibe 403 |
| `POST /api/reactivate` | sesión | igual que revoke; vuelve a emitir las credenciales de billetera |
| `GET /api/citizens` | emisor | `Citizen[]`: `{id,nombre,email,estadoCedula:"activa"\|"revocada"\|"sin_emitir",emitidaEn\|null,perfilCompleto}`; sin handle, UID, nacimiento ni cédula |
| `PUT /api/citizens/{id}/profile` | emisor | `{fechaNacimiento:"AAAA-MM-DD", cedula:"6–9 dígitos"}` → 200; completa el perfil (nombre de la cuenta + `V-cédula`) |
| `POST /api/emision/order` | emisor | `{ciudadanoId, terminalId?}` → `{id}`; 409 si el perfil no está completo. Con `terminalId` (de registro) la orden es de ese terminal; sin él, la toma cualquier terminal de registro en modo emitir (si ninguno lo está, se activa) |
| `GET /api/emision/status[?order=<id>]` | emisor | `{estado:"pendiente"\|"completado"\|"error", mensaje?}`; completado = bloques entregados al terminal; una orden espera 10 min |
| `GET /api/students` | emisor | `Student[]`: `{id,nombre,email,estadoCarnet:"sin_carnet"\|"emitido",emitidoEn\|null}` |
| `POST /api/credentials/university` | emisor | `{estudianteId}` → 200; **409** si ya tiene carnet; si aún no vinculó su celular queda autorizado (`pendiente:true`) y se emite al vincular |
| `GET /api/terminals` | comercio (los suyos) o emisor (todos) | `Terminal[]`: `{id,nombre,rol,atributos}` (+ `local,a,modo`); nunca el token |
| `POST /api/terminals` | comercio | `{nombre,rol,atributos}` → `Terminal` **con `token`** (única vez). 201 |
| `PUT /api/terminals/{id}` | comercio (suyos) o emisor | `{nombre?,rol?,atributos?}` → `Terminal` sin token. El modo (`modo`) solo lo cambia el emisor (también por `PATCH`) |
| `DELETE /api/terminals/{id}` | comercio (suyos) o emisor | 200; su token deja de valer |
| `GET /api/stats` | comercio o emisor | `[{terminalId,hoy,aprobadas,rechazadas}]` de hoy (hora de Venezuela) |
| `GET /api/commerce/summary[?days=7]` | comercio o emisor | totales por resultado, atributo, día y terminal; sin handles |

Usuario (`register`, `login`, `GET /api/me`, y `PUT /api/me/notifications`): los campos van en la raíz y también dentro de `user`:

```json
{"ok":true,"id":"17","email":"ana@sisa.demo","nombre":"Ana Pérez","rol":"ciudadano","telefono":"+584241234567","avisosActivados":true,
 "user":{"id":"17","email":"ana@sisa.demo","nombre":"Ana Pérez","rol":"ciudadano","telefono":"+584241234567","avisosActivados":true}}
```

`rol`: `"ciudadano"`, `"emisor"` o `"comercio"`. `telefono` es `null` si no hay.

Nombres de atributos del front: `mayor_de_18` (A1), `mayor_de_21` (A2), `identidad` (A3), `sangre_alergias` (A4), `contacto_emergencia` (A5), `donante_licencia` (A6).

**Datos opcionales y el chip.** A4 se escribe como `"<tipoSangre> <alergias>"` y A5 como `"<nombre> <teléfono>"` (un +58 414… se escribe 0414…, 11 dígitos; los demás, solo dígitos); cada uno **debe caber en 16 bytes UTF-8** (se cuentan bytes, no caracteres: `á` ocupa 2). El tipo de sangre cuenta dentro de A4, así que las alergias tienen 16 menos el tipo y un espacio (13 con `O+`). A6 es `D;` si es donante. Se escriben en el chip en la próxima emisión.

**Terminales por tipo** (`rol` del terminal): `comercio` pide uno solo de `mayor_de_18`/`mayor_de_21`; `registro`, `identidad`; `salud`, de `sangre_alergias`, `contacto_emergencia`, `donante_licencia` (por defecto los tres); `emergencia`, de `sangre_alergias`, `contacto_emergencia` (modo `emergencia`, sin consentimiento). Sin `atributos` se usan los de por defecto del tipo. El firmware lee solo el primero (`a` en `/api/terminal/config`).

**Front en otro dominio.** Si el front llama a la API directamente (`VITE_API_URL`), hay que definir `CORS_ORIGINS` con el origen exacto del front (varios, separados por coma). La API responde `Access-Control-Allow-Origin` con ese origen, `Access-Control-Allow-Credentials: true` y atiende el preflight. Si el front está en otro sitio y hay HTTPS, la cookie sale `SameSite=None; Secure`; con el mismo origen (rewrite) queda `SameSite=Lax`. Con `CORS_ORIGINS` definido, las peticiones que cambian datos desde otro origen se rechazan con 403. Algunos navegadores bloquean cookies entre sitios distintos (p. ej. dos `*.onrender.com`): lo más seguro es el rewrite de `slj-web`, o ambos bajo el mismo dominio.

### Terminales, billetera y verificadores

| Ruta | Quién | Cuerpo → respuesta |
|---|---|---|
| `POST /api/emision/start` | terminal registro, modo emitir | `{uid}` → `{h,blocks:[{s,b,d}]}` (índice primero) |
| `POST /api/link/start` | terminal en modo vincular | `{h}` → `{c}` (6 dígitos, 2 min) |
| `POST /api/link/confirm` | sesión | `{code,public_key}` → `{credentials:[{a,issuer,blob_hex,expires}]}`; la cédula debe ser de esa cuenta |
| `GET /api/wallet[?public_key=hex]` | sesión | `{credentials:[...]}` vigentes, de cédulas activas |
| `POST /api/events` | terminal | `{t:"verif",h,a,res,ts,emg}` o `{t:"tamper",ts}` → `{ok}` |
| `GET /api/revocations` | público | `{hs:[...]}` |
| `GET /api/issuers` | público | `{"0x01":JWK,"0x02":JWK}` |
| `GET /api/terminal/config` | terminal | `{rol,a,atributos,local,modo}`; `a` es el primer atributo |

Credenciales de billetera derivadas: A1, A2 y A6 (si existen y no han vencido), más el carnet universitario (A3 del emisor `0x02`).

### Perfil del ciudadano y valores de los atributos

El perfil interno (lo completa `PUT /api/citizens/{id}/profile`; también acepta esta forma completa):

```json
{"birthdate":"2000-03-04","identity":{"name":"ANA PEREZ","id":"V-12345678","valid_until":"2030-01-01"},
 "carnet":{"name":"ANA PEREZ","id":"UCV-12345678","valid_until":"2027-07-31"}}
```

| Atributo | Valor (UTF-8) | Se emite si |
|---|---|---|
| A1 / A2 | `0x01` (1 B) | cumple 18 / 21 años |
| A3 | `NOMBRE\|V-12345678\|AAAA-MM-DD` (≤ 64 B) | hay nombre y cédula |
| A4 | `O+ penicilina` (≤ 16 B) | hay tipo de sangre o alergias |
| A5 | `Mama 04141234567` (≤ 16 B) | hay contacto |
| A6 | `D;LIC-B2` (≤ 16 B) | es donante o tiene licencia |

Todo lo firmado vence a los 365 días de la emisión.

## Avisos de WhatsApp (Kapso)

Variables: `KAPSO_API_KEY`, `KAPSO_PHONE_NUMBER_ID`, `KAPSO_WEBHOOK_SECRET`. Teléfonos guardados solo con dígitos y código de país (`0424...` se toma como Venezuela: `58424...`). El ciudadano los cambia con `PATCH /api/me` `{whatsapp, notifications}`.

| Disparador | Mensaje | A quién |
|---|---|---|
| `verif` con `res = 01` | "Hola Ana: se autorizó el acceso. Tu cédula compartió 'Mayor de 18' con Bar La Esquina hoy a las 21:40. ¿No fuiste tú?" + botón **Bloquear cédula** | Dueño de la cédula, si tiene teléfono y avisos activos |
| `verif` con `res = 01` y `emg = true` | "Hola Ana: un terminal de emergencia (X) leyó tu tipo de sangre y contacto hoy a las 21:40. ¿No fuiste tú?" + botón | Igual |
| `verif` con `res = 02` | "…se negó el acceso en X … porque la firma de la tarjeta no es válida (posible copia o tarjeta alterada)." + botón | Igual |
| `verif` con `res = 03` | "…se negó el acceso en X … porque tu cédula está bloqueada. Puedes reactivarla desde el portal." (sin botón) | Igual |
| `verif` con `res = 04` | "…se negó el acceso en X … porque tu cédula está vencida." + botón | Igual |
| `verif` con `res = 05` | "…X pidió 'Sangre y alergias' …, un dato que su terminal no tiene permiso para leer. No se compartió nada." + botón | Igual |
| `verif` con `res = 06` | "…se canceló el acceso en X … porque no aceptaste compartir '…'. No se compartió nada." + botón | Igual |
| `tamper` | "Alerta: el terminal 'X' detectó manipulación hoy a las 21:40. Revisa el equipo." | Dueño del terminal (`terminals.owner_id`) |

Si el envío con botones falla, se manda el mismo texto con "Responde BLOQUEAR." Toda lectura (aprobada o denegada) avisa al dueño de la cédula; si el handle no existe, o la cuenta no tiene teléfono o tiene los avisos apagados, no se envía nada. `KAPSO_BASE_URL` (opcional) cambia la URL base de Kapso, útil para apuntar a un mock en pruebas. Las horas van en hora de Venezuela (UTC-4).

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

No se agregó un código de atributo nuevo: el carnet es un **A3 (Identidad) firmado por la universidad** (cabecera con emisor `0x02`), solo en la billetera, nunca en el chip. Valor: `NOMBRE|N°-CARNET|AAAA-MM-DD` (≤ 64 B), igual que A3. Sale del campo `carnet` del perfil (`{name, id, valid_until}`) o, si falta, se deriva de la identidad (`UCV-<cédula>`). Se emite con `POST /api/credentials/university` si el celular ya está vinculado, o al vincularlo. Un celular puede tener a la vez un A3 nacional y uno universitario: se distinguen por `issuer` (`"0x01"`/`"0x02"`) y por el byte de emisor de la cabecera. El verificador debe validar con la clave que corresponda a ese byte (`/api/issuers`).

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

- `go -C api run ./cmd/admin demo [-historial] [-ana +58.. -luis +58.. -maria +58..]` siembra o restaura (con `-historial`: cédulas de ejemplo, Ana con 3 eventos —uno de emergencia—, Luis con 1 y María con la cédula revocada): cuentas `emisor@`, `comercio@`, `ana@`, `luis@`, `maria@sisa.demo` (misma contraseña) y los terminales Registro (emitir), Bar La Esquina (A1), Clinica Salud (A4), Ambulancia 1 (emergencia) y Transito (A6). Borra cédulas, credenciales, eventos y solicitudes de las cuentas demo y deja los modos en su valor inicial. La contraseña y los tokens quedan en `secrets/demo.json` y no cambian entre restauraciones.
- `cmd/retention` borra eventos de más de 30 días (`RETENTION_DAYS`), códigos vencidos y solicitudes viejas. Se ejecuta a mano (`go -C api run ./cmd/retention`) o, si hace falta, como Cron Job en Render.

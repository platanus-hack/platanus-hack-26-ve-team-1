# 09 · Pruebas

## Estrategia

| Nivel | Qué cubre | Cómo |
|---|---|---|
| Unitario | Mensajes de aviso, firma, códec, cliente Kapso | `go -C api test ./...` |
| Integración | API contra Postgres real, avisos con notificador falso, webhook | `SLJ_INTEGRATION=1 go -C api test ./...` (con `DATABASE_URL`) |
| Cripto cruzada | Go ↔ Node ↔ WebCrypto | `node tools/verify.mjs`, pruebas de conformidad de `web/tests/slj` |
| Front | Núcleo del QR, lint, build | `npm ci && npm run lint && npm test && npm run build` en `web/` |
| Firmware | Compilación | `pio run -d firmware/esp32`, `pio run -d firmware/rc522-diag` |
| E2E simulado | Terminal → API → mock de Kapso | Ver abajo |
| Hardware | Lectura real y WhatsApp real | Checklist |

## Base local para integración

```bash
docker run -d --name slj-pg -e POSTGRES_PASSWORD=slj -p 5432:5432 postgres:16
```

Aplicar las migraciones en orden y definir `DATABASE_URL=postgres://postgres:slj@localhost:5432/postgres?sslmode=disable` en `.env.local`.

## Matriz del flujo de avisos

| # | Caso | Entrada al API | Esperado |
|---|---|---|---|
| 1 | Ana, bar, A1 | `verif` `01` | WhatsApp "se autorizó" con botón; evento en historial |
| 2 | Tarjeta alterada | `verif` `02` | WhatsApp de firma inválida con botón |
| 3 | María revocada | `verif` `03` | WhatsApp "bloqueada" sin botón |
| 4 | Credencial vencida | `verif` `04` | WhatsApp "vencida" con botón |
| 5 | Bar pide salud (A4) | `verif` `05` | WhatsApp "no se compartió nada" |
| 6 | Rechazo con cédula identificada | `verif` `06` | WhatsApp "no aceptaste compartir" (el firmware hoy no lo envía sin handle) |
| 7 | Emergencia | `verif` `01` `emg` | WhatsApp de emergencia con botón |
| 8 | Avisos apagados / sin teléfono / handle desconocido | `verif` cualquiera | Sin mensaje; evento guardado |
| 9 | Botones no soportados | `verif` `01` | Texto con "Responde BLOQUEAR." |
| 10 | BLOQUEAR por webhook | Mensaje firmado | Cédula `revocada`; confirmación; idempotencia ante reintento |
| 11 | Webhook sin firma o firma falsa | — | 401 |
| 12 | Tamper | `tamper` | Alerta al dueño del terminal |

Los casos 1–12 están automatizados en `notify_test.go` y `kapso_integration_test.go`.

## Matriz del consentimiento por WhatsApp

| # | Caso | Entrada | Esperado |
|---|---|---|---|
| 13 | Solicitud | `POST /api/consent` `{h, a:"A1"}` | 200 `pendiente`; WhatsApp con botones al teléfono de Ana |
| 14 | Aprobar | Webhook firmado `consent:<id>:si` | `GET` da `aprobado`; Ana recibe "Listo: compartiste…" |
| 15 | Rechazar | Webhook con botón `no` o texto "NO" | `GET` da `rechazado`; "No se compartió nada." |
| 16 | Segunda respuesta | `si` y luego `no` | La primera gana; el estado no cambia |
| 17 | Teléfono ajeno | Respuesta desde otro número | Se ignora; sigue `pendiente` |
| 18 | Vencida | `expires_at` pasado | `GET` da `vencido`; respuesta tardía ignorada |
| 19 | Sin WhatsApp | Ciudadano sin teléfono o avisos apagados | 422 `sin_whatsapp`; no se envía nada |
| 20 | Cédula desconocida / revocada | `POST /api/consent` | 404 / 409 |
| 21 | Otro terminal | `GET /api/consent/{id}` con token ajeno | 404 |
| 22 | Aviso duplicado | `verif` `01` o `06` tras una solicitud respondida | Sin segundo WhatsApp |
| 23 | Tiempo agotado | `verif` `06` con la solicitud vencida | "La solicitud de {local} venció y no se compartió nada." |
| 24 | Reconocer la respuesta | `Incoming.Consent()` con botón y texto Sí/No, y con BLOQUEAR | Distingue consentimiento de bloqueo |

Los casos 13–24 están automatizados en `notify_test.go`, `kapso_test.go` y `kapso_integration_test.go`.

## E2E simulado con mock de Kapso

1. Levantar un servidor HTTP local que registre cada `POST /meta/whatsapp/v24.0/<id>/messages`.
2. Arrancar la API con `KAPSO_BASE_URL=http://localhost:<puerto>`, `KAPSO_API_KEY` y `KAPSO_PHONE_NUMBER_ID` ficticios.
3. Sembrar `go -C api run ./cmd/admin demo -historial` y usar el token de "Bar La Esquina" (`secrets/demo.json`).
4. Enviar los eventos de la matriz con `curl` o `mega-sim` y comprobar los cuerpos recibidos por el mock, `GET /api/events` y el SSE.

## Checklist con hardware (lo ejecuta el equipo)

- [x] Cédulas de demo emitidas en Render: Ana (`DB3A215CECA2B951`, A1–A6) y María (`C5C288ACD5145568`, sin A1/A2 por ser menor).
- [x] `scan` lee UID y mapa de sectores de las dos cédulas.
- [x] Consentimiento simulado en placa (`a`/`r`): `01` → "Esperando autorización" en el UNO → verde o rojo.
- [ ] ESP32 cargado, REARME del PIC, sin `tamper` al arrancar.
- [ ] Antes de empezar: Ana escribe "hola" al número de Kapso (ventana de 24 h para los botones) y el webhook apunta a `/api/webhooks/kapso`.
- [ ] Ana en el bar → "Esperando autorización" → WhatsApp → **Sí** → aprobado en el UNO.
- [ ] Tras aprobar, Ana recibe la pregunta y la confirmación, sin el aviso "se autorizó el acceso" duplicado.
- [ ] Sin WhatsApp o sin internet: el gateway avisa al operador y `si`/`no` por consola resuelven el pedido.
- [ ] Ana en el bar → WhatsApp → **No** → `06`; sin respuesta en 60 s → `06`.
- [ ] María en el bar → `05` (su cédula no tiene el dato "Mayor de 18").
- [ ] Cédula revocada → `03` y WhatsApp sin botón.
- [ ] Tarjeta alterada → `02` y WhatsApp.
- [ ] Bar con atributo de salud → `05`.
- [ ] Terminal de emergencia → lectura sin consentimiento y aviso.
- [ ] Responder BLOQUEAR → siguiente intento da `03`.
- [ ] Abrir la carcasa → bloqueo, UNO en rojo y alerta.
- [ ] Verificador en modo avión valida el QR de Ana.

## Resultados

Corrida sobre la rama `integracion` (main + p1-backend + p2-esp32 + pic_guardian):

| Nivel | Resultado |
|---|---|
| `go vet` y `go test ./...` | OK |
| `SLJ_INTEGRATION=1 go test ./...` con Postgres 16 en Docker y las 7 migraciones | OK (incluye avisos 01–05, emergencia, respaldo en texto, tamper, webhook y consentimiento por WhatsApp) |
| `node tools/verify.mjs` | Todo correcto |
| Front: `npm run lint` y `npm run build` | OK |
| Firmware: `pio run` | OK (ESP32, en la máquina de old-lx) |
| Gateway: `go vet` y `go test ./...` | OK, también con `-race`; incluye el consentimiento: aprobado, rechazado, vencido, sin respuesta, cédula revocada, sin WhatsApp, API caída y pedido nuevo |
| Consentimiento con la API real (2026-10-25) | Lector → gateway → API OK: la solicitud se crea y vence a los 60 s con `06`. El WhatsApp no llega al teléfono: pendiente en Kapso |
| E2E con mock de Kapso | Cubierto por el notificador falso de la prueba de integración; no se corrió un mock HTTP aparte |
| Hardware y Kapso real | Pendiente (checklist) |

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

## E2E simulado con mock de Kapso

1. Levantar un servidor HTTP local que registre cada `POST /meta/whatsapp/v24.0/<id>/messages`.
2. Arrancar la API con `KAPSO_BASE_URL=http://localhost:<puerto>`, `KAPSO_API_KEY` y `KAPSO_PHONE_NUMBER_ID` ficticios.
3. Sembrar `go -C api run ./cmd/admin demo -historial` y usar el token de "Bar La Esquina" (`secrets/demo.json`).
4. Enviar los eventos de la matriz con `curl` o `mega-sim` y comprobar los cuerpos recibidos por el mock, `GET /api/events` y el SSE.

## Checklist con hardware (lo ejecuta el equipo)

- [ ] ESP32 grabado, REARME del PIC, `[PIC] BLOQ en BAJO`.
- [ ] `scan` lee UID y mapa de sectores de la cédula de Ana.
- [ ] Gateway conectado, hora enviada, Aceptar, cédula de Ana → "acceso concedido" y WhatsApp real.
- [ ] Cédula de Luis (menor) → denegado y WhatsApp a Luis.
- [ ] Cédula de María (revocada) → `03` y WhatsApp sin botón.
- [ ] Tarjeta alterada → `02` y WhatsApp.
- [ ] Bar con atributo de salud → `05`.
- [ ] Rechazar en la Nextion → `06` en pantalla y UNO; sin WhatsApp (no hay handle antes de acercar la cédula).
- [ ] Terminal de emergencia → lectura sin consentimiento y aviso.
- [ ] Responder BLOQUEAR → siguiente intento da `03`.
- [ ] Abrir la carcasa → bloqueo, voz y alerta.
- [ ] Verificador en modo avión valida el QR de Ana.

## Resultados

Corrida sobre la rama `integracion` (main + p1-backend + p2-esp32 + pic_guardian):

| Nivel | Resultado |
|---|---|
| `go vet` y `go test ./...` | OK |
| `SLJ_INTEGRATION=1 go test ./...` con Postgres 16 en Docker y las 6 migraciones | OK (incluye avisos 01–05, emergencia, respaldo en texto, tamper y webhook) |
| `node tools/verify.mjs` | Todo correcto |
| Front: `npm run lint` y `npm run build` | OK |
| Firmware: `pio run` | No ejecutado: `pio` no está instalado en esta máquina |
| E2E con mock de Kapso | Cubierto por el notificador falso de la prueba de integración; no se corrió un mock HTTP aparte |
| Hardware y Kapso real | Pendiente (checklist) |

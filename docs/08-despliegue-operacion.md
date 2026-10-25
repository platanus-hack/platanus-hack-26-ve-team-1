# 08 · Despliegue y operación

## Variables de entorno de la API

| Variable | Obligatoria | Uso |
|---|---|---|
| `DATABASE_URL` | Sí | Postgres. Sin ella solo responde `/api/health` |
| `JWT_SECRET` | Sí | Firma de sesión (Render lo genera) |
| `ISSUER_NACIONAL_KEY`, `ISSUER_UNIVERSIDAD_KEY` | Sí para emitir | PKCS#8 DER en base64 |
| `KAPSO_API_KEY`, `KAPSO_PHONE_NUMBER_ID` | Para avisos | Sin ellas no se envían WhatsApp |
| `KAPSO_WEBHOOK_SECRET` | Para BLOQUEAR | Sin él el webhook responde 503 |
| `KAPSO_BASE_URL` | No | Otra URL base de Kapso (mock en pruebas) |
| `CORS_ORIGINS` | Si el front usa otro dominio | Orígenes exactos separados por coma |
| `PORT` | No | 8080 por defecto |
| `RETENTION_DAYS` | No | Para `cmd/retention` (30) |

Plantilla: `.env.example`. Nunca se versionan `.env.local` ni `secrets/`.

## Render

1. Crear la base Postgres y ejecutar `db/migrations/*.up.sql` en orden (golang-migrate o `psql`).
2. Conectar el repositorio personal espejo (Render no accede a repos de la organización; ver README).
3. Aplicar `render.yaml` (`slj-api`, `slj-web`). Confirmar que el `destination` del rewrite `/api/*` apunta a la URL real de la API.
4. Cargar las variables de la tabla anterior.
5. En Kapso, apuntar el webhook `whatsapp.message.received` a `https://<api>/api/webhooks/kapso` con el mismo secreto.

## Claves y datos de demo

```bash
go -C api run ./cmd/keygen
go -C api run ./cmd/admin demo -historial -ana +58414XXXXXXX -luis +58414XXXXXXX -maria +58414XXXXXXX
```

`demo` crea las cuentas `emisor@`, `comercio@`, `ana@`, `luis@`, `maria@sisa.demo` y los terminales (Registro, Bar La Esquina, Clínica Salud, Ambulancia 1, Tránsito); tokens y contraseña quedan en `secrets/demo.json`. Volver a ejecutarlo restaura el estado inicial antes de cada ensayo.

## Gateway y terminal

1. Poner en `gateway/config.local.json` el puerto del ESP32, el terminal activo y la ruta a `secrets/demo.json` (no se versiona; modelo en `gateway/config.example.json`).
2. Conectar el ESP32 por USB; cerrar el monitor serie (el gateway necesita el puerto).
3. Tras cada grabación del ESP32, **pulsar REARME en el PIC** (el latido se corta más de 10 s al grabar).
4. Cable del GPIO25 del ESP32 al **pin 0** del UNO, con GND común. Para cargar firmware al UNO hay que soltarlo.
5. Arrancar el gateway (`go -C gateway run ./cmd/gateway`). En su consola, `terminal <nombre>` cambia de terminal (por ejemplo `terminal Registro` para emitir).
6. Si el lector no responde: comando `scan` en el monitor serie del ESP32 (UID, tipo de tarjeta y clave de cada sector) y después [`firmware/rc522-diag/README.md`](../firmware/rc522-diag/README.md).

### Emitir una cédula

1. Desde el panel del emisor, ordenar la emisión del ciudadano en el terminal "Registro" (vale 10 minutos).
2. En el gateway, `terminal Registro`: el lector pasa a modo emitir.
3. Apoyar una tarjeta virgen y dejarla quieta hasta que el gateway diga "Cédula emitida y sellada".
4. Comprobarla con `scan`: el sector 1 y los del atributo deben decir `R`.

### Pantalla del comercio (UNO)

PlatformIO puede colgarse al compilar el UNO en Windows (el antivirus congela los procesos de SCons). En ese caso, compilar con `avr-gcc` directamente y cargar el `.hex` con el `avrdude` de PlatformIO (`~/.platformio/packages/tool-avrdude`) en el puerto del UNO.

## Runbook de la demo

| Situación | Acción |
|---|---|
| No llega el WhatsApp | Revisar `KAPSO_*`, que la cuenta tenga teléfono y avisos activos, y el historial del portal (muestra lo mismo); usar el video de respaldo |
| El terminal no lee | Hora enviada por el gateway (el lector no verifica sin ella), tarjeta apoyada y quieta, comando `scan`; luego diagnóstico RC522 |
| No llega la pregunta de WhatsApp | Desde el teléfono del ciudadano, escribir "hola" al número de Kapso (ventana de 24 h); plan B: responder con `a`/`r` en el monitor serie del ESP32 (con el gateway cerrado) |
| El UNO no cambia | Cable del GPIO25 en el pin 0 del UNO, GND común y monitor serie del UNO cerrado |
| Terminal bloqueado | Revisar precinto (RA5 a GND) y pulsar REARME |
| Sin internet en el evento | Compartir datos desde un celular; terminal y verificador funcionan sin red |
| Estado sucio entre ensayos | `go -C api run ./cmd/admin demo -historial` |
| API dormida (plan gratuito) | Abrir `/api/health` un minuto antes |

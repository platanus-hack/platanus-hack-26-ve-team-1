# 07 · Seguridad y privacidad

## Principios

1. **Sabe más, muestra menos:** el chip guarda varios atributos, pero cada terminal solo puede abrir los sectores de su rol y solo pide uno.
2. **Minimización:** el comercio recibe un código de resultado, nunca datos personales; los eventos usan un handle seudónimo.
3. **Transparencia hacia el ciudadano:** cada lectura, aprobada o denegada, llega por WhatsApp y al historial.
4. **Control del ciudadano:** consentimiento por WhatsApp antes de entregar cada dato (salvo emergencia, que siempre notifica), bloqueo con un toque. El lector lee y verifica la cédula antes de preguntar, pero no muestra ni registra un `01` hasta tener la respuesta; sin respuesta en 60 s, `06`.

## Control de acceso por sector

La clave A de cada sector decide qué rol lo lee; la clave B (solo el emisor) permite escribir. Un terminal de bar no puede leer salud aunque lo intente: obtiene `05`.

| Sectores | Dato | Lee |
|---|---|---|
| 1 | Índice (versión y handle) | Todos |
| 2–3 / 4–5 | A1 / A2 | Comercio |
| 6–8 | A3 Identidad | Registro |
| 9–10 / 11–12 | A4 / A5 | Salud, emergencia |
| 13–14 | A6 | Salud, tránsito |

## Modelo de amenazas (STRIDE)

| Amenaza | Escenario | Control | Resultado |
|---|---|---|---|
| Spoofing | Tarjeta clonada o copiada a otra | UID dentro de la firma; MIFARE clonable es una limitación declarada del prototipo; la billetera usa desafío firmado cada ≤30 s | `02`; aviso al dueño |
| Tampering | Alterar bytes de la tarjeta | Firma ECDSA del emisor | `02` |
| Tampering | Abrir la carcasa del terminal | PIC corta el lector; `tamper` y alerta | Terminal bloqueado |
| Repudiation | "Yo no usé mi cédula" | Historial y avisos con hora de Venezuela | Evidencia para el ciudadano |
| Information disclosure | Terminal pide más datos de los que debe | Claves por sector, `05`, aviso "no se compartió nada" | Sin fuga |
| Information disclosure | Un ciudadano consulta datos de otro | Todas las consultas filtran por la sesión; pruebas con dos cuentas | 403/vacío |
| Information disclosure | Rastreo por el servidor | Handle seudónimo, retención 30 días; E2E cifrado en la hoja de ruta | Riesgo residual declarado |
| Spoofing | Responder el consentimiento de otra persona | La respuesta solo cuenta si viene del teléfono registrado de la solicitud; la primera respuesta gana; vence a los 60 s | Se ignora |
| Information disclosure | Consultar solicitudes de otro comercio | `GET /api/consent/{id}` filtra por el terminal autenticado | 404 |
| Denial of service | Webhook falso de Kapso | Firma HMAC-SHA256 e idempotencia | 401 |
| Elevation of privilege | Token de terminal robado | Solo hashes en la base; revocable borrando el terminal | Acceso limitado a `events` y config |
| Elevation of privilege | CSRF desde otro origen | `CORS_ORIGINS` exacto, rechazo 403 de mutaciones foráneas, cookie `SameSite` | Bloqueado |
| Replay | Reusar un QR capturado | Ventana de ±30 s y anti-repetición por atributo | Rechazado |

## Secretos

Claves privadas de los emisores solo como variables de entorno (`ISSUER_NACIONAL_KEY`, `ISSUER_UNIVERSIDAD_KEY`); `secrets/`, `.env*` y `*.key` están en `.gitignore`. En el repositorio solo hay claves públicas (`keys/`). Las claves de Kapso y el secreto del webhook viven en Render.

## Limitaciones conocidas (hoja de ruta)

Chip seguro real con autenticación activa, biometría y foto contra préstamo de cédula, historial cifrado de extremo a extremo, presentaciones no vinculables, registro público de terminales.

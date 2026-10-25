# Pendientes para la demo

Estado al 2026-10-25 en la rama `integracion`. Cada tarea dice quién la toma, qué hay que hacer y cómo saber que quedó lista. Cuando termines una, márcala con `[x]` y haz commit.

**Responsables:** Samuel (API, Kapso, PWA) · koprolito (documentación y pruebas) · Manuela / mozarelly (firmware del ESP32) · Isabella (web) · old-lx (PIC, UNO, gateway, integración del terminal). Si alguien toma otra tarea, cambia el nombre.

## 0. Ya funciona

- **Lector ESP32:** lee y verifica la firma ECDSA en ~115 ms, con lista de revocación en NVS, modos verificar, emergencia, emitir y vincular, y el comando `scan` para diagnosticar tarjetas.
- **Consentimiento después de leer:** un `01` espera la respuesta del ciudadano; `a`/`r` en la consola, o `consent` desde el gateway. Probado en placa con los vectores.
- **UNO:** muestra espera, resultado, "Esperando autorización" y bloqueo. Probado con el ESP32 por el pin 0.
- **Gateway en Go:** hora, revocaciones, configuración, cola de eventos en disco, emisión, vinculación, consentimiento por WhatsApp (`pedido` → API → `consent`, con plan B `si`/`no`) y consola para cambiar de terminal. Pruebas OK.
- **API:** pruebas OK, incluidas las de integración con Postgres según `09-pruebas.md`. Avisos de WhatsApp en cada lectura y BLOQUEAR por webhook.
- **Web y PWA:** lint y build OK.
- **La Nextion se eliminó** del código y la documentación.
- **Cédulas de demo emitidas en Render y comprobadas con `scan`:** Ana Pérez (UID `A5:11:69:6A`, handle `DB3A215CECA2B951`, A1–A6) y María López (UID `E3:91:BF:33`, handle `C5C288ACD5145568`, sin A1 ni A2 por ser menor: en el bar da `05`).
- **Corregido el sellado de tarjetas:** la comprobación del trailer fallaba y dejaba los sectores sin sellar (commit `763f052`).

**Fuera de la demo:** voz (no hay audios) y código de vinculación de la billetera (el terminal no tiene dónde mostrarlo). El firmware y el gateway conservan el modo vincular y el módulo de voz, pero no se usan.

## 1. Camino crítico

Sin estas tareas no hay demo. En orden:

| # | Tarea | Quién | Depende de |
|-|-|-|-|
| 1 | Alimentación del RC522 a través del Q1 (sección 2.1) | old-lx | — |
| 2 | Consentimiento en la API y Kapso ([`consentimiento-whatsapp.md`](consentimiento-whatsapp.md)) | Samuel | — |
| 3 | ~~`pedido`/`consent` en el gateway~~ Hecho (rama `gateway-consentimiento`) | old-lx | — |
| 4 | API desplegada en Render con la migración 000007 y el webhook de Kapso apuntando a ella | Samuel | 2 |
| 5 | ~~Emitir las tarjetas de demo~~ Hecho: Ana y María (sección 3.2) | old-lx | — |
| 6 | Checklist con hardware completo (sección 7) | todos | 1–5 |
| 7 | PR de `integracion` a `main` (sección 8) | Samuel o koprolito | 6 |

## 2. Hardware (old-lx)

### 2.1 Corte de alimentación del RC522 con el Q1

Hoy el RC522 funciona conectado directo al 3V3 del ESP32, pero **no respondía a través del Q1**. Causa más probable: en esas pruebas el PIC estaba **bloqueando** (el ESP32 leía `BLOQ` = 1), y bloqueado pone `HAB_LECTOR` en BAJO, que apaga el RC522 a propósito. Con el PIC a 5 V (como indica `pines-pic.md`) y desbloqueado, el S8050 se satura y el RC522 debería recibir ~3,1 V.

- [ ] Con el Q1 en el circuito: pulsar REARME y confirmar `BLOQ` = 0 (el ESP32 arranca sin `{"t":"tamper"}`). Recién entonces probar el lector con `scan`.
- [ ] Medir el pin 3.3V del RC522 en los dos estados: desbloqueado ~3,1 V; bloqueado ~0 V. Por debajo de 2,9 V el lector falla: revisar el orden de las patas del S8050 (EBC o ECB según el fabricante) y que la base reciba 5 V a través de 1 kΩ.
- [ ] Agregar 100 nF + 10 µF entre 3.3V y GND del RC522, pegados al módulo.
- [ ] **Firmware:** al desbloquear, `onTamperCleared()` llama a `nfcReinit()` en el mismo instante en que el Q1 le devuelve la alimentación al RC522; si el chip aún no arrancó, queda marcado "no detectado" hasta reiniciar el ESP32. Agregar en `nfcReinit()` unos reintentos (por ejemplo 5 × 20 ms) hasta leer una versión válida.
- [ ] **Plan B si no queda antes de la demo:** RC522 directo al 3V3. El guardián igual bloquea el terminal (el UNO en rojo, el lector deja de leer y suelta los pines SPI), solo que sin cortar la alimentación. Decirlo así en la presentación.

**Listo cuando:** el lector lee tarjetas con el Q1 en el circuito y, al abrir el precinto, el RC522 queda sin alimentación.

### 2.2 Guardián PIC

- [ ] Latido: con el ESP32 corriendo, `BLOQ` queda en 0 (en el arranque del ESP32 no aparece `{"t":"tamper"}`).
- [ ] Desconectar el cable del GPIO26 → el PIC bloquea en 10–11 s → el UNO se pone rojo.
- [ ] Abrir el precinto (RA5) → bloqueo inmediato y `tamper` → WhatsApp al dueño del terminal.
- [ ] REARME vuelve al estado normal. Recordar: después de **cada** carga de firmware al ESP32 hay que pulsar REARME.

### 2.3 Cableado y montaje

- [ ] Cable GPIO25 del ESP32 → **pin 0** del UNO (y GND común si no comparten USB). Para cargar firmware al UNO hay que soltarlo.
- [ ] Carcasa: lector accesible, UNO visible para el comercio, precinto del PIC atravesando la tapa.
- [ ] Alimentación: un hub USB con fuente propia, o cables cortos y buenos. Probar 10 minutos seguidos sin reinicios (`Brownout`).

## 3. Firmware ESP32 (Manuela y old-lx)

### 3.1 Probar con tarjetas físicas

- [ ] Con el gateway como "Bar La Esquina": Ana → "Esperando autorización" → `a` → verde; repetir con `r` → rojo.
- [ ] Esperar 60 s sin responder → `06`.
- [ ] María → `05` de inmediato, sin preguntar.
- [ ] Bar pidiendo A4 → `05` de inmediato.

### 3.2 Tarjetas de demo

- [x] Ana Pérez: UID `A5:11:69:6A`, handle `DB3A215CECA2B951`, A1–A6, sellada.
- [x] María López: UID `E3:91:BF:33`, handle `C5C288ACD5145568`, identidad, sangre y contacto, sellada.
- [ ] Si se quiere mostrar una cédula revocada (`03`): revocar una de ellas desde el portal o con BLOQUEAR, o emitir una tercera. Hoy la de María está **activa**.
- [ ] Limpieza opcional: Ana tiene en la base una cédula activa de más (`C12D5ADD4598A1E9`, primer intento sin tarjeta física); se puede revocar desde el panel del emisor.
- [ ] Anotar los handles en `docs/tarjetas-prueba.json` si se usa ese archivo.

### 3.3 Decisiones abiertas (el equipo decide, después se implementa)

| Tema | Hoy | Propuesta |
|-|-|-|
| **Lectura desde un teléfono** | No implementada | Ver sección 3.4 |
| **`scan` lee los datos de los atributos** | Muestra los valores en la consola | Para la demo, dejar solo UID y mapa de sectores, o no usarlo delante del jurado |

### 3.4 Lectura desde un teléfono (si se mantiene en la demo)

Idea: un teléfono que toca la cédula muestra nombre y número de cédula, que ya están impresos en la tarjeta. Para eso la tarjeta necesita un registro **NDEF**:

- [ ] **API (Samuel):** en `POST /api/emision/start`, agregar los bloques del MAD (sector 0, bloques 1 y 2) y del NDEF (sector 15) con un registro de texto "Nombre · V-12.345.678".
- [ ] **Firmware (Manuela/old-lx):** `onGatewayWriteBlock` hoy acepta solo los sectores 1–14. Permitir el sector 0 (bloques 1 y 2, nunca el 0) y el 15, y grabar sus trailers con las claves públicas de NDEF (`A0A1A2A3A4A5` para el MAD, `D3F7D3F7D3F7` para el NDEF), en vez de las claves por rol.
- [ ] Probarlo antes con el teléfono de la demo. **Solo lo leen los Android con chip NFC de NXP; los iPhone no leen MIFARE Classic.**

Si no hay tiempo, mejor sacarlo de la demo que mostrarlo a medias.

### 3.5 Cierre del firmware

- [ ] `git tag demo-esp32` en el commit que se usa en la demo.

## 4. API y WhatsApp (Samuel)

- [ ] Todo lo de [`consentimiento-whatsapp.md`](consentimiento-whatsapp.md): migración, `POST /api/consent`, `GET /api/consent/{id}`, webhook, mensaje y pruebas.
- [ ] Evitar el aviso duplicado tras un consentimiento (sección 4.5 de esa guía).
- [x] La tarea de retención (`cmd/retention`) también borra `consent_requests` viejas.
- [ ] Render: aplicar la migración 000007, revisar `KAPSO_API_KEY`, `KAPSO_PHONE_NUMBER_ID` y `KAPSO_WEBHOOK_SECRET`, y el webhook de Kapso apuntando a `https://<api>/api/webhooks/kapso`.
- [ ] `admin demo` con los teléfonos reales del equipo (`-ana`, `-luis`, `-maria`).
- [ ] Enviar un WhatsApp de prueba a cada teléfono de demo.

## 5. Gateway (old-lx)

- [x] `pedido` → API → `consent` al lector, comandos de consola `si`/`no` y pruebas (sección 6 de la guía de WhatsApp). Probado con la API real: la solicitud se crea y el gateway consulta hasta que vence.
- [ ] **El WhatsApp de consentimiento no llega** (2026-10-25): la API acepta la solicitud, pero el mensaje no llega al teléfono de Ana (…7407), ni con la ventana de 24 h abierta. Revisar en el panel de Kapso el estado del mensaje y su código de error (Samuel).
- [ ] `gateway/config.local.json` con la URL de Render y los tokens de `secrets/demo.json`.
- [ ] Probar de punta a punta: lector → gateway → API → WhatsApp → historial en la web.
- [ ] Dejar compilado `gateway.exe` en la laptop de la demo (no se versiona).

## 6. Web y PWA (Isabella y Samuel)

- [x] Historial del portal: un `06` se lee "No compartiste".
- [ ] Opcional: mostrar en vivo, en el historial, la solicitud pendiente.
- [ ] Verificador en modo avión valida el QR de Ana.

## 7. Checklist con hardware (todos, antes de la demo)

Es el de `docs/09-pruebas.md`, ajustado al flujo nuevo. Se hace con el gateway y la API reales.

- [ ] ESP32 cargado, REARME del PIC, sin `tamper` al arrancar.
- [x] `scan` lee el UID y el mapa de sectores de las cédulas de Ana y María.
- [ ] Ana en el bar → "Esperando autorización" en el UNO → WhatsApp → **Sí** → "Aprobado".
- [ ] Ana en el bar → WhatsApp → **No** → `06` en el UNO.
- [ ] Ana no responde en 60 s → `06`.
- [ ] María en el bar → `05`, sin pregunta, con WhatsApp a María.
- [ ] Cédula revocada → `03`, sin pregunta.
- [ ] Bar pidiendo salud → `05`.
- [ ] Terminal de emergencia → lectura sin consentimiento y aviso.
- [ ] Responder BLOQUEAR → el siguiente intento da `03`.
- [ ] Abrir la carcasa → bloqueo, UNO en rojo y alerta por WhatsApp.
- [ ] Verificador en modo avión valida el QR de Ana.

## 8. Git y entrega

- [ ] **`main` está 35 commits atrás de `integracion`.** Si el jurado revisa `main`, hoy no ve el terminal. Abrir el PR de `integracion` a `main` cuando el checklist pase (o antes, si se acerca la entrega).
- [ ] Revisar que no haya secretos: `secrets/`, `gateway/config.local.json` y `.env` están en `.gitignore`.
- [ ] Regenerar los Word de `docs/word/` con `python tools/docs/build_docs.py`: se generaron antes de quitar la Nextion.

## 9. Documentación pendiente (koprolito y cada responsable)

Ya actualizado (2026-10-25): el terminal sin Nextion, sin voz y sin código de vinculación en la demo; el gateway en Go; el consentimiento por WhatsApp después de leer, en `project-description.md`, `01`, `02`, `03`, `04`, `05`, `07`, `08` (cómo levantar el terminal, emitir una cédula y compilar el UNO) y `09`.

| Archivo | Qué falta |
|-|-|
| `docs/contrato-terminal.md`, enlace 4 | `POST /api/consent` y `GET /api/consent/{id}`, cuando existan en la API |
| `docs/09-pruebas.md` | Las pruebas del consentimiento en la API (sección 4.6 de la guía de WhatsApp) y sus resultados |
| `docs/roadmap.md`, `docs/contexto_del_proyecto.md` | Siguen nombrando la Nextion, la voz y el Arduino Mega. Son documentos históricos: marcarlos así al inicio, o quitarlos |
| `docs/diagramas/` y `docs/word/` | Regenerarlos con `python tools/docs/build_docs.py`: se hicieron antes de estos cambios |

## 10. El día de la demo

1. Una hora antes: abrir `/api/health` para despertar la API (plan gratuito de Render).
2. `admin demo -historial` para dejar los datos limpios.
3. Desde el teléfono de Ana, escribir "hola" al número de Kapso.
4. Conectar el ESP32, pulsar REARME en el PIC y arrancar el gateway: debe mostrar "Terminal activo: Bar La Esquina".
5. Una lectura de prueba con Ana antes de que entre el jurado.
6. Si falla internet: compartir datos desde un celular. Si falla WhatsApp: desactivar los avisos de la cuenta de Ana en el portal y responder `si`/`no` en el gateway. Si falla todo: el video de respaldo.

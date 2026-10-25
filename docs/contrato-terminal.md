# Contrato técnico del terminal

Cómo se comunican las piezas. Si cambias algo de aquí, actualiza este archivo y avisa a los dueños de ambos lados.

**Estado:** borrador. Lo marcado *(confirmar)* espera el visto bueno del responsable indicado. El enlace 4 (gateway ↔ API) está confirmado por Samuel.

Los bytes de la cédula, la firma y el QR están en [`contrato.md`](contrato.md).

```
[Backend / API] <── HTTPS ──> [Gateway (laptop)] <── USB serial ──> [Lector ESP32]
                                                                       │     │
                                     [Pantalla comercio (UNO)] <── TX ─┘     └── HB / BLOQ ──> [PIC guardián]
```

| Enlace | Lado A | Lado B |
|-|-|-|
| 1. Lector ↔ gateway | Manuela | old-lx |
| 2. Lector ↔ PIC | Manuela | old-lx |
| 3. Lector → pantalla del comercio | Manuela | old-lx |
| 4. Gateway ↔ API | old-lx | Samuel |

## Códigos compartidos

**Resultado (`res`):**

| Código | Significado |
|-|-|
| `01` | Aprobado |
| `02` | Firma inválida |
| `03` | Cédula revocada |
| `04` | Cédula vencida |
| `05` | El terminal no tiene permiso para ese dato |
| `06` | Rechazado por el ciudadano |

Una lectura de emergencia aprobada es `res = "01"` con `emg = true`.

**Atributo (`a`):** `A1` Mayor de 18 · `A2` Mayor de 21 · `A3` Identidad · `A4` Sangre y alergias · `A5` Contacto de emergencia · `A6` Donante y licencia.

**Formatos:** `h` (handle) = 16 caracteres hex. `ts` = hora Unix en segundos. Todos los códigos van como string.

## 1. Lector ↔ gateway (serial USB)

115200 baudios, **una línea JSON por mensaje** terminada en `\n`. Quien recibe hace `trim()` e ignora las líneas que no sean JSON válido. Mensajes del gateway: **máximo 120 bytes** por línea (con `\n`); el lector necesita un búfer de recepción de 128 B. El peor caso, `rev` con 2 handles y `more:false`, mide 71 B.

**Lector → gateway:**

| Mensaje | Cuándo |
|-|-|
| `{"t":"hb"}` | Cada 5 s. Mientras no tenga hora (recién arrancado) envía `{"t":"hb","sync":false}` y el gateway responde con `time`, `rev` y `cfg` |
| `{"t":"verif","h":"…","a":"A1","res":"01","ts":1761300000,"emg":false}` | Tras cada lectura |
| `{"t":"pedido","h":"…","a":"A1"}` | Modo verificar: la lectura dio `01` y espera la autorización del ciudadano. El gateway se la pide por WhatsApp y responde `consent`. El lector no muestra ni informa el `01` hasta entonces |
| `{"t":"tamper"}` | `BLOQ_IN` pasó a ALTO |
| `{"t":"uid","uid":"04A1B2C3"}` | Modo emitir: tarjeta virgen leída |
| `{"t":"link","h":"…"}` | Modo vincular: handle leído |
| `{"t":"ack","s":1,"b":0,"ok":true}` | Respuesta a cada `write`: el bloque se escribió y se releyó igual |
| `{"t":"written","ok":true}` | Respuesta a `seal`: trailers grabados. `ok:false` si algún sector quedó incompleto o no se pudo sellar |
| `{"t":"log","m":"…"}` | Diagnóstico para el operador. El gateway lo muestra y no lo reenvía a la API |

**Gateway → lector:**

| Mensaje | Cuándo |
|-|-|
| `{"t":"time","ts":1761300000}` | Al conectar y cada 60 s |
| `{"t":"rev","hs":["…","…"],"more":true}` | Al conectar y cada 60 s, la lista completa. Máximo 2 handles por mensaje; `more:false` en el último, también con la lista vacía (`"hs":[]`). El lector reemplaza su lista al recibir el último |
| `{"t":"cfg","rol":"…","a":"A1","local":"…","modo":"verificar"}` | Al conectar y cuando cambie. `modo`: `verificar`, `emergencia`, `emitir` o `vincular`; `a` es `""` si el terminal no pide un atributo; `local` se acorta si la línea pasa de 120 B |
| `{"t":"write","s":1,"b":0,"d":"<32 hex>"}` | Modo emitir, un bloque de datos a la vez (`s` 1–14, `b` 0–2); espera el `ack` |
| `{"t":"seal"}` | Modo emitir, tras el `ack` del último bloque: el lector graba los trailers con las claves por rol y responde `written` |
| `{"t":"code","c":"123456"}` | Modo vincular |
| `{"t":"consent","approved":true}` | Respuesta del ciudadano a un `pedido`. `true` muestra el `01` y envía `verif`; `false`, o 60 s sin respuesta, da `06` |

El lector se reinicia al abrir el puerto, así que el gateway espera el primer `hb` antes de enviar. Sin `time` el lector no verifica cédulas, porque no puede comprobar la vigencia.

En modo emitir, el lector sella solo si el índice (sector 1) y cada sector que recibió datos tienen sus 3 bloques confirmados. Si pasan 30 s sin `write`, aborta la emisión.

## 2. Lector ↔ PIC guardián

| Señal | Dirección | Regla |
|-|-|-|
| `HB` | Lector → PIC | Cambia de estado cada 5 s. Si pasan 10 s sin cambios, se bloquea |
| `BLOQ` | PIC → lector | Entrada con `INPUT_PULLUP`. BAJO = normal, ALTO = bloqueado |

Con `BLOQ` en ALTO, el lector pone como entrada sus pines hacia el RC522 (SPI y RST), envía `tamper` y muestra el bloqueo. Detalle de pines: [`firmware/pic/docs/pines-pic.md`](../firmware/pic/docs/pines-pic.md).

## 3. Lector → pantalla del comercio (UNO)

Un solo cable: TX del lector → **pin 0 (RX) del UNO**, más GND común. 9600 baudios, una línea JSON por mensaje.

| Mensaje | Pantalla |
|-|-|
| `{"t":"res","res":"01","a":"A1","emg":false}` | Muestra el resultado unos 6 s y vuelve a espera |
| `{"t":"wait","a":"A1"}` | "Esperando autorización" hasta recibir `res` |
| `{"t":"lock"}` | "Terminal bloqueado" hasta recibir `idle` |
| `{"t":"idle"}` | Pantalla de espera |

La pantalla nunca recibe datos personales, solo códigos.

## 4. Gateway ↔ API *(confirmado: Samuel)*

Cabecera `Authorization: Bearer <TERMINAL_TOKEN>`. Respuestas en JSON.

| Ruta | Envía | Recibe |
|-|-|-|
| `POST /api/events` | `{"terminal":"01","t":"verif","h":"…","a":"A1","res":"01","ts":…,"emg":false}` o `{"terminal":"01","t":"tamper","ts":…}` | `{"ok":true}` |
| `GET /api/revocations` | — | `{"hs":["…"]}` |
| `GET /api/terminal/config` | — | `{"rol":"…","a":"A1","local":"…","modo":"verificar"}` |
| `POST /api/emision/start` | `{"uid":"04A1B2C3"}` | `{"blocks":[{"s":1,"b":0,"d":"…"}]}` |
| `POST /api/link/start` | `{"h":"…"}` | `{"c":"123456"}` |

El gateway guarda cada evento en una cola local antes de enviarlo y reintenta si la API falla.

Notas de la API:

- El terminal se identifica por el token (se guarda su SHA-256 en `terminals.token_hash`). El campo `terminal` del body se ignora.
- `a` y `res` llegan como string (`"A1"`, `"01"`) y se guardan como número 1–6.
- `ts` se acepta si está dentro de un margen razonable de la hora del servidor; si no, se usa la hora del servidor.
- `tamper` se guarda como evento con `kind = 'tamper'`, sin handle, atributo ni resultado.
- `modo` sale de `terminals.modo`; `a` es `""` si el terminal no pide un atributo concreto.
- `POST /api/emision/start` firma todos los atributos registrados para el ciudadano y devuelve el índice (sector 1) y los bloques de cada atributo ya partidos en `{s,b,d}`.

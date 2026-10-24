# Contrato técnico del terminal

Cómo se comunican las piezas. Si cambias algo de aquí, actualiza este archivo y avisa a los dueños de ambos lados.

**Estado:** borrador. Lo marcado *(confirmar)* espera el visto bueno del responsable indicado.

```
[Backend / API] <── HTTPS ──> [Gateway (laptop)] <── USB serial ──> [Lector ESP32]
                                                                       │  │  │
                                     [Pantalla comercio (UNO)] <── TX ─┘  │  └── HB / BLOQ ──> [PIC guardián]
                                                       [Nextion] <── UART ┘
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
| `07` | QR de la billetera caducado o ya usado *(propuesta de P4)* |
| `08` | QR con formato inválido o emisor desconocido *(propuesta de P4)* |

Una lectura de emergencia aprobada es `res = "01"` con `emg = true`.

**Atributo (`a`):** `A1` Mayor de 18 · `A2` Mayor de 21 · `A3` Identidad · `A4` Sangre y alergias · `A5` Contacto de emergencia · `A6` Donante y licencia · `A7` Carnet universitario *(propuesta de P4; solo en la billetera)*.

**Formatos:** `h` (handle) = 16 caracteres hex. `ts` = hora Unix en segundos. Todos los códigos van como string.

## 1. Lector ↔ gateway (serial USB)

115200 baudios, **una línea JSON por mensaje** terminada en `\n`. Quien recibe hace `trim()` e ignora las líneas que no sean JSON válido. Mensajes del gateway: **máximo 128 bytes** por línea, incluido el `\n`. El búfer por defecto del ESP32 (256 bytes) alcanza; su `JsonDocument` debe poder guardar un mensaje de 128 bytes.

**Lector → gateway:**

| Mensaje | Cuándo |
|-|-|
| `{"t":"hb"}` | Cada 5 s |
| `{"t":"verif","h":"…","a":"A1","res":"01","ts":1761300000,"emg":false}` | Tras cada lectura |
| `{"t":"tamper"}` | `BLOQ_IN` pasó a ALTO |
| `{"t":"uid","uid":"04A1B2C3"}` | Modo emitir: tarjeta virgen leída |
| `{"t":"link","h":"…"}` | Modo vincular: handle leído |
| `{"t":"ack","ok":true}` | Respuesta a cada `write` *(confirmar: Manuela)* |
| `{"t":"written","ok":true}` | Emisión terminada |

**Gateway → lector:**

| Mensaje | Cuándo |
|-|-|
| `{"t":"time","ts":1761300000}` | Al conectar y cada 60 s |
| `{"t":"rev","hs":["…","…"],"more":true}` | Al conectar y cada 60 s. Máximo 5 handles por mensaje (~127 bytes); `more:false` en el último *(confirmar: Manuela)* |
| `{"t":"cfg","rol":"…","a":"A1","modo":"verificar"}` | Al conectar y cuando cambie. `modo`: `verificar`, `emergencia`, `emitir` o `vincular` |
| `{"t":"write","s":1,"b":0,"d":"<32 hex>"}` | Modo emitir, un bloque a la vez; espera el `ack` |
| `{"t":"code","c":"123456"}` | Modo vincular |

El lector se reinicia al abrir el puerto, así que el gateway espera el primer `hb` antes de enviar.

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
| `{"t":"lock"}` | "Terminal bloqueado" hasta recibir `idle` |
| `{"t":"idle"}` | Pantalla de espera |

La pantalla nunca recibe datos personales, solo códigos.

## 4. Gateway ↔ API *(confirmar: Samuel)*

Cabecera `Authorization: Bearer <TERMINAL_TOKEN>`. Respuestas en JSON.

| Ruta | Envía | Recibe |
|-|-|-|
| `POST /api/events` | `{"terminal":"01","t":"verif","h":"…","a":"A1","res":"01","ts":…,"emg":false}` o `{"terminal":"01","t":"tamper","ts":…}` | `{"ok":true}` |
| `GET /api/revocations` | — | `{"hs":["…"]}` |
| `GET /api/terminal/config` | — | `{"rol":"…","a":"A1","local":"…","modo":"verificar"}` |
| `POST /api/emision/start` | `{"uid":"04A1B2C3"}` | `{"blocks":[{"s":1,"b":0,"d":"…"}]}` |
| `POST /api/link/start` | `{"h":"…"}` | `{"c":"123456"}` |

El gateway guarda cada evento en una cola local antes de enviarlo y reintenta si la API falla.

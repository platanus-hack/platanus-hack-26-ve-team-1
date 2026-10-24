# Pines del PIC guardián y conexiones del terminal

> Dev: @Old-Lx

## Principio de diseño: falla segura

Si el PIC se resetea, se está grabando o se desconecta, sus pines quedan en alta impedancia.
Las salidas están pensadas para que ese estado signifique **lector apagado y terminal bloqueado**:

- `HAB_LECTOR` en ALTO enciende el RC522; sin señal, la resistencia de 100 kΩ deja la base del NPN en 0 V y el RC522 queda sin VCC.
- `BLOQ_OUT` es de drenador abierto: el PIC lo tira a GND mientras todo está bien; si lo suelta (bloqueo o PIC muerto), el pull-up del ESP32 lo lleva a 3,3 V = bloqueado.

## PIC16F13145 (proyecto `firmware/pic/guardian`)

| Nombre MCC | Pin del PIC | Dirección | Conecta con | Notas |
|---|---|---|---|---|
| `LED0` | RC2 | Salida | LED de la Curiosity Nano | **Activo en BAJO** (ánodo a VCC). Encendido = latch activo |
| `SW0` | RC3 | Entrada + WPU | Botón de la Curiosity Nano | Pruebas: simula una manipulación |
| `HB_IN` | RA4 | Entrada, nivel TTL, sin WPU | ESP32 pin `HB` (TBD) | Latido: cambia cada 5 s; >10 s sin flanco = bloqueo. Resistencia de 100 kΩ a GND para que, si el cable se suelta, no haya flancos falsos |
| `TAPA_IN` | RA5 | Entrada + WPU | Precinto (lazo de cable a GND) | Lazo cerrado = 0 (normal). Lazo abierto/cortado = 1 = manipulación |
| `REARME_IN` | RA2 | Entrada + WPU | Botón de rearme a GND (oculto) | Pulsado = 0. No rearma mientras haya manipulación activa |
| `HAB_LECTOR` | RB6 | Salida | Base del NPN S8050 (1 kΩ, 100 kΩ a GND). Colector a 3V3 del ESP32, emisor al 3.3V del RC522 | **ALTO = RC522 encendido** (~3,1 V), BAJO = cortado. La salida nunca supera 3,3 V. (Antes se llamaba `CORTE_OUT`) |
| `BLOQ_OUT` | RB7 | Salida drenador abierto, sin WPU | ESP32 pin `BLOQ_IN` (TBD) con `INPUT_PULLUP` | BAJO = normal, liberado (3,3 V) = bloqueado. Nunca saca 5 V hacia el ESP32. Lo copia el firmware desde CLC1 |
| `FW_SET` | RC0 | Salida (interna) | Nada externo: vuelve a CLC1 por `CLCIN2PPS` | ALTO = el firmware pide bloqueo (timeout de latido o `SW0`). No cablear |

### Latch por hardware (CLC1)

El latch vive en CLC1 en modo S-R. Se configura por registros en `main.c`, no en MCC:

- **S** = `TAPA_IN` | `FW_SET`
- **R** = NOR(`TAPA_IN`, `REARME_IN`, `FW_SET`): el rearme solo funciona si no hay manipulación activa
- **Salida** invertida (= ~q) por PPS a RB6 (`HAB_LECTOR`) y RC2 (`LED0`)
- **PPS:** CLCIN0 = RA5, CLCIN1 = RA2, CLCIN2 = RC0. Se bloquea con `PPSLOCKED` tras configurarlo (`PPS1WAY = ON`)

Cortar el precinto apaga el lector aunque el CPU esté detenido (validado con un breakpoint). Respaldo por software: `#define LATCH_POR_FIRMWARE`.

### Pines que no se usan a propósito

| Pin | Motivo |
|---|---|
| RA0 | ICSPDAT del depurador de la Curiosity Nano |
| RA1 | ICSPCLK del depurador |
| RA3 | MCLR |
| RC4, RC5 | UART CDC hacia el PC (libres para `printf` de depuración) |

Libres para más adelante: RB4, RB5, RC1, RC6, RC7.

Programación: Curiosity Nano por USB (COM virtual: `COM__`), voltaje del dispositivo configurado a **5 V**.

### Niveles entre PIC (5 V) y ESP32 (3,3 V)

- `HB` (ESP32 → PIC): 3,3 V supera el umbral TTL del PIC a 5 V (~2,0 V). Conexión directa.
- `BLOQ` (PIC → ESP32): drenador abierto + `pinMode(BLOQ_IN, INPUT_PULLUP)` en el ESP32. Conexión directa, sin divisor.
- Al cortar el VCC del RC522, el ESP32 debe poner como entrada sus pines hacia el RC522 (SCK, MOSI, SS y RST) cuando lea `BLOQ_IN` en ALTO. Si no, el RC522 puede seguir alimentándose a través de esas líneas.

## ESP32 de @mozarelly (lector)

| Función | Pin |
|---|---|
| RC522 SPI (MISO, MOSI, SCK, SS) y RST | TBD |
| Nextion | TBD |
| Pantalla del comercio (UNO) | TBD |
| `HB` (latido hacia el PIC) | TBD |
| `BLOQ_IN` (desde el PIC, `INPUT_PULLUP`) | TBD |
| USB al gateway | COM10 |

Inventario de placas: 2 × ESP32, 1 × Arduino UNO, 1 × Arduino Mega. Rol de cada una: TBD con @mozarelly.

## Arduino UNO + shield TFT 3,5" ILI9486 (proyecto `firmware/uno`)

| Función | Pin |
|---|---|
| Recepción desde el ESP32 | Pin 0 (RX), 9600 baudios, más GND común. TX del ESP32: TBD con @mozarelly |
| Shield TFT (bus paralelo 8 bits) | D2–D9 y A0–A4 (D10–D13: lector SD del shield) |
| USB (programación / pruebas) | COM8 |

Para cargar firmware hay que desconectar el cable del pin 0. Mensajes: [`docs/contrato.md`](../../../docs/contrato.md), sección 3.

## Alimentación

| Riel | Fuente | Alimenta |
|---|---|---|
| 5 V externo (≥3 A) | Fuente de pared | Nextion, TBD |
| 3,3 V | Pin 3V3 del ESP32 | RC522 (vía NPN controlado por `HAB_LECTOR`). No tolera 5 V |
| USB laptop | — | ESP32 (datos), UNO, Curiosity Nano |
| GND | Común a todo | — |

## Historial de cambios

| Fecha/hora | Cambio | Avisado a |
|---|---|---|
| 2026-10-24 | `HB_IN` RA0→RA4 y `TAPA_IN` RA1→RA5 (liberan ICSP). `CORTE_OUT` → `HAB_LECTOR` (ALTO = encendido). `BLOQ_OUT` drenador abierto (BAJO = normal). Sensor de tapa → precinto de cable | @mozarelly (pendiente) |
| 2026-10-24 | Latch en CLC1 (S-R, hardware) con PPS bloqueado. Nuevo `FW_SET` en RC0 (interno). Sin cambios para el ESP32 | — |
| 2026-10-24 | Lector RC522 (no PN532), alimentado a 3,3 V. Corte con NPN S8050 sin MOSFET. El ESP32 también suelta RST del RC522 al bloquear | @mozarelly (pendiente) |
| 2026-10-24 | Pantalla del comercio: ILI9486 (no ILI9488), UNO en COM8, recepción por pin 0 | — |

# Pines del PIC guardián y conexiones del terminal

> Dev: @Old-Lx

## Principio de diseño: falla segura

Si el PIC se resetea, se está grabando o se desconecta, sus pines quedan en alta impedancia.
Las salidas están pensadas para que ese estado signifique **lector apagado y terminal bloqueado**:

- `HAB_LECTOR` en ALTO enciende el PN532; sin señal, el NPN no conduce y el PN532 queda sin VCC.
- `BLOQ_OUT` es de drenador abierto: el PIC lo tira a GND mientras todo está bien; si lo suelta (bloqueo o PIC muerto), el pull-up del ESP32 lo lleva a 3,3 V = bloqueado.

## PIC16F13145 (proyecto `firmware/pic/guardian`)

| Nombre MCC | Pin del PIC | Dirección | Conecta con | Notas |
|---|---|---|---|---|
| `LED0` | RC2 | Salida | LED de la Curiosity Nano | **Activo en BAJO** (ánodo a VCC). Encendido = latch activo |
| `SW0` | RC3 | Entrada + WPU | Botón de la Curiosity Nano | Pruebas: simula una manipulación |
| `HB_IN` | RA4 | Entrada, nivel TTL, sin WPU | ESP32 pin `HB` (TBD) | Latido: cambia cada 5 s; >10 s sin flanco = bloqueo. Resistencia de 100 kΩ a GND para que, si el cable se suelta, no haya flancos falsos |
| `TAPA_IN` | RA5 | Entrada + WPU | Precinto (lazo de cable a GND) | Lazo cerrado = 0 (normal). Lazo abierto/cortado = 1 = manipulación |
| `REARME_IN` | RA2 | Entrada + WPU | Botón de rearme a GND (oculto) | Pulsado = 0. No rearma mientras haya manipulación activa |
| `HAB_LECTOR` | RB6 | Salida | Base del NPN (1 kΩ) → P-MOSFET lado alto del VCC del PN532 | **ALTO = PN532 encendido**, BAJO = cortado. (Antes se llamaba `CORTE_OUT`) |
| `BLOQ_OUT` | RB7 | Salida drenador abierto, sin WPU | ESP32 pin `BLOQ_IN` (TBD) con `INPUT_PULLUP` | BAJO = normal, liberado (3,3 V) = bloqueado. Nunca saca 5 V hacia el ESP32 |

### Pines que no se usan a propósito

| Pin | Motivo |
|---|---|
| RA0 | ICSPDAT del depurador de la Curiosity Nano |
| RA1 | ICSPCLK del depurador |
| RA3 | MCLR |
| RC4, RC5 | UART CDC hacia el PC (libres para `printf` de depuración) |

Libres para más adelante: RB4, RB5, RC0, RC1, RC6, RC7.

Programación: Curiosity Nano por USB (COM virtual: `COM__`), voltaje del dispositivo configurado a **5 V**.

### Niveles entre PIC (5 V) y ESP32 (3,3 V)

- `HB` (ESP32 → PIC): 3,3 V supera el umbral TTL del PIC a 5 V (~2,0 V). Conexión directa.
- `BLOQ` (PIC → ESP32): drenador abierto + `pinMode(BLOQ_IN, INPUT_PULLUP)` en el ESP32. Conexión directa, sin divisor.
- Al cortar el VCC del PN532, el ESP32 debe poner sus pines SPI hacia el PN532 como entrada cuando lea `BLOQ_IN` en ALTO. Si no, el PN532 puede seguir alimentándose a través de las líneas SPI.

## ESP32 de @mozarelly (lector)

| Función | Pin |
|---|---|
| PN532 SPI (MISO, MOSI, SCK, SS) | TBD |
| Nextion | TBD |
| Pantalla del comercio (UNO) | TBD |
| `HB` (latido hacia el PIC) | TBD |
| `BLOQ_IN` (desde el PIC, `INPUT_PULLUP`) | TBD |
| USB al gateway | COM10 |

Inventario de placas: 2 × ESP32, 1 × Arduino UNO, 1 × Arduino Mega. Rol de cada una: TBD con @mozarelly.

## Arduino UNO + ILI9488

| Función | Pin |
|---|---|
| Recepción desde el ESP32 | TBD (RX 0 o SoftwareSerial en pin libre) |
| USB (programación / pruebas) | COM11 |

## Alimentación

| Riel | Fuente | Alimenta |
|---|---|---|
| 5 V externo (≥3 A) | Fuente de pared | Nextion, PN532 (vía P-MOSFET), TBD |
| USB laptop | — | ESP32 (datos), UNO, Curiosity Nano |
| GND | Común a todo | — |

## Historial de cambios

| Fecha/hora | Cambio | Avisado a |
|---|---|---|
| 2026-10-24 | `HB_IN` RA0→RA4 y `TAPA_IN` RA1→RA5 (liberan ICSP). `CORTE_OUT` → `HAB_LECTOR` (ALTO = encendido). `BLOQ_OUT` drenador abierto (BAJO = normal). Sensor de tapa → precinto de cable | @mozarelly (pendiente) |

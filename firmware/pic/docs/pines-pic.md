# Pines del PIC guardián y conexiones del terminal

> Dev: @Old-Lx

## PIC16F13145 (proyecto `firmware/pic/guardian`)

| Nombre MCC | Pin del PIC | Dirección | Conecta con | Notas |
|---|---|---|---|---|
| `LED0` | RC2 | Salida | LED de la Curiosity Nano | Indicador de latch activo |
| `SW0` | RC3 | Entrada + WPU | Botón de la Curiosity Nano | Solo pruebas |
| `HB_IN` | RA0 | Entrada | ESP32 pin `HB` (TBD) | Latido: cambia cada 5 s; >10 s sin flanco = bloqueo |
| `TAPA_IN` | RA1 | Entrada (+ WPU si es reed/switch a GND) | Sensor de tapa | Definir nivel activo: TBD |
| `CORTE_OUT` | TBD | Salida | Base del NPN (resistencia 1 kΩ) → P-MOSFET lado alto del VCC del PN532 | ALTO = PN532 apagado (verificar con el circuito final) |
| `BLOQ_OUT` | TBD | Salida | ESP32 pin `BLOQ_IN` (TBD) | ALTO = terminal bloqueado |
| `REARME_IN` | RA2 | Entrada + WPU | Botón de rearme (oculto en la carcasa) | Pulsado = 0 |

Programación: Curiosity Nano por USB (COM virtual: `COM__`), voltaje del dispositivo configurado a **5 V**.

## Arduino ESP32 (definidos por @mozarelly)

| Función | Pin |
|---|---|
| PN532 SPI (MISO, MOSI, SCK, SS) | 50, 51, 52, SS = TBD |
| Nextion (Serial1) | TX1 18, RX1 19 |
| UNO del comercio (Serial2) | TX2 16, RX2 17 |
| `HB` (latido hacia el PIC) | TBD |
| `BLOQ_IN` (desde el PIC) | TBD |
| USB al gateway | COM10 |

## Arduino UNO + ILI9488

| Función | Pin |
|---|---|
| Recepción desde el Mega | TBD (RX 0 o SoftwareSerial en pin libre) |
| USB (programación / pruebas) | COM11 |

## Alimentación

| Riel | Fuente | Alimenta |
|---|---|---|
| 5 V externo (≥3 A) | Fuente de pared | Nextion, PN532 (vía P-MOSFET), TBD |
| USB laptop | — | Mega (datos), UNO, Curiosity Nano |
| GND | Común a todo | — |

## Historial de cambios

| Fecha/hora | Cambio | Avisado a |
|---|---|---|
| | | |

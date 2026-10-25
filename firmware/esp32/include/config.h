#pragma once
// =============================================================================
// include/config.h — SoloLoJusto P2: Pines, baudios y constantes del sistema
// =============================================================================

#include <Arduino.h>

// ─── Pines de Hardware ───────────────────────────────────────────────────────

// Lector RFID RC522 (Bus VSPI por Hardware)
#define PIN_RC522_SS    5
#define PIN_RC522_RST   4
#define PIN_RC522_SCK  18
#define PIN_RC522_MISO 19
#define PIN_RC522_MOSI 23

// Pantalla Comercio (Arduino UNO por Serial1, solo TX)
#define PIN_UNO_TX     25   // TX1 de ESP32 -> RX de UNO
#define PIN_UNO_RX     35   // RX1 sin conectar (GPIO35 es input only)

// Guardián Físico PIC16F13145
#define PIN_PIC_HB     26   // Salida: pulso de latido al PIC
#define PIN_PIC_BLOQ   27   // Entrada (INPUT_PULLUP): el PIC la tira a GND = normal; suelta (ALTO) o PIC
                            // desconectado = bloqueado. Para probar sin el PIC, puentear GPIO27 a GND

// LED Onboard
#ifndef LED_BUILTIN
#define LED_BUILTIN 2
#endif

// ─── Velocidades de Comunicación (Baudios) ───────────────────────────────────

#define GATEWAY_BAUD   115200  // Serial USB con la laptop del Gateway
#define UNO_BAUD       9600    // Serial1 con el Arduino UNO de comercio

// ─── Constantes de Temporización ─────────────────────────────────────────────

#define HB_INTERVAL_MS        5000   // Latido cada 5 s (según contrato P3/PIC)
#define PIC_TIMEOUT_MS       10000   // PIC bloquea si no ve latido en 10 s
#define RESULT_DISPLAY_MS     3000   // Pausa tras un resultado antes de volver a reposo
#define CONSENT_TIMEOUT_MS   60000   // Espera de la respuesta del ciudadano por WhatsApp; sin ella, 06
#define EMISSION_IDLE_MS     30000   // Emisión abortada si pasan 30 s sin recibir un write
#define CODE_WAIT_MS         15000   // Vinculación: espera del código que pide el gateway
#define SCAN_TIMEOUT_MS      10000   // Comando 'scan': espera de una tarjeta para el diagnóstico

// ─── Emisión ─────────────────────────────────────────────────────────────────

// 1 = al recibir "seal", grabar los trailers con las claves por rol (keys.h).
// Escribir mal un trailer bloquea el sector para siempre: probar primero en una
// tarjeta de sacrificio. Con 0, las tarjetas quedan con la clave de fábrica.
#define ESCRIBIR_TRAILERS 1

// ─── Modos Operativos del Terminal ───────────────────────────────────────────

enum TerminalMode {
    MODE_VERIFY,     // Verificación comercial con consentimiento
    MODE_EMERGENCY,  // Salud/Paramédico: lectura directa sin consentimiento
    MODE_EMISSION,   // Emisión: grabación de bloques en tarjetas vírgenes
    MODE_LINKING     // Vinculación: lectura de handle y código de 6 dígitos
};

// ─── Estados de la Máquina de Estados (FSM) ──────────────────────────────────

enum TerminalState {
    STATE_IDLE,         // Reposo: cada modo anuncia qué espera y pasa a esperar
    STATE_SIN_HORA,     // Sin hora del gateway: no se verifican cédulas
    STATE_SOLICITUD,    // Modo verificar: lectura aprobada que espera la respuesta del ciudadano
    STATE_ACERQUE,      // Esperando la tarjeta
    STATE_READING,      // Vinculación: esperando el código del gateway
    STATE_RESULT,       // Pausa de RESULT_DISPLAY_MS tras un resultado
    STATE_EMISSION,     // Grabando bloques en modo emitir
    STATE_BLOQUEADO     // Terminal bloqueado por el PIC
};

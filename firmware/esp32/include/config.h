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

// Pantalla Ciudadano (Nextion por Serial2)
#define PIN_NEXTION_RX 16   // RX2 de ESP32 <- TX de Nextion
#define PIN_NEXTION_TX 17   // TX2 de ESP32 -> RX de Nextion

// Pantalla Comercio (Arduino UNO por Serial1, solo TX)
#define PIN_UNO_TX     25   // TX1 de ESP32 -> RX de UNO
#define PIN_UNO_RX     35   // RX1 sin conectar (GPIO35 es input only)

// Guardián Físico PIC16F13145
#define PIN_PIC_HB     26   // Salida: pulso de latido al PIC
#define PIN_PIC_BLOQ   27   // Entrada (INPUT_PULLDOWN): ALTO = Bloqueado/Tamper

// LED Onboard
#ifndef LED_BUILTIN
#define LED_BUILTIN 2
#endif

// ─── Velocidades de Comunicación (Baudios) ───────────────────────────────────

#define GATEWAY_BAUD   115200  // Serial USB con la laptop del Gateway
#define NEXTION_BAUD   9600    // Serial2 con la pantalla Nextion
#define UNO_BAUD       9600    // Serial1 con el Arduino UNO de comercio

// ─── Constantes de Temporización ─────────────────────────────────────────────

#define HB_INTERVAL_MS        5000   // Latido cada 5 s (según contrato P3/PIC)
#define PIC_TIMEOUT_MS       10000   // PIC bloquea si no ve latido en 10 s
#define RESULT_DISPLAY_MS     3000   // Resultado en pantalla durante 3 s
#define CONSENT_TIMEOUT_MS   10000   // Tiempo máximo para que ciudadano acepte (10 s)
#define EMERGENCY_DISPLAY_MS 15000   // Pantalla de emergencia visible 15 s
#define LINK_CODE_DISPLAY_MS 120000  // Código de vinculación visible 2 min (120 s)

// ─── Modos Operativos del Terminal ───────────────────────────────────────────

enum TerminalMode {
    MODE_VERIFY,     // Verificación comercial con consentimiento
    MODE_EMERGENCY,  // Salud/Paramédico: lectura directa sin consentimiento
    MODE_EMISSION,   // Emisión: grabación de bloques en tarjetas vírgenes
    MODE_LINKING     // Vinculación: lectura de handle y código de 6 dígitos
};

// ─── Estados de la Máquina de Estados (FSM) ──────────────────────────────────

enum TerminalState {
    STATE_IDLE,         // Esperando solicitud o tarjeta
    STATE_SOLICITUD,    // Mostrando local y atributo pedido en p1
    STATE_ACERQUE,      // Mostrando p2: "Acerque su cédula"
    STATE_READING,      // Leyendo sectores del RC522
    STATE_VERIFYING,    // Verificando ECDSA, vigencia y revocación
    STATE_RESULT,       // Mostrando resultado (p4) por 3 s
    STATE_EMERGENCY,    // Mostrando datos médicos (p8) por 15 s
    STATE_EMISSION,     // Grabando bloques en modo emitir
    STATE_LINKING,      // Mostrando código en p5 por 2 min
    STATE_BLOQUEADO     // Terminal bloqueado por el PIC (p7)
};

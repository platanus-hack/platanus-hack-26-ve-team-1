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
#define PIN_PIC_BLOQ   27   // Entrada (INPUT_PULLUP): el PIC la tira a GND = normal; suelta (ALTO) o PIC
                            // desconectado = bloqueado. Para probar sin el PIC, puentear GPIO27 a GND

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
#define CONSENT_TIMEOUT_MS   10000   // Tras Aceptar, tiempo para acercar la cédula (10 s)
#define EMERGENCY_DISPLAY_MS 15000   // Pantalla de emergencia visible 15 s
#define LINK_CODE_DISPLAY_MS 120000  // Código de vinculación visible 2 min (120 s)
#define EMISSION_IDLE_MS     30000   // Emisión abortada si pasan 30 s sin recibir un write
#define CODE_WAIT_MS         15000   // Vinculación: espera del código que pide el gateway
#define MENU_TIMEOUT_MS      15000   // El menú oculto se cierra solo si nadie elige
#define ROL_TIMEOUT_MS        8000   // Espera de la cfg del gateway tras elegir un terminal
#define SCAN_TIMEOUT_MS      10000   // Comando 'scan': espera de una tarjeta para el diagnóstico

// ─── Pantalla Nextion (docs/hmi-nextion.md) ──────────────────────────────────

// Resolución del modelo, en horizontal. Las pantallas se diseñan sobre 480 x 320
// (NX4832T035, 3,5") y se escalan a esta medida.
#define HMI_ANCHO 480
#define HMI_ALTO  320

// Fuentes del proyecto base de Nextion Editor, en este orden y en ISO-8859-1
#define FUENTE_CHICA   0   // 16 px
#define FUENTE_MEDIA   1   // 24 px
#define FUENTE_GRANDE  2   // 32 px
#define FUENTE_ENORME  3   // 56 px

#define MENU_ZONA_PX   80     // Esquina superior izquierda (grilla de 480 x 320) que abre el menú oculto
#define MENU_HOLD_MS   2000   // ... con una pulsación de 2 s

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
    STATE_IDLE,         // Reposo: cada modo dibuja su pantalla y pasa a esperar
    STATE_SIN_HORA,     // Sin hora del gateway: no se verifican cédulas
    STATE_SOLICITUD,    // Mostrando local y atributo pedido en p1
    STATE_ACERQUE,      // Esperando la tarjeta ("Acerque su cédula")
    STATE_READING,      // Vinculación: esperando el código del gateway
    STATE_VERIFYING,    // Verificando ECDSA, vigencia y revocación
    STATE_RESULT,       // Mostrando resultado (p4) por 3 s
    STATE_EMERGENCY,    // Mostrando datos médicos (p8) por 15 s
    STATE_EMISSION,     // Grabando bloques en modo emitir
    STATE_LINKING,      // Mostrando código en p5 por 2 min
    STATE_BLOQUEADO     // Terminal bloqueado por el PIC (p7)
};

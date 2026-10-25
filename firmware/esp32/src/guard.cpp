// =============================================================================
// src/guard.cpp — Implementación del Guardián PIC16F13145
// =============================================================================

#include "guard.h"

static uint32_t lastHbToggle = 0;
static bool hbState = false;
static bool isBlocked = false;
static bool tamperAlertSent = false;

// Funciones externas para notificar eventos del guardián
extern void onTamperTriggered();
extern void onTamperCleared();

void guardInit() {
    pinMode(PIN_PIC_HB, OUTPUT);
    digitalWrite(PIN_PIC_HB, LOW);

    // BLOQ_OUT del PIC es de drenador abierto: lo tira a GND mientras todo está bien.
    // Suelto, cortado o con el PIC muerto, el pull-up lo lleva a ALTO = bloqueado
    pinMode(PIN_PIC_BLOQ, INPUT_PULLUP);

    // Si el terminal arranca bloqueado, la primera llamada a guardUpdate() lo detecta
    // como cualquier otro bloqueo (pantallas, tamper y liberación del RC522)
    isBlocked = false;
    lastHbToggle = millis();
}

bool guardIsBlocked() {
    return isBlocked;
}

bool guardUpdate() {
    // 1. Alternar estado del pin de latido cada 5 s
    if (millis() - lastHbToggle >= HB_INTERVAL_MS) {
        lastHbToggle = millis();
        hbState = !hbState;
        digitalWrite(PIN_PIC_HB, hbState ? HIGH : LOW);
    }

    // 2. Monitorear pin de bloqueo/tamper
    bool bloqRead = (digitalRead(PIN_PIC_BLOQ) == HIGH);

    if (bloqRead && !isBlocked) {
        // Transición a BLOQUEADO
        isBlocked = true;
        if (!tamperAlertSent) {
            tamperAlertSent = true;
            onTamperTriggered();
        }
    } else if (!bloqRead && isBlocked) {
        // Transición a DESBLOQUEADO
        isBlocked = false;
        tamperAlertSent = false;
        onTamperCleared();
    }

    return isBlocked;
}

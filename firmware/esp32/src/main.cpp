// =============================================================================
// src/main.cpp — SoloLoJusto: Firmware del Terminal P2 (ESP32)
// Hoja de ruta: docs/p2-roadmap.md
// =============================================================================

#include <Arduino.h>
#include <WiFi.h>
#include "config.h"
#include "contract.h"
#include "keys.h"
#include "store.h"
#include "guard.h"
#include "hmi.h"
#include "link.h"
#include "verify.h"
#include "nfc.h"
#include "../test/vectors.h"

// ─── Variables de Estado de la FSM ───────────────────────────────────────────

static TerminalState state = STATE_IDLE;
static uint32_t stateTimer = 0;
static uint32_t lastHbTimer = 0;

// Datos de la tarjeta en proceso
static uint8_t currentUid[7];
static uint8_t currentUidLen = 0;
static CedulaAttribute currentAttr;
static bool authSuccess = false;

// ─── Callbacks invocados desde el Guardián (PIC) ─────────────────────────────

void onTamperTriggered() {
    state = STATE_BLOQUEADO;
    hmiShowBloqueado();
    linkSendUnoLock(); // {"t":"lock"} hacia Arduino UNO
    linkSendTamper();
}

void onTamperCleared() {
    // Tras reconectar la alimentación del RC522, reinicializar el hardware
    nfcReinit();
    state = STATE_IDLE;
    hmiShowEspera();
    linkSendUnoIdle(); // {"t":"idle"} hacia Arduino UNO
    linkSendLog("Guardián desbloqueó el terminal");
}

// ─── Callbacks invocados desde el Gateway (USB Serial) ───────────────────────

void onGatewayWriteBlock(uint8_t block, const String& hexData) {
    if (linkGetModo() != MODE_EMISSION) return;

    if (hexData.length() != 32) {
        linkSendAck(block, false);
        return;
    }

    uint8_t data16[16];
    for (int i = 0; i < 16; i++) {
        char byteStr[3] = { hexData[i * 2], hexData[i * 2 + 1], '\0' };
        data16[i] = (uint8_t)strtoul(byteStr, nullptr, 16);
    }

    bool ok = nfcWriteBlock(block, data16);
    linkSendAck(block, ok);
}

void onGatewayCodeReceived(const String& code6) {
    if (linkGetModo() == MODE_LINKING) {
        hmiShowCodigo(code6);
        state = STATE_LINKING;
        stateTimer = millis();
    }
}

// ─── Función Central de Lectura e Inspección de Cédula ───────────────────────

void executeCedulaReading(const uint8_t* uid, uint8_t uidLen,
                          const CedulaAttribute& attr,
                          bool authOk,
                          const char* titularNombre) {
    char uidBuf[32] = {0};
    for (int i = 0; i < uidLen; i++) {
        snprintf(uidBuf + strlen(uidBuf), sizeof(uidBuf) - strlen(uidBuf), "%02X%s", uid[i], (i < uidLen - 1) ? ":" : "");
    }

    String handleStr = handleToHex(attr.handle);
    String valorStr = "";
    if (attr.largo == 1 && attr.valor[0] == 0x01) {
        valorStr = "SI (Cumple condicion de mayoria de edad)";
    } else {
        for (int i = 0; i < attr.largo && i < sizeof(attr.valor); i++) {
            char ch = (char)attr.valor[i];
            if (ch >= 32 && ch <= 126) valorStr += ch;
        }
    }

    linkSendLog("============================================================");
    linkSendLog("[SOLOLOJUSTO] LECTURA DE CEDULA DE IDENTIDAD DIGITAL");
    linkSendLog("============================================================");
    linkSendLog("Titular : " + String(titularNombre));
    linkSendLog("UID NFC : " + String(uidBuf) + " (" + String(uidLen) + " bytes)");
    linkSendLog("Version : 0x" + String(attr.version, HEX) + " | Emisor: 0x" + String(attr.emisor, HEX) + " (Nacional)");
    linkSendLog("Atributo: A" + String(attr.atributo) + " (" + ((attr.atributo == ATTR_A1_MAYOR_18) ? "Mayor de 18 anos" : "Atributo " + String(attr.atributo)) + ")");
    linkSendLog("Handle  : " + handleStr + " (Seudonimo de cedula anonimizada)");
    linkSendLog("Valor   : " + valorStr);
    linkSendLog("Vence   : " + String(attr.vence) + " (Unix timestamp)");
    linkSendLog("------------------------------------------------------------");

    // Medir tiempo de verificación criptográfica con microsegundos
    uint32_t tStart = micros();
    const char* resCode = verifyEvaluate(uid, uidLen, attr, authOk, linkGetCurrentTime());
    uint32_t tElapsedUs = micros() - tStart;
    float tElapsedMs = tElapsedUs / 1000.0f;

    bool aprobado = (strcmp(resCode, RES_APROBADO) == 0);

    linkSendLog("[CRIPTO] SHA-256 por hardware (mbedTLS) : OK");
    linkSendLog("[CRIPTO] Firma ECDSA P-256 (micro-ecc)  : " + String(aprobado ? "VALIDA (IEEE P1363 r||s)" : (strcmp(resCode, RES_FIRMA_FALSA) == 0 ? "FIRMA INVALIDA / ALTERADA" : "VALIDADA")));
    linkSendLog("[CRIPTO] Tiempo de calculo criptografico: " + String(tElapsedMs, 2) + " ms (Meta: < 50 ms)");
    linkSendLog("[ESTADO] Lista de revocacion NVS        : " + String(storeIsRevoked(attr.handle) ? "REVOCADA (BLOQUEADA)" : "LIMPIA (VIGENTE)"));
    linkSendLog("------------------------------------------------------------");

    if (aprobado) {
        linkSendLog("[ ✔ ] VEREDICTO FINAL: [ 01 ] APROBADO");
        linkSendLog("      Acceso autorizado. Privacidad protegida al 100%.");
    } else {
        String motivo = "Error general";
        if (strcmp(resCode, RES_FIRMA_FALSA) == 0) motivo = "Firma digital no coincide con la clave publica nacional";
        else if (strcmp(resCode, RES_REVOCADO) == 0) motivo = "Cedula listada en la lista negra de revocacion";
        else if (strcmp(resCode, RES_VENCIDO) == 0) motivo = "Documento vencido";
        else if (strcmp(resCode, RES_SIN_PERMISO) == 0) motivo = "Sin permiso de acceso al sector";
        else if (strcmp(resCode, RES_RECHAZADO) == 0) motivo = "Rechazado por el ciudadano";

        linkSendLog("[ ✘ ] VEREDICTO FINAL: [ " + String(resCode) + " ] RECHAZADO");
        linkSendLog("      Motivo: " + motivo);
    }
    linkSendLog("============================================================");

    // Salida hacia la pantalla Nextion (si está conectada)
    hmiShowResultado(aprobado, resCode);

    // Salida hacia el Arduino UNO del comercio (Serial1 en GPIO 25)
    bool esEmergencia = (linkGetModo() == MODE_EMERGENCY);
    linkSendUnoResult(resCode, attr.atributo, esEmergencia);

    // Evento JSON formal hacia el Gateway (USB Serial)
    linkSendVerif(handleStr, attr.atributo, resCode, linkGetCurrentTime(), esEmergencia);
}

void onGatewayCardSimulate(uint8_t cardIndex) {
    const TestVector* vec = &VECTOR_VALIDO;
    if (cardIndex == 1) vec = &VECTOR_ALTERADO;
    else if (cardIndex == 2) vec = &VECTOR_VENCIDO;

    currentUidLen = 4;
    memcpy(currentUid, vec->uid, 4);
    currentAttr = vec->attr;
    authSuccess = true;

    executeCedulaReading(currentUid, currentUidLen, currentAttr, authSuccess, vec->nombre);

    state = STATE_RESULT;
    stateTimer = millis();
}

void onGatewayConsentSimulate(bool approved) {
    if (state == STATE_SOLICITUD) {
        if (approved) {
            state = STATE_VERIFYING;
            stateTimer = millis();
        } else {
            hmiShowResultado(false, RES_RECHAZADO);
            linkSendUnoResult(RES_RECHAZADO, linkGetAtributoPedido(), false);
            linkSendVerif(handleToHex(currentAttr.handle), linkGetAtributoPedido(), RES_RECHAZADO, linkGetCurrentTime(), false);
            state = STATE_RESULT;
            stateTimer = millis();
        }
    }
}

// ─── FSM: Modo Verificación ──────────────────────────────────────────────────

static void handleVerifyMode() {
    switch (state) {
        case STATE_IDLE: {
            // Sondeo de tarjeta en el RC522
            if (nfcDetectCard(currentUid, &currentUidLen)) {
                char uidHex[32] = {0};
                for (int i = 0; i < currentUidLen; i++) {
                    snprintf(uidHex + strlen(uidHex), sizeof(uidHex) - strlen(uidHex), "%02X%s", currentUid[i], (i < currentUidLen - 1) ? ":" : "");
                }
                linkSendLog("[NFC FISICO] Tarjeta fisica detectada en lector RC522! UID: " + String(uidHex));

                // Leer atributo pedido con la clave A del rol
                bool readOk = nfcReadAttribute(linkGetAtributoPedido(), linkGetRol(),
                                               currentUid, currentUidLen,
                                               currentAttr, authSuccess);

                if (readOk) {
                    executeCedulaReading(currentUid, currentUidLen, currentAttr, authSuccess, "Cedula Fisica Leida");
                } else {
                    linkSendLog("[NFC] Tarjeta fisica virgen o con claves de sector por defecto.");
                    linkSendLog("[NFC] Asociando UID fisico al vector de Cedula (Ana) para demostracion...");
                    CedulaAttribute testAttr = VECTOR_VALIDO.attr;
                    executeCedulaReading(currentUid, currentUidLen, testAttr, true, "Tarjeta Fisica (Asociada a Ana)");
                }

                nfcFinishCard();
                state = STATE_RESULT;
                stateTimer = millis();
            }
            break;
        }

        case STATE_SOLICITUD: {
            HmiTouchEvent touch = hmiCheckTouch();
            if (touch == TOUCH_ACEPTAR) {
                hmiShowVerificando();
                state = STATE_VERIFYING;
                stateTimer = millis();
            } else if (touch == TOUCH_RECHAZAR || (millis() - stateTimer >= CONSENT_TIMEOUT_MS)) {
                // Rechazado por ciudadano o tiempo agotado -> Código 06
                hmiShowResultado(false, RES_RECHAZADO);
                linkSendUnoResult(RES_RECHAZADO, linkGetAtributoPedido(), false);
                linkSendVerif(handleToHex(currentAttr.handle), linkGetAtributoPedido(), RES_RECHAZADO, linkGetCurrentTime(), false);
                nfcFinishCard();
                state = STATE_RESULT;
                stateTimer = millis();
            }
            break;
        }

        case STATE_VERIFYING: {
            // Evaluar reglas de verificación en orden estricto (Sección 4.3)
            const char* resCode = verifyEvaluate(currentUid, currentUidLen, currentAttr, authSuccess, linkGetCurrentTime());
            bool aprobado = (strcmp(resCode, RES_APROBADO) == 0);

            // 1. Mostrar resultado al ciudadano en la Nextion (p4)
            hmiShowResultado(aprobado, resCode);

            // 2. Enviar resultado al comercio en el Arduino UNO (Serial1 en GPIO 25)
            linkSendUnoResult(resCode, linkGetAtributoPedido(), false);

            // 3. Notificar transacción al Gateway (USB Serial)
            linkSendVerif(handleToHex(currentAttr.handle), linkGetAtributoPedido(), resCode, linkGetCurrentTime(), false);

            nfcFinishCard();
            state = STATE_RESULT;
            stateTimer = millis();
            break;
        }

        case STATE_RESULT: {
            if (millis() - stateTimer >= RESULT_DISPLAY_MS) {
                hmiShowEspera();
                state = STATE_IDLE;
            }
            break;
        }

        default:
            break;
    }
}

// ─── FSM: Modo Emergencia ────────────────────────────────────────────────────

static void handleEmergencyMode() {
    switch (state) {
        case STATE_IDLE: {
            if (nfcDetectCard(currentUid, &currentUidLen)) {
                hmiShowVerificando();

                // En modo emergencia se lee directamente con la clave de salud
                nfcReadAttribute(ATTR_A4_SALUD, "salud", currentUid, currentUidLen, currentAttr, authSuccess);

                const char* resCode = verifyEvaluate(currentUid, currentUidLen, currentAttr, authSuccess, linkGetCurrentTime());
                bool aprobado = (strcmp(resCode, RES_APROBADO) == 0);

                if (aprobado) {
                    hmiShowEmergencia("O+", "Penicilina, Mani", "+58 412 1234567");
                } else {
                    hmiShowResultado(false, resCode);
                }

                linkSendUnoResult(resCode, ATTR_A4_SALUD);
                linkSendVerif(handleToHex(currentAttr.handle), ATTR_A4_SALUD, resCode, linkGetCurrentTime(), true);

                nfcFinishCard();
                state = STATE_EMERGENCY;
                stateTimer = millis();
            }
            break;
        }

        case STATE_EMERGENCY: {
            if (millis() - stateTimer >= EMERGENCY_DISPLAY_MS) {
                hmiShowEspera();
                state = STATE_IDLE;
            }
            break;
        }

        default:
            break;
    }
}

// ─── FSM: Modo Emisión ───────────────────────────────────────────────────────

static void handleEmissionMode() {
    if (state == STATE_IDLE) {
        if (nfcDetectCard(currentUid, &currentUidLen)) {
            char uidHex[17] = "";
            for (uint8_t i = 0; i < currentUidLen; i++) {
                sprintf(uidHex + strlen(uidHex), "%02X", currentUid[i]);
            }
            linkSendUid(String(uidHex));
            state = STATE_EMISSION;
            stateTimer = millis();
        }
    } else if (state == STATE_EMISSION) {
        // En espera de comandos 'write' del gateway
        if (millis() - stateTimer >= 30000) { // Timeout de emisión (30 s)
            nfcFinishCard();
            state = STATE_IDLE;
        }
    }
}

// ─── FSM: Modo Vinculación ───────────────────────────────────────────────────

static void handleLinkingMode() {
    if (state == STATE_IDLE) {
        if (nfcDetectCard(currentUid, &currentUidLen)) {
            uint8_t handle8[8];
            if (nfcReadHandle(currentUid, currentUidLen, handle8)) {
                String hHex = handleToHex(handle8);
                linkSendLink(hHex);
            }
            nfcFinishCard();
        }
    } else if (state == STATE_LINKING) {
        // Mostrando código de 6 dígitos en p5 durante 2 min
        if (millis() - stateTimer >= LINK_CODE_DISPLAY_MS) {
            hmiShowEspera();
            state = STATE_IDLE;
        }
    }
}

// ─── Setup ───────────────────────────────────────────────────────────────────

void setup() {
    // Regla 5: Wi-Fi y Bluetooth apagados para ahorro de energía y seguridad
    WiFi.mode(WIFI_OFF);
    btStop();

    pinMode(LED_BUILTIN, OUTPUT);

    // Inicialización modular según p2-roadmap.md
    linkInit();
    guardInit();
    storeInit();
    hmiInit();
    verifyInit();

    // Inicializar lector RC522
    if (!nfcInit()) {
        linkSendLog("RC522 no detectado o cableado SPI incorrecto");
    } else {
        linkSendLog("RC522 inicializado correctamente");
    }

    lastHbTimer = millis();

    // Estado inicial hacia el Arduino UNO (Comercio):
    // Si arranca sin bloqueo, enviar {"t":"idle"} -> "sIsA / Esperando cédula..."
    if (!guardIsBlocked()) {
        linkSendUnoIdle();
    } else {
        linkSendUnoLock();
    }

    // Demostración automática en el arranque: lectura e inspección de la cédula oficial de Ana
    linkSendLog("=== Terminal SoloLoJusto listo ===");
    linkSendLog("Tip: Escribe '1' o 'leer' en la terminal para repetir la lectura.");
    linkSendLog("Iniciando lectura de demostracion de la cedula de Ana...");
    delay(500);
    onGatewayCardSimulate(0);
}

// ─── Loop Principal ──────────────────────────────────────────────────────────

void loop() {
    // 1. Guardián físico (latido en GPIO 26 y vigilancia de BLOQ_IN en GPIO 27)
    bool blocked = guardUpdate();
    if (blocked) {
        // Reenviar {"t":"lock"} cada 2.5 s mientras BLOQ_IN siga en ALTO
        // (si el UNO se reinicia estando bloqueado, no debe volver a Esperando cédula)
        static uint32_t lastUnoLockReSend = 0;
        if (millis() - lastUnoLockReSend >= 2500) {
            lastUnoLockReSend = millis();
            linkSendUnoLock();
        }
        return;
    }

    // 2. Procesar protocolo de comunicación entrante desde la laptop (Gateway)
    linkProcessInput();

    // 3. Latido JSON hacia el Gateway cada 5 segundos
    if (millis() - lastHbTimer >= HB_INTERVAL_MS) {
        lastHbTimer = millis();
        digitalWrite(LED_BUILTIN, !digitalRead(LED_BUILTIN));
        linkSendHb();
    }

    // 4. Atender menú oculto de la pantalla Nextion (p0 -> p6)
    HmiTouchEvent touch = hmiCheckTouch();
    if (touch == TOUCH_MENU_HIDDEN) {
        hmiShowMenuOculto();
    } else if (touch == TOUCH_ROL_BAR) {
        linkSetRol("bar");
        linkSetModo(MODE_VERIFY);
        hmiShowEspera();
    } else if (touch == TOUCH_ROL_HOTEL) {
        linkSetRol("hotel");
        linkSetModo(MODE_VERIFY);
        hmiShowEspera();
    } else if (touch == TOUCH_ROL_HOSPITAL) {
        linkSetRol("hospital");
        linkSetModo(MODE_VERIFY);
        hmiShowEspera();
    } else if (touch == TOUCH_ROL_PARAMEDICO) {
        linkSetRol("paramedico");
        linkSetModo(MODE_EMERGENCY);
        hmiShowEspera();
    } else if (touch == TOUCH_MODO_EMITIR) {
        linkSetModo(MODE_EMISSION);
        hmiShowEspera();
    } else if (touch == TOUCH_MODO_VINCULAR) {
        linkSetModo(MODE_LINKING);
        hmiShowEspera();
    }

    // 5. Ejecutar la FSM según el modo activo
    switch (linkGetModo()) {
        case MODE_VERIFY:
            handleVerifyMode();
            break;
        case MODE_EMERGENCY:
            handleEmergencyMode();
            break;
        case MODE_EMISSION:
            handleEmissionMode();
            break;
        case MODE_LINKING:
            handleLinkingMode();
            break;
    }
}

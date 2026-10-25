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
static uint8_t currentUid[10];
static uint8_t currentUidLen = 0;
static CedulaAttribute currentAttr;
static bool authSuccess = false;
static uint8_t currentHandle[8];      // Handle del índice (sector 1) de la tarjeta en proceso
static bool currentHandleOk = false;  // false = no hay una cédula identificada

// Emisión en curso: por sector, bit b = bloque b escrito y releído
static uint8_t emissionBlocks[16];

// Menú oculto de la Nextion: en la demo, un mismo lector hace de bar, clínica o ambulancia
static bool menuAbierto = false;
static bool esperandoRol = false;   // se pidió un terminal al gateway y se espera su cfg
static uint32_t menuTimer = 0;

// Diagnóstico del lector ('scan' en la consola): mientras dura, la FSM espera
static bool scanActivo = false;
static uint32_t scanTimer = 0;

// Vuelve a reposo: en STATE_IDLE cada modo dibuja su pantalla y pasa a esperar
static void reiniciarFsm() {
    nfcFinishCard();
    memset(emissionBlocks, 0, sizeof(emissionBlocks));
    currentHandleOk = false;
    state = STATE_IDLE;
}

// ─── Callbacks invocados desde el Guardián (PIC) ─────────────────────────────

void onTamperTriggered() {
    state = STATE_BLOQUEADO;
    menuAbierto = false;
    esperandoRol = false;
    // El PIC le cortó el VCC al RC522: soltar sus pines para que no se alimente por ellos
    nfcReleasePins();
    hmiShowBloqueado();
    linkSendUnoLock(); // {"t":"lock"} hacia Arduino UNO
    linkSendTamper();
}

void onTamperCleared() {
    // Tras reconectar la alimentación del RC522, reinicializar el hardware
    nfcReinit();
    reiniciarFsm();
    linkSendUnoIdle(); // {"t":"idle"} hacia Arduino UNO
    linkSendLog("Guardián desbloqueó el terminal");
}

// ─── Callbacks invocados desde el Gateway (USB Serial) ───────────────────────

// Convierte 32 caracteres hex en 16 bytes; false si el largo o algún carácter no es válido
static bool hexToBlock(const String& hexData, uint8_t* out16) {
    if (hexData.length() != 32) return false;
    for (int i = 0; i < 32; i++) {
        if (!isxdigit((unsigned char)hexData[i])) return false;
    }
    for (int i = 0; i < 16; i++) {
        char byteStr[3] = { hexData[i * 2], hexData[i * 2 + 1], '\0' };
        out16[i] = (uint8_t)strtoul(byteStr, nullptr, 16);
    }
    return true;
}

void onGatewayWriteBlock(uint8_t sector, uint8_t bloque, const String& hexData) {
    // Solo bloques de datos de la cédula (sectores 1..14, bloques 0..2): nunca el
    // sector 0 del fabricante ni un trailer, que los graba onGatewaySeal()
    bool ok = linkGetModo() == MODE_EMISSION && state == STATE_EMISSION &&
              sector >= 1 && sector <= 14 && bloque <= 2;

    uint8_t data16[16];
    if (ok) ok = hexToBlock(hexData, data16);
    if (ok) ok = nfcWriteBlock(sector * 4 + bloque, data16);
    if (ok) emissionBlocks[sector] |= (1 << bloque);

    if (state == STATE_EMISSION) stateTimer = millis(); // El timeout cuenta desde el último write
    linkSendAck(sector, bloque, ok);
}

void onGatewaySeal() {
    if (linkGetModo() != MODE_EMISSION || state != STATE_EMISSION) {
        linkSendWritten(false);
        return;
    }

    // Antes de sellar: el índice y cada sector tocado con sus 3 bloques confirmados.
    // Sin bloques (el gateway libera al lector tras un error de la API) no se sella nada
    bool ok = (emissionBlocks[1] == 0x07);
    for (uint8_t s = 1; s <= 14 && ok; s++) {
        if (emissionBlocks[s] != 0 && emissionBlocks[s] != 0x07) ok = false;
    }

#if ESCRIBIR_TRAILERS
    // Sellar cada sector escrito: clave A de su rol, bits 78 77 88 y clave B del emisor
    for (uint8_t s = 1; s <= 14 && ok; s++) {
        if (emissionBlocks[s] == 0x07) {
            ok = nfcWriteTrailer(s, getKeyAForSector(s), ACCESS_BITS, KEY_B_EMISOR);
            if (!ok) linkSendLog("No se pudo sellar el sector " + String(s));
        }
    }
#endif

    linkSendWritten(ok);
    nfcFinishCard();
    memset(emissionBlocks, 0, sizeof(emissionBlocks));
    hmiShowEmisionFin(ok);
    state = STATE_RESULT;
    stateTimer = millis();
}

void onGatewayCodeReceived(const String& code6) {
    if (linkGetModo() == MODE_LINKING) {
        hmiShowCodigo(code6);
        state = STATE_LINKING;
        stateTimer = millis();
    }
}

// ─── Función Central de Lectura e Inspección de Cédula ───────────────────────

// Valor del atributo como texto: A1/A2 valen 0x01; A3-A6 son UTF-8, que la pantalla
// convierte a su codificación. Se quitan los caracteres de control
static String valorComoTexto(const CedulaAttribute& attr) {
    String s = "";
    for (int i = 0; i < attr.largo && i < (int)sizeof(attr.valor); i++) {
        uint8_t ch = attr.valor[i];
        if (ch >= 32 && ch != 127) s += (char)ch;
    }
    return s;
}

// Handle al que se atribuye el resultado ante el backend: el del registro solo si su
// firma se verificó (01, 03 y 04 llegan después de la firma); si no (02, 05), el del índice
static const uint8_t* handleParaEvento(const char* resCode, const CedulaAttribute& attr,
                                       const uint8_t* handleIndice) {
    bool firmaVerificada = strcmp(resCode, RES_APROBADO) == 0 ||
                           strcmp(resCode, RES_REVOCADO) == 0 ||
                           strcmp(resCode, RES_VENCIDO) == 0;
    return firmaVerificada ? attr.handle : handleIndice;
}

void executeCedulaReading(const uint8_t* uid, uint8_t uidLen,
                          const CedulaAttribute& attr,
                          uint8_t atributoPedido,
                          bool authOk,
                          const uint8_t* handleIndice,
                          const char* titularNombre,
                          bool enviarEvento) {
    char uidBuf[32] = {0};
    for (int i = 0; i < uidLen; i++) {
        snprintf(uidBuf + strlen(uidBuf), sizeof(uidBuf) - strlen(uidBuf), "%02X%s", uid[i], (i < uidLen - 1) ? ":" : "");
    }

    String handleStr = handleToHex(attr.handle);
    String valorStr = "";
    if (attr.largo == 1 && attr.valor[0] == 0x01) {
        valorStr = "SI (Cumple condicion de mayoria de edad)";
    } else {
        valorStr = valorComoTexto(attr);
    }

    linkSendLog("============================================================");
    linkSendLog("[SOLOLOJUSTO] LECTURA DE CEDULA DE IDENTIDAD DIGITAL");
    linkSendLog("============================================================");
    linkSendLog("Titular : " + String(titularNombre));
    linkSendLog("UID NFC : " + String(uidBuf) + " (" + String(uidLen) + " bytes)");
    linkSendLog("Version : 0x" + String(attr.version, HEX) + " | Emisor: 0x" + String(attr.emisor, HEX) + " (Nacional)");
    linkSendLog("Atributo: A" + String(attr.atributo) + " (pedido: A" + String(atributoPedido) + ")");
    linkSendLog("Handle  : " + handleStr + " (Seudonimo de cedula anonimizada)");
    linkSendLog("Valor   : " + valorStr);
    linkSendLog("Vence   : " + String(attr.vence) + " (Unix timestamp)");
    linkSendLog("------------------------------------------------------------");

    // Medir tiempo de verificación criptográfica con microsegundos
    uint32_t tStart = micros();
    const char* resCode = verifyEvaluate(uid, uidLen, attr, atributoPedido, authOk, linkGetCurrentTime());
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

    // Salida hacia la pantalla Nextion del ciudadano
    bool esEmergencia = (linkGetModo() == MODE_EMERGENCY);
    hmiShowResultado(resCode, atributoPedido, esEmergencia);

    // Salida hacia el Arduino UNO del comercio (Serial1 en GPIO 25)
    linkSendUnoResult(resCode, atributoPedido, esEmergencia);

    // Evento JSON formal hacia el Gateway (USB Serial)
    if (enviarEvento) {
        const uint8_t* handleEvento = handleParaEvento(resCode, attr, handleIndice);
        linkSendVerif(handleToHex(handleEvento), atributoPedido, resCode, linkGetCurrentTime(), esEmergencia);
    }
}

void onGatewayCardSimulate(uint8_t cardIndex) {
    if (linkGetModo() != MODE_VERIFY) {
        linkSendLog("La simulacion de tarjeta solo funciona en modo verificar");
        return;
    }

    const TestVector* vec = &VECTOR_VALIDO;
    if (cardIndex == 1) vec = &VECTOR_ALTERADO;
    else if (cardIndex == 2) vec = &VECTOR_VENCIDO;

    currentUidLen = 4;
    memcpy(currentUid, vec->uid, 4);
    currentAttr = vec->attr;
    authSuccess = true;

    // Prueba de consola: pantallas y criptografía, sin enviar el evento al backend
    // (el handle de los vectores no es de una cédula emitida)
    executeCedulaReading(currentUid, currentUidLen, currentAttr, vec->attr.atributo, authSuccess,
                         vec->attr.handle, vec->nombre, false);

    state = STATE_RESULT;
    stateTimer = millis();
}

// Rechazo del ciudadano (06): solo se informa al backend si hay una cédula identificada.
// En el flujo normal se rechaza antes de acercar la cédula, así que no hay handle
static void enviarRechazo() {
    linkSendLog("Solicitud rechazada por el ciudadano (06)");
    hmiShowResultado(RES_RECHAZADO, linkGetAtributoPedido(), false);
    linkSendUnoResult(RES_RECHAZADO, linkGetAtributoPedido(), false);
    if (currentHandleOk) {
        linkSendVerif(handleToHex(currentHandle), linkGetAtributoPedido(), RES_RECHAZADO, linkGetCurrentTime(), false);
    }
}

// El ciudadano aceptó: recién ahora se espera la cédula, durante CONSENT_TIMEOUT_MS
static void aceptarSolicitud() {
    hmiShowAcerque(linkGetNombreLocal());
    linkSendLog("Solicitud aceptada: acerque la cedula (" + String(CONSENT_TIMEOUT_MS / 1000) + " s)");
    state = STATE_ACERQUE;
    stateTimer = millis();
}

void onGatewayConsentSimulate(bool approved) {
    // Sin la Nextion, la consola es la única forma de aceptar: decir por qué no aplica
    if (linkGetModo() != MODE_VERIFY) {
        linkSendLog("El consentimiento solo aplica en modo verificar");
        return;
    }
    if (state != STATE_SOLICITUD) {
        if (state == STATE_SIN_HORA) linkSendLog("Sin hora del gateway: todavia no hay solicitud que aceptar");
        else if (state == STATE_ACERQUE) linkSendLog("La solicitud ya fue aceptada: acerque la cedula");
        else linkSendLog("No hay una solicitud esperando respuesta");
        return;
    }
    if (approved) {
        aceptarSolicitud();
    } else {
        enviarRechazo();
        state = STATE_RESULT;
        stateTimer = millis();
    }
}

// ─── Lectura de la cédula presente ───────────────────────────────────────────

// Sin hora del gateway no se leen cédulas: la vigencia no se puede comprobar
static void avisarSinHora() {
    static uint32_t lastAviso = 0;
    if (lastAviso == 0 || millis() - lastAviso >= 10000) {
        lastAviso = millis();
        linkSendLog("Sin hora del gateway: no se leen cedulas hasta recibir {\"t\":\"time\"}");
    }
}

// Lee el índice y un atributo de la tarjeta presente. false = no es una cédula o la
// lectura quedó incompleta (se retiró antes de tiempo): no hay resultado que mostrar
static bool leerCedula(uint8_t atributo) {
    currentHandleOk = nfcReadHandle(currentHandle);
    if (!currentHandleOk) {
        linkSendLog("[NFC] La tarjeta no tiene indice de cedula");
        return false;
    }
    if (!nfcReadAttribute(atributo, linkGetRol(), currentAttr, authSuccess)) {
        linkSendLog("[NFC] Lectura incompleta: acerque la cedula de nuevo");
        return false;
    }
    return true;
}

static void mostrarLecturaFallida() {
    hmiShowAviso("No se pudo leer la cédula", "Acérquela de nuevo y no la retire");
    state = STATE_RESULT;
    stateTimer = millis();
}

// ─── FSM: Modo Verificación ──────────────────────────────────────────────────
// Reposo = solicitud: "{local} pide: {dato}". El ciudadano acepta y recién entonces
// acerca la cédula (roadmap, flujo de verificación); nada se lee antes de aceptar.

static void handleVerifyMode(HmiTouchEvent touch) {
    switch (state) {
        case STATE_IDLE: {
            if (!linkHasTimeSync()) {
                hmiShowSinConexion();
                state = STATE_SIN_HORA;
                break;
            }
            hmiShowSolicitud(linkGetNombreLocal(), linkGetAtributoPedido());
            linkSendLog(linkGetNombreLocal() + " pide: " + nombreAtributo(linkGetAtributoPedido()) +
                        ". Esperando que el ciudadano acepte (consola: 'a' acepta, 'r' rechaza)");
            state = STATE_SOLICITUD;
            break;
        }

        case STATE_SIN_HORA: {
            if (linkHasTimeSync()) state = STATE_IDLE;
            else avisarSinHora();
            break;
        }

        case STATE_SOLICITUD: {
            if (touch == TOUCH_ACEPTAR && linkGetAtributoPedido() != 0) {
                aceptarSolicitud();
            } else if (touch == TOUCH_RECHAZAR) {
                // Rechazado por el ciudadano -> Código 06
                enviarRechazo();
                state = STATE_RESULT;
                stateTimer = millis();
            }
            break;
        }

        case STATE_ACERQUE: {
            // Cancelar o no acercar la cédula a tiempo vuelve a la solicitud, sin evento
            if (touch == TOUCH_CANCELAR || millis() - stateTimer >= CONSENT_TIMEOUT_MS) {
                linkSendLog(touch == TOUCH_CANCELAR ? "Lectura cancelada" : "No se acerco ninguna tarjeta a tiempo");
                state = STATE_IDLE;
                break;
            }

            // Sondeo de tarjeta en el RC522
            if (nfcDetectCard(currentUid, &currentUidLen)) {
                char uidHex[32] = {0};
                for (int i = 0; i < currentUidLen; i++) {
                    snprintf(uidHex + strlen(uidHex), sizeof(uidHex) - strlen(uidHex), "%02X%s", currentUid[i], (i < currentUidLen - 1) ? ":" : "");
                }
                linkSendLog("[NFC FISICO] Tarjeta fisica detectada en lector RC522! UID: " + String(uidHex));
                hmiShowVerificando(linkGetNombreLocal());

                // Leer el índice y el atributo pedido con la clave A del rol
                uint8_t pedido = linkGetAtributoPedido();
                if (leerCedula(pedido)) {
                    executeCedulaReading(currentUid, currentUidLen, currentAttr, pedido, authSuccess,
                                         currentHandle, "Cedula Fisica Leida", true);
                    state = STATE_RESULT;
                    stateTimer = millis();
                } else {
                    mostrarLecturaFallida();
                }
                nfcFinishCard();
            }
            break;
        }

        case STATE_RESULT: {
            if (millis() - stateTimer >= RESULT_DISPLAY_MS) {
                currentHandleOk = false;
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
            if (!linkHasTimeSync()) {
                hmiShowSinConexion();
                state = STATE_SIN_HORA;
                break;
            }
            hmiShowEmergenciaEspera();
            state = STATE_ACERQUE;
            break;
        }

        case STATE_SIN_HORA: {
            if (linkHasTimeSync()) state = STATE_IDLE;
            else avisarSinHora();
            break;
        }

        case STATE_ACERQUE: {
            if (!nfcDetectCard(currentUid, &currentUidLen)) break;
            hmiShowVerificando("Modo emergencia");

            // En modo emergencia se lee directamente, sin consentimiento, con la clave del rol
            if (!leerCedula(ATTR_A4_SALUD)) {
                nfcFinishCard();
                mostrarLecturaFallida();
                break;
            }

            const char* resCode = verifyEvaluate(currentUid, currentUidLen, currentAttr, ATTR_A4_SALUD,
                                                 authSuccess, linkGetCurrentTime());
            bool aprobado = (strcmp(resCode, RES_APROBADO) == 0);

            if (aprobado) {
                // A4 es "<sangre> <alergias>" (p. ej. "O+ penicilina"); el contacto es A5
                String a4 = valorComoTexto(currentAttr);
                int espacio = a4.indexOf(' ');
                String sangre = (espacio < 0) ? a4 : a4.substring(0, espacio);
                String alergias = (espacio < 0) ? "" : a4.substring(espacio + 1);

                String contacto = "";
                CedulaAttribute a5;
                bool a5Auth = false;
                if (nfcReadAttribute(ATTR_A5_CONTACTO, linkGetRol(), a5, a5Auth) &&
                    strcmp(verifyEvaluate(currentUid, currentUidLen, a5, ATTR_A5_CONTACTO, a5Auth,
                                          linkGetCurrentTime()), RES_APROBADO) == 0) {
                    contacto = valorComoTexto(a5);
                }

                hmiShowEmergencia(sangre, alergias, contacto);
                state = STATE_EMERGENCY;
            } else {
                hmiShowResultado(resCode, ATTR_A4_SALUD, true);
                state = STATE_RESULT;
            }

            linkSendUnoResult(resCode, ATTR_A4_SALUD, true);
            linkSendVerif(handleToHex(handleParaEvento(resCode, currentAttr, currentHandle)), ATTR_A4_SALUD,
                          resCode, linkGetCurrentTime(), true);

            nfcFinishCard();
            stateTimer = millis();
            break;
        }

        case STATE_EMERGENCY: {
            if (millis() - stateTimer >= EMERGENCY_DISPLAY_MS) {
                currentHandleOk = false;
                state = STATE_IDLE;
            }
            break;
        }

        case STATE_RESULT: {
            if (millis() - stateTimer >= RESULT_DISPLAY_MS) {
                currentHandleOk = false;
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
    switch (state) {
        case STATE_IDLE: {
            hmiShowEmisionEspera();
            state = STATE_ACERQUE;
            break;
        }

        case STATE_ACERQUE: {
            if (!nfcDetectCard(currentUid, &currentUidLen)) break;
            char uidHex[21] = "";
            for (uint8_t i = 0; i < currentUidLen; i++) {
                sprintf(uidHex + strlen(uidHex), "%02X", currentUid[i]);
            }
            memset(emissionBlocks, 0, sizeof(emissionBlocks));
            linkSendUid(String(uidHex));
            hmiShowEmisionGrabando();
            state = STATE_EMISSION;
            stateTimer = millis();
            break;
        }

        case STATE_EMISSION: {
            // En espera de comandos 'write' y 'seal' del gateway
            if (millis() - stateTimer >= EMISSION_IDLE_MS) {
                linkSendLog("Emision abortada: 30 s sin bloques del gateway");
                nfcFinishCard();
                memset(emissionBlocks, 0, sizeof(emissionBlocks));
                hmiShowEmisionFin(false);
                state = STATE_RESULT;
                stateTimer = millis();
            }
            break;
        }

        case STATE_RESULT: {
            if (millis() - stateTimer >= RESULT_DISPLAY_MS) state = STATE_IDLE;
            break;
        }

        default:
            break;
    }
}

// ─── FSM: Modo Vinculación ───────────────────────────────────────────────────

static void handleLinkingMode() {
    switch (state) {
        case STATE_IDLE: {
            hmiShowVinculacionEspera();
            state = STATE_ACERQUE;
            break;
        }

        case STATE_ACERQUE: {
            if (!nfcDetectCard(currentUid, &currentUidLen)) break;
            uint8_t handle8[8];
            if (nfcReadHandle(handle8)) {
                linkSendLink(handleToHex(handle8));
                hmiShowAviso("Pidiendo el código...", "Ya puede retirar la cédula");
                state = STATE_READING;
                stateTimer = millis();
            } else {
                linkSendLog("[NFC] La tarjeta no tiene indice de cedula");
                mostrarLecturaFallida();
            }
            nfcFinishCard();
            break;
        }

        case STATE_READING: {
            // Esperando el código que el gateway pide a la API
            if (millis() - stateTimer >= CODE_WAIT_MS) {
                hmiShowAviso("No llegó el código", "Revise la conexión del terminal");
                state = STATE_RESULT;
                stateTimer = millis();
            }
            break;
        }

        case STATE_LINKING: {
            // Mostrando código de 6 dígitos durante 2 min
            if (millis() - stateTimer >= LINK_CODE_DISPLAY_MS) state = STATE_IDLE;
            break;
        }

        case STATE_RESULT: {
            if (millis() - stateTimer >= RESULT_DISPLAY_MS) state = STATE_IDLE;
            break;
        }

        default:
            break;
    }
}

// ─── Menú oculto ─────────────────────────────────────────────────────────────

// El rol real está atado al token del gateway: el menú le pide el terminal de ese rol
// y espera su cfg, que reinicia la FSM con la configuración nueva
static void atenderMenu(HmiTouchEvent touch) {
    const char* rol = nullptr;
    switch (touch) {
        case TOUCH_MENU_COMERCIO:   rol = "comercio"; break;
        case TOUCH_MENU_SALUD:      rol = "salud"; break;
        case TOUCH_MENU_EMERGENCIA: rol = "emergencia"; break;
        case TOUCH_MENU_TRANSITO:   rol = "transito"; break;
        case TOUCH_MENU_REGISTRO:   rol = "registro"; break;
        case TOUCH_MENU_VOLVER:
            menuAbierto = false;
            reiniciarFsm();
            return;
        default:
            if (millis() - menuTimer >= MENU_TIMEOUT_MS) {
                menuAbierto = false;
                reiniciarFsm();
            }
            return;
    }

    linkSendRol(rol);
    hmiShowAviso("Cambiando de terminal...", "Rol pedido al gateway: " + String(rol));
    menuAbierto = false;
    esperandoRol = true;
    menuTimer = millis();
}

// ─── Diagnóstico del lector ──────────────────────────────────────────────────
// 'scan' en la consola prueba el RC522 y la tarjeta sin hora ni consentimiento:
// UID, tipo, qué clave abre cada sector y el índice. No lee los atributos.

void onGatewayScan() {
    if (state == STATE_EMISSION) {
        linkSendLog("[SCAN] Hay una emision en curso: espere a que termine");
        return;
    }

    uint8_t version = nfcProbeChip();
    String v = "0x" + String(version, HEX);
    if (!nfcIsHardwareDetected()) {
        linkSendLog("[SCAN] El RC522 no responde (version " + v + "): revisar 3,3 V, GND y SPI (SS 5, SCK 18, MISO 19, MOSI 23, RST 4)");
        return;
    }

    linkSendLog("[SCAN] RC522 responde (version " + v + "). Acerque la tarjeta (" + String(SCAN_TIMEOUT_MS / 1000) + " s)");
    nfcFinishCard();
    scanActivo = true;
    scanTimer = millis();
}

static void imprimirDiagnostico(const NfcDiagnostico& d) {
    String uid = "";
    for (uint8_t i = 0; i < d.uidLen; i++) {
        char hex[4];
        snprintf(hex, sizeof(hex), "%02X%s", d.uid[i], (i < d.uidLen - 1) ? ":" : "");
        uid += hex;
    }
    char sak[8];
    snprintf(sak, sizeof(sak), "0x%02X", d.sak);
    linkSendLog("[SCAN] Tarjeta: UID " + uid + " (" + String(d.uidLen) + " B), SAK " + String(sak) + ", " + String(d.tipo));

    if (!d.clasica) {
        linkSendLog("[SCAN] No es MIFARE Classic: el lector la ve, pero no tiene sectores con clave. No sirve como cedula");
        return;
    }

    // Una letra por sector: F = clave de fábrica, R = clave del rol, x = no abre, ? = sin probar
    String mapa = "";
    bool fabrica = true, retirada = false, desconocida = false;
    for (uint8_t s = 1; s <= 14; s++) {
        char c = '?';
        switch (d.sector[s]) {
            case SECTOR_FABRICA: c = 'F'; break;
            case SECTOR_ROL:     c = 'R'; fabrica = false; break;
            case SECTOR_NO_ABRE: c = 'x'; fabrica = false; desconocida = true; break;
            default:             retirada = true; break;
        }
        mapa += String(s) + ":" + c + " ";
    }
    linkSendLog("[SCAN] Sectores (F fabrica, R clave del rol, x no abre): " + mapa);

    bool esCedula = d.indiceLeido && d.bloque4[0] == 0x01;
    if (d.indiceLeido) {
        String hex = "";
        for (uint8_t i = 0; i < 16; i++) {
            char b[3];
            snprintf(b, sizeof(b), "%02X", d.bloque4[i]);
            hex += b;
        }
        String linea = "[SCAN] Indice (bloque 4): " + hex;
        if (esCedula) linea += " -> handle " + handleToHex(d.bloque4 + 1);
        linkSendLog(linea);
    }

    if (retirada) {
        linkSendLog("[SCAN] La tarjeta se retiro antes de terminar: mantengala quieta y repita 'scan'");
    } else if (esCedula) {
        linkSendLog(fabrica ? "[SCAN] Cedula emitida pero sin sellar (trailers de fabrica)"
                            : "[SCAN] Cedula emitida. En modo verificar: hora, 'a' y acercarla antes de 10 s");
    } else if (fabrica) {
        linkSendLog("[SCAN] Tarjeta virgen: en modo verificar da 'no tiene indice'. Se graba en modo emitir");
    } else if (desconocida) {
        linkSendLog("[SCAN] Hay sectores con una clave desconocida: tarjeta de otro sistema o con un trailer mal grabado");
    } else {
        linkSendLog("[SCAN] Sectores sellados, pero el indice no es de una cedula");
    }
}

static void atenderScan() {
    NfcDiagnostico d;
    if (nfcDiagnose(d)) {
        imprimirDiagnostico(d);
    } else if (millis() - scanTimer < SCAN_TIMEOUT_MS) {
        return;
    } else {
        linkSendLog("[SCAN] Ninguna tarjeta en " + String(SCAN_TIMEOUT_MS / 1000) +
                    " s. A 0-3 cm de la antena; las de 125 kHz el RC522 no las lee");
    }
    scanActivo = false;
    reiniciarFsm();
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
    bool rc522Ok = nfcInit();
    String version = " (version 0x" + String(nfcGetVersion(), HEX) + ")";
    if (!rc522Ok) {
        linkSendLog("RC522 no detectado o cableado SPI incorrecto" + version);
    } else {
        linkSendLog("RC522 inicializado correctamente" + version);
    }

    lastHbTimer = millis();

    // Estado inicial hacia el Arduino UNO (Comercio): {"t":"idle"} -> "sIsA / Esperando cédula...".
    // Si el PIC ya está bloqueando, el primer guardUpdate() lo detecta y envía {"t":"lock"}
    linkSendUnoIdle();

    linkSendLog("=== Terminal SoloLoJusto listo ===");
    linkSendLog("Tip: escribe 'ayuda' en la terminal para ver los comandos de prueba.");
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

    // 4. Configuración nueva del gateway: si cambió el rol, el dato, el local o el modo
    // (o se esperaba la respuesta del menú oculto), la FSM vuelve a reposo y redibuja
    static uint32_t lastCfg = 0;
    static String cfgRol, cfgLocal;
    static uint8_t cfgAttr = 0;
    static TerminalMode cfgModo = MODE_VERIFY;
    if (linkGetCfgVersion() != lastCfg) {
        lastCfg = linkGetCfgVersion();
        bool cambio = cfgRol != linkGetRol() || cfgLocal != linkGetNombreLocal() ||
                      cfgAttr != linkGetAtributoPedido() || cfgModo != linkGetModo();
        cfgRol = linkGetRol();
        cfgLocal = linkGetNombreLocal();
        cfgAttr = linkGetAtributoPedido();
        cfgModo = linkGetModo();
        if (cambio || esperandoRol) {
            menuAbierto = false;
            esperandoRol = false;
            reiniciarFsm();
        }
    }

    // Diagnóstico del lector en curso: la FSM retoma desde reposo al terminar
    if (scanActivo) {
        atenderScan();
        return;
    }

    // 5. Pantalla táctil: un evento por vuelta, para el menú oculto o para la FSM
    HmiTouchEvent touch = hmiCheckTouch();
    if (touch == TOUCH_REINICIO && state != STATE_EMISSION && state != STATE_READING) {
        // La Nextion se reinició por su cuenta: redibujar desde reposo
        menuAbierto = false;
        reiniciarFsm();
    }
    if (menuAbierto) {
        atenderMenu(touch);
        return;
    }
    if (esperandoRol) {
        if (millis() - menuTimer >= ROL_TIMEOUT_MS) {
            linkSendLog("El gateway no respondió al cambio de terminal");
            esperandoRol = false;
            reiniciarFsm();
        }
        return;
    }
    if (touch == TOUCH_MENU_HIDDEN) {
        nfcFinishCard();
        menuAbierto = true;
        menuTimer = millis();
        hmiShowMenuOculto();
        return;
    }

    // 6. Ejecutar la FSM según el modo activo
    switch (linkGetModo()) {
        case MODE_VERIFY:
            handleVerifyMode(touch);
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

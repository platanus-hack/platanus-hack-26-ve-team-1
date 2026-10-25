// =============================================================================
// src/link.cpp — Implementación del protocolo de comunicación con Gateway y UNO
// =============================================================================

#include "link.h"
#include "store.h"
#include <ArduinoJson.h>

// Configuración activa (rol con los nombres del backend: comercio, registro, salud, transito, emergencia)
static String currentRol = "comercio";
static uint8_t currentAttr = ATTR_A1_MAYOR_18;
static String currentLocal = "Bar El Rincon";
static TerminalMode currentModo = MODE_VERIFY;

// Reloj interno sincronizado por Gateway
static uint32_t baseTimestamp = 0;
static uint32_t syncLocalMillis = 0;
static bool timeSynchronized = false;

// Búfer de recepción Serial (máximo 512 B según especificación)
static char rxBuffer[512];
static size_t rxIndex = 0;

// Sincronización en curso de la lista de revocación (trozos "rev" con more:true)
static bool revSyncing = false;
static uint32_t lastRevMillis = 0;

// Cantidad de cfg recibidas del gateway
static uint32_t cfgVersion = 0;

// Callbacks externos para eventos del Gateway
extern void onGatewayWriteBlock(uint8_t sector, uint8_t bloque, const String& hexData);
extern void onGatewaySeal();
extern void onGatewayCodeReceived(const String& code6);
extern void onGatewayCardSimulate(uint8_t cardIndex);
extern void onGatewayConsent(bool approved);
extern void onGatewayScan();

void linkInit() {
    Serial.setRxBufferSize(1024);
    Serial.begin(GATEWAY_BAUD);

    // Serial1 hacia Arduino UNO (TX: 25, RX: 35)
    Serial1.begin(UNO_BAUD, SERIAL_8N1, PIN_UNO_RX, PIN_UNO_TX);
}

String linkGetRol() { return currentRol; }
uint8_t linkGetAtributoPedido() { return currentAttr; }
String linkGetNombreLocal() { return currentLocal; }
TerminalMode linkGetModo() { return currentModo; }
bool linkHasTimeSync() { return timeSynchronized; }
uint32_t linkGetCfgVersion() { return cfgVersion; }

// "A1".."A6" -> 1..6; "" (el terminal no pide un atributo concreto) u otro valor -> 0
static uint8_t atributoDesdeTexto(const char* a) {
    if (a != nullptr && a[0] == 'A' && a[1] >= '1' && a[1] <= '6' && a[2] == '\0') {
        return a[1] - '0';
    }
    return 0;
}

uint32_t linkGetCurrentTime() {
    if (!timeSynchronized) return 0;
    return baseTimestamp + ((millis() - syncLocalMillis) / 1000);
}

// ─── Envíos ESP32 → Gateway ──────────────────────────────────────────────────

void linkSendHb() {
    // Sin hora (recién arrancado) el latido lo avisa y el gateway reenvía time, rev y cfg
    Serial.println(timeSynchronized ? "{\"t\":\"hb\"}" : "{\"t\":\"hb\",\"sync\":false}");
}

void linkSendVerif(const String& handleHex, uint8_t atributo, const char* resCode, uint32_t timestamp, bool emergencia) {
    // El backend exige el atributo como "A1".."A6"
    char attrStr[8];
    snprintf(attrStr, sizeof(attrStr), "A%u", atributo);

    JsonDocument doc;
    doc["t"] = "verif";
    doc["h"] = handleHex;
    doc["a"] = attrStr;
    doc["res"] = resCode;
    doc["ts"] = timestamp;
    doc["emg"] = emergencia;
    serializeJson(doc, Serial);
    Serial.println();
}

void linkSendTamper() {
    Serial.println("{\"t\":\"tamper\"}");
}

void linkSendUid(const String& uidHex) {
    JsonDocument doc;
    doc["t"] = "uid";
    doc["uid"] = uidHex;
    serializeJson(doc, Serial);
    Serial.println();
}

void linkSendAck(uint8_t sector, uint8_t bloque, bool ok) {
    JsonDocument doc;
    doc["t"] = "ack";
    doc["s"] = sector;
    doc["b"] = bloque;
    doc["ok"] = ok;
    serializeJson(doc, Serial);
    Serial.println();
}

void linkSendWritten(bool ok) {
    JsonDocument doc;
    doc["t"] = "written";
    doc["ok"] = ok;
    serializeJson(doc, Serial);
    Serial.println();
}

void linkSendLink(const String& handleHex) {
    JsonDocument doc;
    doc["t"] = "link";
    doc["h"] = handleHex;
    serializeJson(doc, Serial);
    Serial.println();
}

void linkSendLog(const String& mensaje) {
    JsonDocument doc;
    doc["t"] = "log";
    doc["m"] = mensaje;
    serializeJson(doc, Serial);
    Serial.println();
}

void linkSendUnoResult(const char* resCode, uint8_t atributo, bool emergencia) {
    // Regla de P3: No mandar códigos 07 ni 08 (el UNO los muestra como "Error de lectura")
    const char* safeRes = resCode;
    if (strcmp(resCode, "07") == 0 || strcmp(resCode, "08") == 0) {
        safeRes = RES_RECHAZADO; // "06"
    }

    // Formatear atributo como "A1", "A2", etc.
    char attrStr[8];
    snprintf(attrStr, sizeof(attrStr), "A%u", atributo);

    // Contrato exacto para el Arduino UNO: {"t":"res","res":"01","a":"A1","emg":false}
    JsonDocument doc;
    doc["t"] = "res";
    doc["res"] = safeRes;
    doc["a"] = attrStr;
    doc["emg"] = emergencia;
    serializeJson(doc, Serial1);
    Serial1.println();
}

void linkSendUnoLock() {
    // Contrato Arduino UNO: {"t":"lock"} ("TERMINAL BLOQUEADO" en rojo)
    JsonDocument doc;
    doc["t"] = "lock";
    serializeJson(doc, Serial1);
    Serial1.println();
}

void linkSendUnoIdle() {
    // Contrato Arduino UNO: {"t":"idle"} ("sIsA / Esperando cédula...")
    JsonDocument doc;
    doc["t"] = "idle";
    serializeJson(doc, Serial1);
    Serial1.println();
}

// ─── Recepción Gateway → ESP32 ──────────────────────────────────────────────

static void processJsonLine(const char* line) {
    // Si no empieza por '{', procesar como comando rápido de consola para el operador humano
    if (line[0] != '{') {
        String s = String(line);
        s.trim();
        s.toLowerCase();

        if (s == "tap" || s == "leer" || s == "1" || s == "ana" || s == "cedula" || s == "valida") {
            onGatewayCardSimulate(0); // Vector 0: Ana Perez (Valida Mayor 18)
            return;
        } else if (s == "2" || s == "alterada" || s == "falsa") {
            onGatewayCardSimulate(1); // Vector 1: Tarjeta alterada (Firma 02)
            return;
        } else if (s == "3" || s == "vencida") {
            onGatewayCardSimulate(2); // Vector 2: Tarjeta vencida (04)
            return;
        } else if (s == "a" || s == "acepto" || s == "aceptar") {
            onGatewayConsent(true);   // El ciudadano acepta la solicitud
            return;
        } else if (s == "r" || s == "rechazo" || s == "rechazar") {
            onGatewayConsent(false);  // El ciudadano la rechaza (06)
            return;
        } else if (s == "scan" || s == "nfc") {
            onGatewayScan();  // Diagnóstico del RC522 y de la tarjeta, en cualquier modo
            return;
        } else if (s == "help" || s == "ayuda" || s == "?") {
            linkSendLog("=== Comandos de Terminal ===");
            linkSendLog("  Antes de probar vectores: {\"t\":\"time\",\"ts\":<hora Unix>} (sin hora da 04)");
            linkSendLog("  '1' o 'leer'     -> Leer cedula oficial (Ana - Mayor 18)");
            linkSendLog("  '2' o 'alterada' -> Probar cedula manipulada (Firma falsa 02)");
            linkSendLog("  '3' o 'vencida'  -> Probar cedula vencida (04)");
            linkSendLog("  'a' / 'r'        -> Aceptar / rechazar la solicitud");
            linkSendLog("  Tarjeta fisica: con hora, 'a' y apoyarla antes de 10 s");
            linkSendLog("  'scan'           -> Probar el lector y ver que tiene la tarjeta (sin hora ni 'a')");
            return;
        }

        linkSendLog("Comando '" + s + "' no reconocido. Escribe '1' o 'leer' para probar la cedula.");
        return;
    }

    JsonDocument doc;
    DeserializationError err = deserializeJson(doc, line);
    if (err) {
        linkSendLog("JSON invalido recibido");
        return;
    }

    const char* t = doc["t"] | "";

    // 1. time: Ajusta reloj Unix
    if (strcmp(t, "time") == 0) {
        if (!doc["ts"].isNull()) {
            baseTimestamp = doc["ts"].as<uint32_t>();
            syncLocalMillis = millis();
            timeSynchronized = true;
            linkSendLog("Reloj sincronizado");
        }
    }
    // 2. rev: Lista de revocación completa, en trozos; more:false en el último.
    // Un trozo que llega más de 5 s después del anterior empieza una lista nueva
    // (la sincronización previa quedó cortada).
    else if (strcmp(t, "rev") == 0) {
        if (!revSyncing || millis() - lastRevMillis > 5000) {
            storeBeginRevocationSync();
            revSyncing = true;
        }
        lastRevMillis = millis();

        JsonArray arr = doc["hs"].as<JsonArray>();
        for (const char* hHex : arr) {
            uint8_t hBytes[8];
            if (hHex != nullptr && hexToHandle(hHex, hBytes)) {
                storeStageRevocation(hBytes);
            }
        }

        bool more = doc["more"] | false;
        if (!more) {
            storeCommitRevocationSync();
            revSyncing = false;
            linkSendLog("Lista revocacion actualizada: " + String(storeGetRevocationCount()) + " handles");
        }
    }
    // 3. cfg: Configuración de rol y modo
    else if (strcmp(t, "cfg") == 0) {
        if (!doc["rol"].isNull()) currentRol = doc["rol"].as<String>();
        if (!doc["a"].isNull()) currentAttr = atributoDesdeTexto(doc["a"] | "");
        if (!doc["local"].isNull()) currentLocal = doc["local"].as<String>();

        const char* modoStr = doc["modo"] | "verificar";
        if (strcmp(modoStr, "emergencia") == 0) currentModo = MODE_EMERGENCY;
        else if (strcmp(modoStr, "emitir") == 0) currentModo = MODE_EMISSION;
        else if (strcmp(modoStr, "vincular") == 0) currentModo = MODE_LINKING;
        else currentModo = MODE_VERIFY;

        cfgVersion++;
        linkSendLog("Configuracion aplicada");
    }
    // 4. write: Modo emitir, un bloque de datos (s = sector 1..14, b = bloque 0..2 del sector)
    else if (strcmp(t, "write") == 0) {
        uint8_t s = doc["s"] | 0;
        uint8_t b = doc["b"] | 0;
        String d = doc["d"] | "";
        onGatewayWriteBlock(s, b, d);
    }
    // 5. seal: Modo emitir, ya llegaron todos los bloques: grabar trailers y responder written
    else if (strcmp(t, "seal") == 0) {
        onGatewaySeal();
    }
    // 6. code: Código de vinculación
    else if (strcmp(t, "code") == 0) {
        String c = doc["c"] | "";
        onGatewayCodeReceived(c);
    }
    // 7. tap: Simulación de tarjeta desde consola
    else if (strcmp(t, "tap") == 0) {
        uint8_t card = doc["card"] | 0;
        onGatewayCardSimulate(card);
    }
    // 8. consent: Respuesta del ciudadano a la solicitud
    else if (strcmp(t, "consent") == 0) {
        bool app = doc["approved"] | true;
        onGatewayConsent(app);
    }
}

void linkProcessInput() {
    while (Serial.available()) {
        char c = (char)Serial.read();
        // Aceptar tanto \n como \r para máxima compatibilidad con todos los monitores
        if (c == '\n' || c == '\r') {
            rxBuffer[rxIndex] = '\0';
            if (rxIndex > 0) {
                processJsonLine(rxBuffer);
            }
            rxIndex = 0;
        } else {
            if (rxIndex < sizeof(rxBuffer) - 1) {
                rxBuffer[rxIndex++] = c;
            } else {
                rxIndex = 0;
                linkSendLog("Buffer de entrada desbordado (>512 B)");
            }
        }
    }
}

// =============================================================================
// rc522-diag — Prueba aislada del RC522 en el ESP32 (mismos pines que firmware/esp32)
// Sin gateway, hora ni consentimiento: solo el lector y la tarjeta.
// Monitor a 115200. Comandos: 'r' reinicia el lector y repite la prueba del bus,
// 'g' alterna la ganancia de recepción entre la de fábrica (33 dB) y la máxima (48 dB).
// =============================================================================

#include <Arduino.h>
#include <SPI.h>
#include <MFRC522.h>

#define PIN_RC522_SS    5
#define PIN_RC522_RST   4
#define PIN_RC522_SCK  18
#define PIN_RC522_MISO 19
#define PIN_RC522_MOSI 23
#define PIN_PIC_HB     26   // Latido al PIC: sin flancos, a los 10 s le corta la corriente al RC522
#define PIN_PIC_BLOQ   27   // Desde el PIC (drenador abierto): BAJO = normal, ALTO = bloqueado o sin PIC

static MFRC522 rfid(PIN_RC522_SS, PIN_RC522_RST);

static bool busOk = false;          // versión válida y escritura/relectura correcta
static byte versionPrevia = 0x00;   // última lectura del registro de versión
static bool gananciaMax = false;
static int bloqPrevio = -1;

static bool versionValida(byte v) {
    return v != 0x00 && v != 0xFF;
}

static const char* nombreVersion(byte v) {
    switch (v) {
        case 0x91: return "NXP MFRC522 v1.0";
        case 0x92: return "NXP MFRC522 v2.0";
        case 0x88: return "clon FM17522 (funciona)";
        case 0x12:
        case 0xB2: return "clon (suele funcionar)";
        case 0x00:
        case 0xFF: return "SIN RESPUESTA";
        default:   return "valor raro (clon desconocido o MISO al aire)";
    }
}

static uint8_t gananciaDb() {
    static const uint8_t db[] = {18, 23, 18, 23, 33, 38, 43, 48};
    return db[(rfid.PCD_GetAntennaGain() >> 4) & 0x07];
}

// Lee el registro de versión 10 veces (camino SCK/MISO/SS) y escribe y relee un
// registro inofensivo (camino MOSI). Deja el chip recién inicializado
static bool probarBus() {
    rfid.PCD_Init();
    byte v = rfid.PCD_ReadRegister(MFRC522::VersionReg);
    bool estable = true;
    for (uint8_t i = 0; i < 9; i++) {
        if (rfid.PCD_ReadRegister(MFRC522::VersionReg) != v) estable = false;
    }
    versionPrevia = v;
    Serial.printf("[1] Registro de version: 0x%02X -> %s\n", v, nombreVersion(v));
    if (!estable) {
        Serial.println("    CAMBIA entre lecturas: contacto flojo en SCK, MISO, SS o GND");
    }
    if (!versionValida(v)) {
        Serial.println("    El chip no contesta por SPI. Revisar en este orden:");
        Serial.println("    - Pin 3.3V del RC522 con el multimetro: 3,0-3,4 V. GND comun con el ESP32");
        Serial.println("    - Tira de pines del modulo SOLDADA (insertada a presion no hace contacto)");
        Serial.println("    - SDA->GPIO5  SCK->GPIO18  MOSI->GPIO23  MISO->GPIO19  RST->GPIO4  (MOSI/MISO sin cruzar)");
        return false;
    }

    // TReloadRegL es del temporizador: PCD_Init lo vuelve a dejar como estaba
    const byte patrones[] = {0x55, 0xAA, 0x3C};
    bool escribe = true;
    for (byte p : patrones) {
        rfid.PCD_WriteRegister(MFRC522::TReloadRegL, p);
        if (rfid.PCD_ReadRegister(MFRC522::TReloadRegL) != p) escribe = false;
    }
    rfid.PCD_Init();
    Serial.printf("[2] Escritura y relectura de un registro: %s\n",
                  escribe ? "OK (MOSI funciona)" : "FALLA: revisar MOSI->GPIO23");
    if (!escribe) return false;

    rfid.PCD_SetAntennaGain(gananciaMax ? MFRC522::RxGain_max : MFRC522::RxGain_33dB);
    bool antena = (rfid.PCD_ReadRegister(MFRC522::TxControlReg) & 0x03) == 0x03;
    Serial.printf("[3] Antena %s, ganancia de recepcion %u dB\n", antena ? "encendida" : "APAGADA", gananciaDb());
    Serial.println("    Mida AHORA el pin 3.3V del RC522 contra GND: 3,0-3,4 V.");
    Serial.println("    Menos de 2,5 V = alimentacion insuficiente (el LED prende igual, pero no hay campo)");
    Serial.println("[4] Apoye la tarjeta plana sobre la antena (0-2 cm)...");
    return true;
}

// Informa cada cambio de BLOQ. Sin el PIC conectado a GPIO27, el pull-up la deja en ALTO
static void revisarPic() {
    int bloq = digitalRead(PIN_PIC_BLOQ);
    if (bloq == bloqPrevio) return;
    bloqPrevio = bloq;
    if (bloq == HIGH) {
        Serial.println("[PIC] BLOQ en ALTO: el PIC esta bloqueando, o no esta cableado a GPIO27.");
        Serial.println("      Si el RC522 se alimenta por Q1, esta SIN corriente aunque su LED brille");
        Serial.println("      (lo alimentan SS y RST del ESP32). En la Curiosity Nano: LED encendido = bloqueado.");
        Serial.println("      Precinto cerrado (RA5 a GND) y pulsar REARME (RA2 a GND), o resetear el PIC.");
        Serial.println("      Con el RC522 directo al 3V3 del ESP32, ignore este aviso.");
    } else {
        Serial.println("[PIC] BLOQ en BAJO: el PIC deja el lector encendido");
    }
}

static void leerTarjeta() {
    if (!rfid.PICC_IsNewCardPresent()) return;
    if (!rfid.PICC_ReadCardSerial()) {
        Serial.println("[4] Algo respondio, pero el UID llego incompleto: tarjeta plana, quieta y mas cerca");
        return;
    }

    Serial.print("[4] TARJETA LEIDA. UID:");
    for (byte i = 0; i < rfid.uid.size; i++) Serial.printf(" %02X", rfid.uid.uidByte[i]);
    MFRC522::PICC_Type tipo = MFRC522::PICC_GetType(rfid.uid.sak);
    Serial.printf(" | SAK 0x%02X | ", rfid.uid.sak);
    Serial.println(MFRC522::PICC_GetTypeName(tipo));

    bool clasica = tipo == MFRC522::PICC_TYPE_MIFARE_MINI || tipo == MFRC522::PICC_TYPE_MIFARE_1K ||
                   tipo == MFRC522::PICC_TYPE_MIFARE_4K;
    if (!clasica) {
        Serial.println("    No es MIFARE Classic: el lector funciona, pero esta tarjeta no sirve como cedula");
    } else {
        // Solo lectura: bloque 4 (sector 1) con la clave A de fábrica
        MFRC522::MIFARE_Key key;
        memset(key.keyByte, 0xFF, sizeof(key.keyByte));
        MFRC522::StatusCode st = rfid.PCD_Authenticate(MFRC522::PICC_CMD_MF_AUTH_KEY_A, 7, &key, &rfid.uid);
        if (st != MFRC522::STATUS_OK) {
            Serial.print("    El sector 1 no abre con la clave de fabrica (");
            Serial.print(MFRC522::GetStatusCodeName(st));
            Serial.println("): normal en una cedula ya sellada. El lector funciona");
        } else {
            byte buf[18];
            byte len = sizeof(buf);
            st = rfid.MIFARE_Read(4, buf, &len);
            if (st == MFRC522::STATUS_OK) {
                Serial.print("    Bloque 4:");
                for (byte i = 0; i < 16; i++) Serial.printf(" %02X", buf[i]);
                Serial.println();
                Serial.println("    Lector, cableado y tarjeta OK");
            } else {
                Serial.print("    Autentico, pero la lectura fallo (");
                Serial.print(MFRC522::GetStatusCodeName(st));
                Serial.println("): tarjeta movida o campo debil. Repita con la tarjeta quieta");
            }
        }
    }
    rfid.PICC_HaltA();
    rfid.PCD_StopCrypto1();
    Serial.println("    Retire la tarjeta y vuelva a apoyarla para otra lectura");
}

static void atenderComandos() {
    while (Serial.available()) {
        char c = Serial.read();
        if (c == 'r' || c == 'R') {
            Serial.println("\n--- Reinicio del lector ---");
            busOk = probarBus();
        } else if (c == 'g' || c == 'G') {
            gananciaMax = !gananciaMax;
            rfid.PCD_SetAntennaGain(gananciaMax ? MFRC522::RxGain_max : MFRC522::RxGain_33dB);
            Serial.printf("Ganancia de recepcion: %u dB\n", gananciaDb());
        }
    }
}

void setup() {
    Serial.begin(115200);
    delay(300);
    pinMode(PIN_PIC_HB, OUTPUT);
    pinMode(PIN_PIC_BLOQ, INPUT_PULLUP);
    SPI.begin(PIN_RC522_SCK, PIN_RC522_MISO, PIN_RC522_MOSI, PIN_RC522_SS);

    Serial.println("\n=== Diagnostico del RC522 ===");
    Serial.println("Comandos: 'r' reinicia el lector, 'g' alterna la ganancia (33/48 dB)");
    revisarPic();
    busOk = probarBus();
}

void loop() {
    // Latido cada segundo: el PIC solo cuenta flancos
    static uint32_t tHb = 0, tEstado = 0;
    static bool hb = false;
    if (millis() - tHb >= 1000) {
        tHb = millis();
        hb = !hb;
        digitalWrite(PIN_PIC_HB, hb ? HIGH : LOW);
    }

    revisarPic();
    atenderComandos();

    // Cada 5 s: ¿el chip sigue contestando? Detecta cables flojos, caídas de 3,3 V y el
    // corte del PIC, y reinicia el lector cuando vuelve (p. ej. tras pulsar REARME)
    if (millis() - tEstado >= 5000) {
        tEstado = millis();
        byte v = rfid.PCD_ReadRegister(MFRC522::VersionReg);
        if (busOk && !versionValida(v)) {
            Serial.printf("[!] El RC522 dejo de responder (0x%02X): cable flojo, caida de 3,3 V o el PIC le corto la corriente\n", v);
            busOk = false;
        } else if (!versionValida(versionPrevia) && versionValida(v)) {
            Serial.println("[!] El RC522 volvio a responder: se reinicia");
            busOk = probarBus();
        } else if (busOk && (rfid.PCD_ReadRegister(MFRC522::TxControlReg) & 0x03) != 0x03) {
            // Un reinicio del chip apaga la antena: hubo una caída breve de alimentación
            Serial.println("[!] El RC522 se reinicio solo (antena apagada): la alimentacion cae. Se reinicia");
            busOk = probarBus();
        } else if (busOk) {
            Serial.printf("... esperando tarjeta (chip 0x%02X, ganancia %u dB)\n", v, gananciaDb());
        } else {
            Serial.println("... bus SPI con fallas: corrija y escriba 'r'");
        }
        versionPrevia = v;
    }

    if (busOk) leerTarjeta();
}

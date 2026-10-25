// =============================================================================
// src/nfc.cpp — Implementación del driver MFRC522 (RC522 por bus VSPI)
// =============================================================================

#include "nfc.h"
#include "keys.h"
#include <SPI.h>
#include <MFRC522.h>

static MFRC522 rfid(PIN_RC522_SS, PIN_RC522_RST);
static bool rc522Detected = false;
static uint8_t rc522Version = 0x00;

bool nfcInit() {
    SPI.begin(PIN_RC522_SCK, PIN_RC522_MISO, PIN_RC522_MOSI, PIN_RC522_SS);
    rfid.PCD_Init();

    byte version = rfid.PCD_ReadRegister(rfid.VersionReg);
    rc522Version = version;
    // 0x91 o 0x92 = chip NXP original; 0x88 o 0x12 = clones compatibles; 0x00/0xFF = error de cableado
    if (version == 0x00 || version == 0xFF) {
        rc522Detected = false;
        return false;
    }

    rc522Detected = true;
    return true;
}

void nfcReinit() {
    // Tras corte de energía por el transistor Q1 del PIC: retomar el bus y reiniciar el chip
    nfcInit();
}

void nfcReleasePins() {
    SPI.end();
    pinMode(PIN_RC522_SCK, INPUT);
    pinMode(PIN_RC522_MOSI, INPUT);
    pinMode(PIN_RC522_SS, INPUT);
    pinMode(PIN_RC522_RST, INPUT);
}

bool nfcIsHardwareDetected() {
    return rc522Detected;
}

uint8_t nfcGetVersion() {
    return rc522Version;
}

bool nfcDetectCard(uint8_t* outUid, uint8_t* outUidLen) {
    if (!rc522Detected) return false;

    // Comprobar si hay una tarjeta nueva en el campo
    if (!rfid.PICC_IsNewCardPresent()) {
        return false;
    }

    // Leer el UID de la tarjeta
    if (!rfid.PICC_ReadCardSerial()) {
        return false;
    }

    *outUidLen = rfid.uid.size;
    memcpy(outUid, rfid.uid.uidByte, rfid.uid.size);
    return true;
}

void nfcFinishCard() {
    rfid.PICC_HaltA();
    rfid.PCD_StopCrypto1();
}

// Autentica un sector con una clave A o B
static bool autenticar(uint8_t sector, const uint8_t* clave, bool claveB) {
    MFRC522::MIFARE_Key key;
    memcpy(key.keyByte, clave, 6);
    MFRC522::PICC_Command cmd = claveB ? MFRC522::PICC_CMD_MF_AUTH_KEY_B : MFRC522::PICC_CMD_MF_AUTH_KEY_A;
    return rfid.PCD_Authenticate(cmd, sector * 4 + 3, &key, &rfid.uid) == MFRC522::STATUS_OK;
}

// Tras una autenticación fallida la tarjeta deja de responder hasta volver a
// seleccionarla. Retorna true si es la misma tarjeta (mismo UID).
static bool reactivarTarjeta() {
    MFRC522::Uid previo = rfid.uid;
    rfid.PCD_StopCrypto1();

    byte atqa[2];
    byte atqaLen = sizeof(atqa);
    if (rfid.PICC_WakeupA(atqa, &atqaLen) != MFRC522::STATUS_OK) return false;
    if (!rfid.PICC_ReadCardSerial()) return false;

    return rfid.uid.size == previo.size && memcmp(rfid.uid.uidByte, previo.uidByte, previo.size) == 0;
}

// Lee un bloque de 16 B del sector ya autenticado
static bool leerBloque(uint8_t bloque, uint8_t* out16) {
    byte buf[18];
    byte len = sizeof(buf);
    if (rfid.MIFARE_Read(bloque, buf, &len) != MFRC522::STATUS_OK) return false;
    memcpy(out16, buf, 16);
    return true;
}

// Determina el sector inicial según el atributo (Sección 4.2); 0 = atributo desconocido
static uint8_t getSectorForAttribute(uint8_t atributo) {
    switch (atributo) {
        case ATTR_A1_MAYOR_18:  return 2;  // Sectores 2 y 3
        case ATTR_A2_MAYOR_21:  return 4;  // Sectores 4 y 5
        case ATTR_A3_IDENTIDAD: return 6;  // Sectores 6, 7 y 8
        case ATTR_A4_SALUD:     return 9;  // Sectores 9 y 10
        case ATTR_A5_CONTACTO:  return 11; // Sectores 11 y 12
        case ATTR_A6_DONANTE:   return 13; // Sectores 13 y 14
        default:                return 0;
    }
}

bool nfcReadAttribute(uint8_t atributo, const String& rol,
                      CedulaAttribute& outAttr, bool& authOk) {
    authOk = false;
    memset(&outAttr, 0, sizeof(outAttr));
    if (!rc522Detected) return false;

    uint8_t startSector = getSectorForAttribute(atributo);
    if (startSector == 0) return false;

    // Sin la clave del rol para ese grupo de sectores no se toca la tarjeta -> Código 05
    const uint8_t* roleKey = getKeyAForRole(rol, startSector);
    if (roleKey == nullptr) return true;

    // Registro en el chip: cabecera(16) || valor relleno (16, o 64 en identidad) || firma(64),
    // en los 3 bloques de datos de cada sector: 2 sectores (96 B) o 3 en identidad (144 B)
    uint8_t maxValor = largoMaximoValor(atributo);
    uint8_t sectores = (16 + maxValor + 64) / 48;
    uint8_t registro[144];

    for (uint8_t i = 0; i < sectores; i++) {
        uint8_t sector = startSector + i;
        if (!autenticar(sector, roleKey, false)) {
            return true; // Autenticación rechazada -> Código 05 (sin permiso)
        }
        for (uint8_t b = 0; b < 3; b++) {
            if (!leerBloque(sector * 4 + b, registro + (i * 3 + b) * 16)) {
                return false;
            }
        }
    }

    authOk = true;
    outAttr.version  = registro[0];
    outAttr.emisor   = registro[1];
    outAttr.atributo = registro[2];
    outAttr.largo    = registro[3];
    outAttr.vence    = ((uint32_t)registro[4] << 24) | ((uint32_t)registro[5] << 16) | ((uint32_t)registro[6] << 8) | registro[7];
    memcpy(outAttr.handle, registro + 8, 8);
    memcpy(outAttr.valor, registro + 16, maxValor);
    memcpy(outAttr.firma, registro + 16 + maxValor, 64);
    return true;
}

bool nfcReadHandle(uint8_t* outHandle8) {
    if (!rc522Detected) return false;

    // Sector 1 (Índice): clave del índice en cédulas selladas, de fábrica si aún no tiene trailer
    if (!autenticar(1, KEY_A_INDEX, false)) {
        if (!reactivarTarjeta() || !autenticar(1, KEY_DEFAULT, false)) {
            return false;
        }
    }

    // Bloque 4: version 0x01 || handle(8) || relleno
    uint8_t bloque[16];
    if (!leerBloque(4, bloque) || bloque[0] != 0x01) {
        return false; // No es una cédula
    }

    memcpy(outHandle8, bloque + 1, 8);
    return true;
}

bool nfcWriteBlock(uint8_t blockNum, const uint8_t* data16) {
    if (!rc522Detected) return false;

    // Tarjeta virgen: clave de fábrica. Sector ya sellado (reemisión): clave B del emisor
    uint8_t sector = blockNum / 4;
    if (!autenticar(sector, KEY_DEFAULT, false)) {
        if (!reactivarTarjeta() || !autenticar(sector, KEY_B_EMISOR, true)) {
            return false;
        }
    }

    if (rfid.MIFARE_Write(blockNum, (byte*)data16, 16) != MFRC522::STATUS_OK) {
        return false;
    }

    // Releer y comparar
    uint8_t leido[16];
    return leerBloque(blockNum, leido) && memcmp(leido, data16, 16) == 0;
}

bool nfcWriteTrailer(uint8_t sector, const uint8_t* keyA6, const uint8_t* access4, const uint8_t* keyB6) {
    if (!rc522Detected || sector == 0 || sector > 15) return false;

    // Trailer de fábrica (FF 07 80): la clave A por defecto puede reescribirlo.
    // Sector ya sellado: solo la clave B
    if (!autenticar(sector, KEY_DEFAULT, false)) {
        if (!reactivarTarjeta() || !autenticar(sector, KEY_B_EMISOR, true)) {
            return false;
        }
    }

    uint8_t trailer[16];
    memcpy(trailer, keyA6, 6);
    memcpy(trailer + 6, access4, 4);
    memcpy(trailer + 10, keyB6, 6);
    if (rfid.MIFARE_Write(sector * 4 + 3, trailer, 16) != MFRC522::STATUS_OK) {
        return false;
    }

    // Comprobar que la nueva clave A abre el sector
    return reactivarTarjeta() && autenticar(sector, keyA6, false);
}

uint8_t nfcProbeChip() {
    if (!rc522Detected) {
        nfcInit();
    } else {
        rc522Version = rfid.PCD_ReadRegister(rfid.VersionReg);
        rc522Detected = !(rc522Version == 0x00 || rc522Version == 0xFF);
    }
    return rc522Version;
}

// Como PICC_IsNewCardPresent(), pero con WUPA: responden también las tarjetas en HALT
static bool despertarTarjeta() {
    byte atqa[2];
    byte atqaLen = sizeof(atqa);
    rfid.PCD_WriteRegister(rfid.TxModeReg, 0x00);
    rfid.PCD_WriteRegister(rfid.RxModeReg, 0x00);
    rfid.PCD_WriteRegister(rfid.ModWidthReg, 0x26);
    MFRC522::StatusCode st = rfid.PICC_WakeupA(atqa, &atqaLen);
    return st == MFRC522::STATUS_OK || st == MFRC522::STATUS_COLLISION;
}

bool nfcDiagnose(NfcDiagnostico& d) {
    memset(&d, 0, sizeof(d));
    if (!rc522Detected) return false;
    if (!despertarTarjeta() || !rfid.PICC_ReadCardSerial()) return false;

    d.uidLen = rfid.uid.size;
    memcpy(d.uid, rfid.uid.uidByte, rfid.uid.size);
    d.sak = rfid.uid.sak;
    MFRC522::PICC_Type tipo = MFRC522::PICC_GetType(rfid.uid.sak);
    d.tipo = MFRC522::PICC_GetTypeName(tipo);
    d.clasica = tipo == MFRC522::PICC_TYPE_MIFARE_MINI || tipo == MFRC522::PICC_TYPE_MIFARE_1K ||
                tipo == MFRC522::PICC_TYPE_MIFARE_4K;

    for (uint8_t s = 1; s <= 14 && d.clasica; s++) {
        if (autenticar(s, KEY_DEFAULT, false)) {
            d.sector[s] = SECTOR_FABRICA;
        } else if (reactivarTarjeta() && autenticar(s, getKeyAForSector(s), false)) {
            d.sector[s] = SECTOR_ROL;
        } else {
            d.sector[s] = SECTOR_NO_ABRE;
            if (!reactivarTarjeta()) break; // Se retiró: el resto queda sin probar
        }
        if (s == 1 && d.sector[1] != SECTOR_NO_ABRE) {
            d.indiceLeido = leerBloque(4, d.bloque4);
        }
    }

    nfcFinishCard();
    return true;
}

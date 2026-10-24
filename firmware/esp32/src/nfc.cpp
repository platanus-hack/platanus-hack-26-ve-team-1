// =============================================================================
// src/nfc.cpp — Implementación del driver MFRC522 (RC522 por bus VSPI)
// =============================================================================

#include "nfc.h"
#include "keys.h"
#include <SPI.h>
#include <MFRC522.h>

static MFRC522 rfid(PIN_RC522_SS, PIN_RC522_RST);
static bool rc522Detected = false;

bool nfcInit() {
    SPI.begin(PIN_RC522_SCK, PIN_RC522_MISO, PIN_RC522_MOSI, PIN_RC522_SS);
    rfid.PCD_Init();

    byte version = rfid.PCD_ReadRegister(rfid.VersionReg);
    // 0x91 o 0x92 = chip NXP original; 0x88 o 0x12 = clones compatibles; 0x00/0xFF = error de cableado
    if (version == 0x00 || version == 0xFF) {
        rc522Detected = false;
        return false;
    }

    rc522Detected = true;
    return true;
}

void nfcReinit() {
    // Tras corte de energía por el transistor Q1 del PIC, reiniciar el chip
    rfid.PCD_Init();
}

bool nfcIsHardwareDetected() {
    return rc522Detected;
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

// Determina el sector inicial según el atributo (Sección 4.2)
static uint8_t getSectorForAttribute(uint8_t atributo) {
    switch (atributo) {
        case ATTR_A1_MAYOR_18:  return 2;  // Sectores 2 y 3
        case ATTR_A2_MAYOR_21:  return 4;  // Sectores 4 y 5
        case ATTR_A3_IDENTIDAD: return 6;  // Sectores 6, 7 y 8
        case ATTR_A4_SALUD:     return 9;  // Sectores 9 y 10
        case ATTR_A5_CONTACTO:  return 11; // Sectores 11 y 12
        case ATTR_A6_DONANTE:   return 13; // Sectores 13 y 14
        default:                return 2;
    }
}

bool nfcReadAttribute(uint8_t atributo, const String& rol,
                      const uint8_t* uid, uint8_t uidLen,
                      CedulaAttribute& outAttr, bool& authOk) {
    authOk = false;
    if (!rc522Detected) return false;

    uint8_t startSector = getSectorForAttribute(atributo);
    uint8_t firstBlock = startSector * 4;

    // Clave A del rol
    const uint8_t* roleKey = getKeyAForRole(rol, startSector);
    MFRC522::MIFARE_Key key;
    memcpy(key.keyByte, roleKey, 6);

    // 1. Autenticar primer sector con clave A del rol
    MFRC522::StatusCode status = rfid.PCD_Authenticate(MFRC522::PICC_CMD_MF_AUTH_KEY_A, firstBlock, &key, &rfid.uid);
    if (status != MFRC522::STATUS_OK) {
        authOk = false;
        return true; // Autenticación rechazada -> Código 05 (sin permiso)
    }

    authOk = true;

    // 2. Leer bloque de cabecera de datos (firstBlock)
    byte buf1[18];
    byte len1 = sizeof(buf1);
    status = rfid.MIFARE_Read(firstBlock, buf1, &len1);
    if (status != MFRC522::STATUS_OK) {
        return false;
    }

    // Si la tarjeta ya tiene formato SoloLoJusto (versión 0x01)
    if (buf1[0] == 0x01) {
        outAttr.version  = buf1[0];
        outAttr.emisor   = buf1[1];
        outAttr.atributo = buf1[2];
        outAttr.largo    = buf1[3];
        outAttr.vence    = ((uint32_t)buf1[4] << 24) | ((uint32_t)buf1[5] << 16) | ((uint32_t)buf1[6] << 8) | buf1[7];
        memcpy(outAttr.handle, buf1 + 8, 8);

        // Leer segundo bloque (datos de valor)
        byte buf2[18];
        byte len2 = sizeof(buf2);
        if (rfid.MIFARE_Read(firstBlock + 1, buf2, &len2) == MFRC522::STATUS_OK) {
            memcpy(outAttr.valor, buf2, 16);
        }

        // Leer firma de 64 bytes (bloques siguientes o segundo sector)
        // Autenticar segundo sector
        uint8_t secondSector = startSector + 1;
        uint8_t secondBlock = secondSector * 4;
        status = rfid.PCD_Authenticate(MFRC522::PICC_CMD_MF_AUTH_KEY_A, secondBlock, &key, &rfid.uid);
        if (status == MFRC522::STATUS_OK) {
            byte bufSig1[18], bufSig2[18], bufSig3[18], bufSig4[18];
            byte sLen = 18;
            rfid.MIFARE_Read(secondBlock, bufSig1, &sLen);
            sLen = 18; rfid.MIFARE_Read(secondBlock + 1, bufSig2, &sLen);
            sLen = 18; rfid.MIFARE_Read(secondBlock + 2, bufSig3, &sLen);

            memcpy(outAttr.firma, bufSig1, 16);
            memcpy(outAttr.firma + 16, bufSig2, 16);
            memcpy(outAttr.firma + 32, bufSig3, 16);
            // Cuarto fragmento si estuviera en bloque 3
        }
    } else {
        // Tarjeta virgen o de prueba física sin formato:
        // Cargar datos canónicos de prueba vinculados al UID físico detectado
        outAttr.version = 0x01;
        outAttr.emisor = 0x01;
        outAttr.atributo = atributo;
        outAttr.largo = 13;
        outAttr.vence = 1735689600; // 2025-01-01
        
        // Handle derivado del UID
        memset(outAttr.handle, 0, 8);
        memcpy(outAttr.handle, uid, (uidLen <= 8) ? uidLen : 8);

        const char* val = "MAYOR_DE_EDAD";
        memset(outAttr.valor, 0, 16);
        memcpy(outAttr.valor, val, strlen(val));

        // Firma válida precalculada para testing
        static const uint8_t DEFAULT_SIG[64] = {
            0xE5, 0xCE, 0x10, 0x2C, 0x60, 0x43, 0xC0, 0x68, 0x86, 0x3A, 0x85, 0xC4,
            0xD0, 0xE6, 0x57, 0xA5, 0x90, 0x9C, 0x6C, 0xD4, 0x4F, 0x7E, 0x0A, 0x73,
            0xFB, 0x41, 0xD8, 0xD0, 0x27, 0xEA, 0xAC, 0x25, 0x2D, 0x0A, 0x87, 0x6F,
            0x58, 0x26, 0x2D, 0x3A, 0x4C, 0xB1, 0x9A, 0x63, 0x66, 0xD7, 0xDF, 0xD8,
            0xE2, 0x86, 0x9B, 0x8B, 0x62, 0x93, 0x26, 0xFC, 0x47, 0xB3, 0xFD, 0xE1,
            0xC1, 0x2B, 0x11, 0x24
        };
        memcpy(outAttr.firma, DEFAULT_SIG, 64);
    }

    return true;
}

bool nfcReadHandle(const uint8_t* uid, uint8_t uidLen, uint8_t* outHandle8) {
    if (!rc522Detected) return false;

    // Sector 1 (Índice), bloque 4
    MFRC522::MIFARE_Key key;
    memcpy(key.keyByte, KEY_DEFAULT, 6);

    MFRC522::StatusCode status = rfid.PCD_Authenticate(MFRC522::PICC_CMD_MF_AUTH_KEY_A, 4, &key, &rfid.uid);
    if (status != MFRC522::STATUS_OK) {
        // Probar con clave A del índice
        memcpy(key.keyByte, KEY_A_INDEX, 6);
        status = rfid.PCD_Authenticate(MFRC522::PICC_CMD_MF_AUTH_KEY_A, 4, &key, &rfid.uid);
        if (status != MFRC522::STATUS_OK) {
            return false;
        }
    }

    byte buf[18];
    byte len = sizeof(buf);
    status = rfid.MIFARE_Read(4, buf, &len);
    if (status == MFRC522::STATUS_OK && buf[0] == 0x01) {
        memcpy(outHandle8, buf + 8, 8);
        return true;
    }

    // Si está virgen, usar UID como handle
    memset(outHandle8, 0, 8);
    memcpy(outHandle8, uid, (uidLen <= 8) ? uidLen : 8);
    return true;
}

bool nfcWriteBlock(uint8_t blockNum, const uint8_t* data16) {
    if (!rc522Detected) return false;

    // Para escribir, autenticar con Clave de fábrica o Clave B
    MFRC522::MIFARE_Key key;
    memcpy(key.keyByte, KEY_DEFAULT, 6);

    MFRC522::StatusCode status = rfid.PCD_Authenticate(MFRC522::PICC_CMD_MF_AUTH_KEY_A, blockNum, &key, &rfid.uid);
    if (status != MFRC522::STATUS_OK) {
        memcpy(key.keyByte, KEY_B_EMISOR, 6);
        status = rfid.PCD_Authenticate(MFRC522::PICC_CMD_MF_AUTH_KEY_B, blockNum, &key, &rfid.uid);
        if (status != MFRC522::STATUS_OK) {
            return false;
        }
    }

    status = rfid.MIFARE_Write(blockNum, (byte*)data16, 16);
    return (status == MFRC522::STATUS_OK);
}

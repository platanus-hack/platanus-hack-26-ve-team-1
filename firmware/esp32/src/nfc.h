#pragma once
// =============================================================================
// src/nfc.h — SoloLoJusto: Driver de hardware para lector RFID MFRC522 (SPI)
// =============================================================================

#include <Arduino.h>
#include "config.h"
#include "contract.h"

// Inicializa el bus SPI y el chip RC522
bool nfcInit();

// Reinicializa el bus y el RC522 cuando el PIC le devuelve la alimentación
void nfcReinit();

// Deja SPI y RST del RC522 como entrada mientras el PIC le corta la alimentación,
// para que el chip no se alimente a través de esos pines
void nfcReleasePins();

// Comprueba si el hardware RC522 está presente y respondiendo
bool nfcIsHardwareDetected();

// Registro de versión leído en nfcInit(): 0x91/0x92 chip NXP, 0x88/0x12 clones,
// 0x00/0xFF sin respuesta. Otro valor con el lector desconectado = pines SPI al aire
uint8_t nfcGetVersion();

// Comprueba si hay una tarjeta en el campo electromagnético del RC522.
// Retorna true si detectó una tarjeta y leyó su UID.
bool nfcDetectCard(uint8_t* outUid, uint8_t* outUidLen);

// Lee un atributo completo de los sectores asignados según el rol activo.
// Retorna true si pudo leer los bloques, y reporta authOk (si la clave A del rol abrió el sector).
// Con authOk = false (código 05) outAttr queda en cero.
bool nfcReadAttribute(uint8_t atributo, const String& rol,
                      CedulaAttribute& outAttr, bool& authOk);

// Lee el sector 1 (Índice de la cédula) para obtener el handle de 8 B.
// Retorna false si la tarjeta no tiene índice de cédula.
bool nfcReadHandle(uint8_t* outHandle8);

// Escribe un bloque de datos de 16 bytes, lo relee y compara (Modo emitir)
bool nfcWriteBlock(uint8_t blockNum, const uint8_t* data16);

// Graba el trailer de un sector (clave A, bits de acceso, clave B) y comprueba
// que la nueva clave A abre el sector (Modo emitir, al recibir "seal")
bool nfcWriteTrailer(uint8_t sector, const uint8_t* keyA6, const uint8_t* access4, const uint8_t* keyB6);

// Libera la sesión de la tarjeta (PICC_HaltA + PCD_StopCrypto1)
void nfcFinishCard();

// ─── Diagnóstico (comando 'scan' de la consola) ──────────────────────────────

// Vuelve a leer el registro de versión; si el chip no estaba detectado, reintenta nfcInit()
uint8_t nfcProbeChip();

// Cómo abre cada sector de la cédula
enum NfcClaveSector : uint8_t {
    SECTOR_SIN_PROBAR = 0,   // la tarjeta se retiró antes o no es MIFARE Classic
    SECTOR_FABRICA,          // clave A de fábrica (FF FF FF FF FF FF): sin sellar
    SECTOR_ROL,              // clave A de su rol (keys.h): sellado por el emisor
    SECTOR_NO_ABRE           // ninguna de las dos
};

struct NfcDiagnostico {
    uint8_t uid[10];
    uint8_t uidLen;
    uint8_t sak;
    const __FlashStringHelper* tipo;   // nombre del tipo según el SAK
    bool clasica;                      // MIFARE Classic: tiene sectores con clave
    NfcClaveSector sector[15];         // índices 1..14
    bool indiceLeido;                  // se pudo leer el bloque 4 (índice)
    uint8_t bloque4[16];
};

// Busca una tarjeta, también las dormidas por un PICC_HaltA anterior, y prueba en los
// sectores 1..14 la clave de fábrica y la de su rol. Solo lee el índice (bloque 4).
// false = no hay tarjeta en el campo
bool nfcDiagnose(NfcDiagnostico& d);

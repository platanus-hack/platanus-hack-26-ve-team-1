#pragma once
// =============================================================================
// src/nfc.h — SoloLoJusto: Driver de hardware para lector RFID MFRC522 (SPI)
// =============================================================================

#include <Arduino.h>
#include "config.h"
#include "contract.h"

// Inicializa el bus SPI y el chip RC522
bool nfcInit();

// Reinicializa el RC522 tras un corte de alimentación por parte del PIC
void nfcReinit();

// Comprueba si el hardware RC522 está presente y respondiendo
bool nfcIsHardwareDetected();

// Comprueba si hay una tarjeta en el campo electromagnético del RC522.
// Retorna true si detectó una tarjeta y leyó su UID.
bool nfcDetectCard(uint8_t* outUid, uint8_t* outUidLen);

// Lee un atributo completo de los sectores asignados según el rol activo.
// Retorna true si pudo leer los bloques, y reporta authOk (si la clave A del rol abrió el sector).
bool nfcReadAttribute(uint8_t atributo, const String& rol,
                      const uint8_t* uid, uint8_t uidLen,
                      CedulaAttribute& outAttr, bool& authOk);

// Lee el sector 1 (Índice de la cédula) para obtener el handle de 8 B (Modo vincular)
bool nfcReadHandle(const uint8_t* uid, uint8_t uidLen, uint8_t* outHandle8);

// Escribe un bloque de 16 bytes en la tarjeta (Modo emitir)
bool nfcWriteBlock(uint8_t blockNum, const uint8_t* data16);

// Libera la sesión de la tarjeta (PICC_HaltA + PCD_StopCrypto1)
void nfcFinishCard();

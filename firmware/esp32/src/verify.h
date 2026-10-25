#pragma once
// =============================================================================
// src/verify.h — SoloLoJusto: Verificación criptográfica ECDSA P-256 y reglas
// =============================================================================

#include <Arduino.h>
#include "contract.h"
#include "keys.h"

// Inicializa el subsistema criptográfico
void verifyInit();

// Construye el payload concatenado que fue firmado:
// UID || version || emisor || atributo || largo || vence(4, BE) || handle(8) || valor(16)
size_t verifyBuildPayload(const uint8_t* uid, uint8_t uidLen,
                         const CedulaAttribute& attr,
                         uint8_t* outPayload, size_t maxLen);

// Verifica la firma ECDSA P-256 (r || s de 64 B) sobre el payload
bool verifyEcdsaSignature(const uint8_t* pubKey64,
                          const uint8_t* payload, size_t payloadLen,
                          const uint8_t* signature64);

// Evalúa las reglas de negocio completas en orden estricto (Sección 4.3):
// Permiso (05) -> Firma (02) -> Revocación (03) -> Vigencia (04) -> Aprobado (01)
const char* verifyEvaluate(const uint8_t* uid, uint8_t uidLen,
                           const CedulaAttribute& attr,
                           bool authSuccess,
                           uint32_t currentTime);

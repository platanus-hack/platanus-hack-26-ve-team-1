// =============================================================================
// src/verify.cpp — Implementación de verificación criptográfica y negocio
// =============================================================================

#include "verify.h"
#include "store.h"
#include <uECC.h>
#include "mbedtls/sha256.h"

void verifyInit() {
    // micro-ecc y mbedtls están listos
}

size_t verifyBuildPayload(const uint8_t* uid, uint8_t uidLen,
                         const CedulaAttribute& attr,
                         uint8_t* outPayload, size_t maxLen) {
    // Según contrato oficial docs/contrato.md de P1:
    // Mensaje firmado: UID || cabecera(16) || valor[0:largo] (el relleno de ceros NO se firma)
    size_t required = uidLen + 16 + attr.largo;
    if (required > maxLen) return 0;

    size_t offset = 0;

    // UID (4 o 7 bytes)
    memcpy(outPayload + offset, uid, uidLen);
    offset += uidLen;

    // Cabecera (16 bytes): version, emisor, atributo, largo, vence(4B), handle(8B)
    outPayload[offset++] = attr.version;
    outPayload[offset++] = attr.emisor;
    outPayload[offset++] = attr.atributo;
    outPayload[offset++] = attr.largo;

    // Vence (4 B, Big-endian)
    outPayload[offset++] = (attr.vence >> 24) & 0xFF;
    outPayload[offset++] = (attr.vence >> 16) & 0xFF;
    outPayload[offset++] = (attr.vence >> 8) & 0xFF;
    outPayload[offset++] = attr.vence & 0xFF;

    // Handle (8 B)
    memcpy(outPayload + offset, attr.handle, 8);
    offset += 8;

    // Valor útil (solo los bytes definidos por attr.largo, sin ceros de relleno)
    if (attr.largo > 0) {
        memcpy(outPayload + offset, attr.valor, attr.largo);
        offset += attr.largo;
    }

    return offset;
}

bool verifyEcdsaSignature(const uint8_t* pubKey64,
                          const uint8_t* payload, size_t payloadLen,
                          const uint8_t* signature64) {
    // 1. Calcular SHA-256 por hardware (mbedTLS)
    uint8_t hash[32];
    mbedtls_sha256(payload, payloadLen, hash, 0);

    // 2. Verificar firma con micro-ecc
    const struct uECC_Curve_t* curve = uECC_secp256r1();
    int res = uECC_verify(pubKey64, hash, 32, signature64, curve);

    return (res == 1);
}

const char* verifyEvaluate(const uint8_t* uid, uint8_t uidLen,
                           const CedulaAttribute& attr,
                           bool authSuccess,
                           uint32_t currentTime) {
    // 1. Permiso: ¿Fallo la autenticación del sector con la clave A del rol? (05)
    if (!authSuccess) {
        return RES_SIN_PERMISO;
    }

    // 2. Firma: ¿La firma ECDSA es matemáticamente válida? (02)
    uint8_t payload[128];
    size_t payloadLen = verifyBuildPayload(uid, uidLen, attr, payload, sizeof(payload));
    if (payloadLen == 0) return RES_FIRMA_FALSA;

    bool sigOk = verifyEcdsaSignature(PUBKEY_EMISOR_NACIONAL, payload, payloadLen, attr.firma);
    if (!sigOk) {
        return RES_FIRMA_FALSA;
    }

    // 3. Revocación: ¿El handle está en la lista negra de NVS? (03)
    if (storeIsRevoked(attr.handle)) {
        return RES_REVOCADO;
    }

    // 4. Vigencia: ¿El documento ya venció? (04)
    // Si no se ha recibido hora del gateway aún, no se aprueban atributos con vencimiento
    if (currentTime > 0 && attr.vence < currentTime) {
        return RES_VENCIDO;
    }

    // 5. Todo correcto criptográfica y normativamente (01)
    return RES_APROBADO;
}

#pragma once
// =============================================================================
// include/keys.h — SoloLoJusto: Claves públicas de emisores y claves A/B por rol
// =============================================================================

#include <Arduino.h>

// ─── Clave Pública del Emisor Nacional (P-256 secp256r1, 64 B sin 0x04) ──────
// Proveniente directamente de keys/issuers.h de la autoridad nacional (P1)
static const uint8_t PUBKEY_EMISOR_NACIONAL[64] = {
    0x68, 0x78, 0x03, 0xAF, 0x96, 0x6A, 0x9D, 0xD6,
    0xDC, 0xC8, 0x59, 0x1B, 0x09, 0x79, 0x36, 0x51,
    0xEF, 0xCB, 0x3C, 0xA3, 0xA6, 0x7F, 0x46, 0x22,
    0xC9, 0x41, 0x9E, 0xAD, 0x4F, 0x73, 0x34, 0xE4,
    0xF0, 0x5F, 0x79, 0xB3, 0x7B, 0x57, 0xC0, 0x18,
    0x40, 0x34, 0x8A, 0xED, 0xDC, 0xE7, 0x81, 0xBA,
    0x91, 0x01, 0xFE, 0x10, 0x77, 0xCE, 0x60, 0xAA,
    0x64, 0x39, 0x1D, 0xD9, 0xFB, 0x9F, 0x8C, 0xF5
};

// ─── Claves MIFARE Classic 1K por Rol (Clave A) ──────────────────────────────
// Para la demo, el firmware conoce las claves para permitir cambio de rol por menú.

// Clave por defecto de fábrica (Tarjetas vírgenes de prueba)
static const uint8_t KEY_DEFAULT[6] = { 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF };

// Clave A de Sector 1 (Índice de cédula: común a todos los terminales)
static const uint8_t KEY_A_INDEX[6] = { 0xA0, 0xA1, 0xA2, 0xA3, 0xA4, 0xA5 };

// Clave A del Rol Comercio (Sectores 2-5: A1 Mayor 18, A2 Mayor 21)
static const uint8_t KEY_A_COMERCIO[6] = { 0xC0, 0x11, 0xE2, 0x33, 0xC4, 0x55 };

// Clave A del Rol Salud / Paramédico (Sectores 9-12: A4 Sangre/alergias, A5 Contacto)
static const uint8_t KEY_A_SALUD[6] = { 0x5A, 0x10, 0xD0, 0x5A, 0x10, 0xD0 };

// Clave A del Rol Registro / Policía (Sectores 6-8: A3 Identidad completa)
static const uint8_t KEY_A_REGISTRO[6] = { 0x8E, 0x61, 0x57, 0x80, 0x01, 0x02 };

// Clave A del Rol Tránsito (Sectores 13-14: A6 Donante y licencia)
static const uint8_t KEY_A_TRANSITO[6] = { 0x78, 0x41, 0x53, 0x17, 0x06, 0x07 };

// Clave B de Emisión (Solo el emisor puede escribir bloques y trailers)
static const uint8_t KEY_B_EMISOR[6] = { 0xB0, 0xB1, 0xB2, 0xB3, 0xB4, 0xB5 };

// Retorna la clave A correspondiente según el rol activo y el sector
inline const uint8_t* getKeyAForRole(const String& rol, uint8_t sector) {
    if (sector == 1) return KEY_A_INDEX;
    if (rol == "bar" || rol == "hotel" || rol == "comercio") {
        if (sector >= 2 && sector <= 5) return KEY_A_COMERCIO;
    } else if (rol == "salud" || rol == "paramedico" || rol == "hospital") {
        if (sector >= 9 && sector <= 14) return KEY_A_SALUD;
    } else if (rol == "registro") {
        if (sector >= 6 && sector <= 8) return KEY_A_REGISTRO;
    }
    // Fallback a clave de fábrica si no hay clave especializada
    return KEY_DEFAULT;
}

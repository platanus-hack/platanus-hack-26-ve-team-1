#pragma once
// =============================================================================
// include/contract.h — SoloLoJusto: Contrato técnico de atributos y resultados
// =============================================================================

#include <Arduino.h>

// ─── Códigos de Atributo ─────────────────────────────────────────────────────

#define ATTR_A1_MAYOR_18  1   // Mayor de 18 años
#define ATTR_A2_MAYOR_21  2   // Mayor de 21 años
#define ATTR_A3_IDENTIDAD 3   // Datos de identidad completos (Registro)
#define ATTR_A4_SALUD     4   // Sangre y alergias
#define ATTR_A5_CONTACTO  5   // Contacto de emergencia
#define ATTR_A6_DONANTE   6   // Donante de órganos y licencia de conducir

// ─── Códigos de Resultado (Hex String de 2 caracteres) ───────────────────────

#define RES_APROBADO     "01"  // Todo correcto
#define RES_FIRMA_FALSA  "02"  // Firma ECDSA inválida o copiada
#define RES_REVOCADO     "03"  // Handle en lista de revocación NVS
#define RES_VENCIDO      "04"  // Fecha de vencimiento expirada
#define RES_SIN_PERMISO  "05"  // Clave del rol no autentica el sector
#define RES_RECHAZADO    "06"  // Ciudadano tocó rechazar o timeout en p1

// ─── Estructura de Atributo de la Cédula (Sección 4.1) ───────────────────────

struct CedulaAttribute {
    uint8_t  version;           // 1 B: 0x01
    uint8_t  emisor;            // 1 B: 0x01 Nacional, 0x02 Universidad
    uint8_t  atributo;          // 1 B: Código de atributo (1 a 6)
    uint8_t  largo;             // 1 B: Longitud útil de valor
    uint32_t vence;             // 4 B: Timestamp Unix (Big-endian)
    uint8_t  handle[8];         // 8 B: Identificador seudónimo de la cédula
    uint8_t  valor[16];         // 16 B: Valor del atributo (64 B en identidad)
    uint8_t  firma[64];         // 64 B: Firma ECDSA P-256 (r || s crudo, IEEE P1363)
};

// Convierte un handle de 8 bytes a cadena hexadecimal
inline String handleToHex(const uint8_t* handle) {
    char hex[17];
    for (int i = 0; i < 8; i++) {
        sprintf(hex + (i * 2), "%02X", handle[i]);
    }
    hex[16] = '\0';
    return String(hex);
}

// Convierte una cadena hexadecimal de 16 caracteres a 8 bytes
inline bool hexToHandle(const char* hexStr, uint8_t* outHandle) {
    if (strlen(hexStr) != 16) return false;
    for (int i = 0; i < 8; i++) {
        char byteStr[3] = { hexStr[i * 2], hexStr[i * 2 + 1], '\0' };
        outHandle[i] = (uint8_t)strtoul(byteStr, nullptr, 16);
    }
    return true;
}

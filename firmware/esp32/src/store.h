#pragma once
// =============================================================================
// src/store.h — SoloLoJusto: Lista de revocación en NVS (Preferences)
// =============================================================================

#include <Arduino.h>
#include "contract.h"

#define MAX_REVOKED_HANDLES 256

// Inicializa el almacenamiento NVS y carga la lista de revocación a RAM
void storeInit();

// Comprueba si un handle de 8 bytes está revocado
bool storeIsRevoked(const uint8_t* handle8);

// Reinicia la lista de revocación en NVS
void storeResetRevocations();

// Añade un handle de 8 bytes a la lista de revocación y persiste en NVS
bool storeAddRevocation(const uint8_t* handle8);

// Obtiene la cantidad total de handles revocados actualmente
size_t storeGetRevocationCount();

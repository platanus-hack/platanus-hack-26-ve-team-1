// =============================================================================
// src/store.cpp — Implementación de persistencia NVS con Preferences
// =============================================================================

#include "store.h"
#include <Preferences.h>

static Preferences prefs;
static uint8_t ramRevList[MAX_REVOKED_HANDLES][8];
static size_t revCount = 0;

void storeInit() {
    prefs.begin("slj", false);

    revCount = prefs.getUInt("revn", 0);
    if (revCount > MAX_REVOKED_HANDLES) revCount = MAX_REVOKED_HANDLES;

    if (revCount > 0) {
        size_t bytesRead = prefs.getBytes("rev", ramRevList, revCount * 8);
        revCount = bytesRead / 8;
    }
}

size_t storeGetRevocationCount() {
    return revCount;
}

bool storeIsRevoked(const uint8_t* handle8) {
    for (size_t i = 0; i < revCount; i++) {
        if (memcmp(ramRevList[i], handle8, 8) == 0) {
            return true;
        }
    }
    return false;
}

void storeResetRevocations() {
    revCount = 0;
    prefs.putUInt("revn", 0);
    prefs.remove("rev");
}

bool storeAddRevocation(const uint8_t* handle8) {
    if (storeIsRevoked(handle8)) return true; // Ya estaba
    if (revCount >= MAX_REVOKED_HANDLES) return false; // Límite de capacidad

    memcpy(ramRevList[revCount], handle8, 8);
    revCount++;

    // Guardar en NVS
    prefs.putUInt("revn", revCount);
    prefs.putBytes("rev", ramRevList, revCount * 8);
    return true;
}

static uint8_t stagedList[MAX_REVOKED_HANDLES][8];
static size_t stagedCount = 0;

void storeBeginRevocationSync() {
    stagedCount = 0;
}

bool storeStageRevocation(const uint8_t* handle8) {
    if (stagedCount >= MAX_REVOKED_HANDLES) return false; // Límite de capacidad
    memcpy(stagedList[stagedCount], handle8, 8);
    stagedCount++;
    return true;
}

void storeCommitRevocationSync() {
    // El gateway reenvía la lista cada 60 s: solo se escribe la NVS si cambió
    if (stagedCount == revCount && memcmp(stagedList, ramRevList, revCount * 8) == 0) return;

    memcpy(ramRevList, stagedList, stagedCount * 8);
    revCount = stagedCount;
    prefs.putUInt("revn", revCount);
    if (revCount > 0) prefs.putBytes("rev", ramRevList, revCount * 8);
    else prefs.remove("rev");
}

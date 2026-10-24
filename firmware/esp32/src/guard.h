#pragma once
// =============================================================================
// src/guard.h — SoloLoJusto: Interfaz con el Guardián Físico (PIC16F13145)
// =============================================================================

#include <Arduino.h>
#include "config.h"

// Inicializa los pines del PIC (GPIO 26 HB, GPIO 27 BLOQ)
void guardInit();

// Actualiza el latido de hardware en GPIO 26 y vigila la línea BLOQ en GPIO 27.
// Retorna true si el terminal está actualmente bloqueado por el PIC.
bool guardUpdate();

// Retorna true si el terminal está actualmente bloqueado
bool guardIsBlocked();

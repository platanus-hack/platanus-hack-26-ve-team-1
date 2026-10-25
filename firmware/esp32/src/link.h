#pragma once
// =============================================================================
// src/link.h — SoloLoJusto: Protocolo Serial JSON con Gateway (USB) y UNO
// =============================================================================

#include <Arduino.h>
#include "config.h"
#include "contract.h"

// Inicializa Serial USB (Gateway) y Serial1 (Arduino UNO)
void linkInit();

// Procesa líneas JSON entrantes por Serial USB (Gateway)
void linkProcessInput();

// Getters de configuración del Gateway
String linkGetRol();
uint8_t linkGetAtributoPedido();
String linkGetNombreLocal();
TerminalMode linkGetModo();
uint32_t linkGetCurrentTime();
bool linkHasTimeSync();

// Cuenta las cfg recibidas: cambia con cada una, aunque traiga los mismos valores
uint32_t linkGetCfgVersion();

// Envíos oficiales al Gateway (Sección 4.4)
void linkSendHb();
void linkSendVerif(const String& handleHex, uint8_t atributo, const char* resCode, uint32_t timestamp, bool emergencia);
void linkSendTamper();
void linkSendUid(const String& uidHex);
void linkSendAck(uint8_t sector, uint8_t bloque, bool ok);
void linkSendWritten(bool ok);
void linkSendLink(const String& handleHex);
void linkSendLog(const String& mensaje);

// Modo verificar: lectura aprobada que necesita la autorización del ciudadano. El
// gateway la pide por WhatsApp y responde {"t":"consent","approved":…}
void linkSendPedido(const String& handleHex, uint8_t atributo);

// Envío a la pantalla de comercio del Arduino UNO (Serial1 en GPIO 25)
void linkSendUnoResult(const char* resCode, uint8_t atributo, bool emergencia = false);
void linkSendUnoWait(uint8_t atributo);   // Esperando la respuesta del ciudadano
void linkSendUnoLock();
void linkSendUnoIdle();

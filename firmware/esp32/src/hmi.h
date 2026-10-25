#pragma once
// =============================================================================
// src/hmi.h — SoloLoJusto: Pantalla del ciudadano (Nextion por Serial2)
// =============================================================================
// Las pantallas se dibujan desde el ESP32 (cls, fill, xstr) sobre un proyecto base
// mínimo de Nextion Editor: una página vacía y cuatro fuentes (docs/hmi-nextion.md).
// Los botones también son dibujados: los toques llegan como coordenadas (sendxy=1).

#include <Arduino.h>
#include "config.h"

// Eventos táctiles de los botones dibujados
enum HmiTouchEvent {
    TOUCH_NONE,
    TOUCH_ACEPTAR,         // Solicitud: compartir el dato
    TOUCH_RECHAZAR,        // Solicitud: no compartirlo
    TOUCH_CANCELAR,        // "Acerque su cédula": volver a la solicitud
    TOUCH_MENU_HIDDEN,     // Pulsación larga en la esquina superior izquierda (pantallas de reposo)
    TOUCH_MENU_COMERCIO,   // Menú oculto: terminal de cada rol del backend
    TOUCH_MENU_SALUD,
    TOUCH_MENU_EMERGENCIA,
    TOUCH_MENU_TRANSITO,
    TOUCH_MENU_REGISTRO,
    TOUCH_MENU_VOLVER,
    TOUCH_REINICIO         // La Nextion se reinició por su cuenta: hay que redibujar
};

// Inicializa Serial2 y la pantalla (bkcmd, sendxy, brillo)
void hmiInit();

// Envía un comando Nextion crudo terminado con 0xFF 0xFF 0xFF
void hmiSendCmd(const String& cmd);

// Procesa lo que llega de la Nextion (toques y respuestas); un evento por llamada
HmiTouchEvent hmiCheckTouch();

// Pantallas (docs/p2-roadmap.md, sección 4.5)
void hmiShowSinConexion();                                                  // p0: sin hora del gateway
void hmiShowSolicitud(const String& local, uint8_t atributo);              // p1: reposo del modo verificar
void hmiShowAcerque(const String& local);                                  // p2
void hmiShowVerificando(const String& titulo);                             // p3
void hmiShowResultado(const char* resCode, uint8_t atributo, bool emergencia); // p4
void hmiShowCodigo(const String& codigo6);                                 // p5
void hmiShowMenuOculto();                                                  // p6
void hmiShowBloqueado();                                                   // p7
void hmiShowEmergencia(const String& sangre, const String& alergias, const String& contacto); // p8
void hmiShowEmergenciaEspera();                                            // reposo del modo emergencia
void hmiShowVinculacionEspera();                                           // reposo del modo vincular
void hmiShowEmisionEspera();                                               // reposo del modo emitir
void hmiShowEmisionGrabando();
void hmiShowEmisionFin(bool ok);
void hmiShowAviso(const String& titulo, const String& detalle);

// Pantalla de prueba: fuentes, colores y tildes (Paso 4 del Roadmap)
void hmiTestHello();

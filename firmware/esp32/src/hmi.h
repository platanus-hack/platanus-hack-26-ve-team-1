#pragma once
// =============================================================================
// src/hmi.h — SoloLoJusto: Interfaz con la pantalla Nextion (Serial2)
// =============================================================================

#include <Arduino.h>
#include "config.h"

// Eventos táctiles recibidos de la pantalla Nextion
enum HmiTouchEvent {
    TOUCH_NONE,
    TOUCH_ACEPTAR,     // Botón Aceptar en p1
    TOUCH_RECHAZAR,    // Botón Rechazar en p1
    TOUCH_MENU_HIDDEN, // Toque en zona oculta de p0 -> abre p6
    TOUCH_ROL_BAR,     // Botón Bar en p6
    TOUCH_ROL_HOTEL,   // Botón Hotel en p6
    TOUCH_ROL_HOSPITAL,// Botón Hospital en p6
    TOUCH_ROL_PARAMEDICO, // Botón Paramédico en p6
    TOUCH_MODO_EMITIR, // Botón Emitir en p6
    TOUCH_MODO_VINCULAR// Botón Vincular en p6
};

// Inicializa el puerto Serial2 con la Nextion
void hmiInit();

// Envía un comando Nextion crudo terminado con 0xFF 0xFF 0xFF
void hmiSendCmd(const String& cmd);

// Comprueba y procesa datos recibidos desde la Nextion (toques y respuestas)
HmiTouchEvent hmiCheckTouch();

// Funciones de navegación de páginas Nextion (Sección 4.5)
void hmiShowEspera();                                                  // p0
void hmiShowSolicitud(const String& local, const String& atributo);    // p1
void hmiShowAcerque();                                                 // p2
void hmiShowVerificando();                                             // p3
void hmiShowResultado(bool aprobado, const String& codigoRes);         // p4
void hmiShowCodigo(const String& codigo6);                             // p5
void hmiShowMenuOculto();                                              // p6
void hmiShowBloqueado();                                               // p7
void hmiShowEmergencia(const String& sangre, const String& alergias, const String& contacto); // p8

// Función de prueba "Hola Mundo" para verificar la pantalla (Paso 4 del Roadmap)
void hmiTestHello();

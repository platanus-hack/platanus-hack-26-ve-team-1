// =============================================================================
// src/hmi.cpp — Pantalla del ciudadano: dibujo y toques sobre la Nextion (Serial2)
// =============================================================================

#include "hmi.h"
#include "contract.h"
#include "link.h"

// ─── Colores (RGB565 en decimal, como los pide la Nextion) ───────────────────

static const uint16_t C_FONDO  = 2212;   // #0F1720
static const uint16_t C_BANDA  = 6438;   // #1B2633
static const uint16_t C_TEXTO  = 65535;  // #FFFFFF
static const uint16_t C_TENUE  = 40246;  // #9AA6B4
static const uint16_t C_ACENTO = 15228;  // #3D6FE0
static const uint16_t C_BOTON  = 14890;  // #3A4656
static const uint16_t C_VERDE  = 7403;   // #1E9E5A
static const uint16_t C_ROJO   = 51655;  // #C83A3A
static const uint16_t C_AMBAR  = 58563;  // #E39A1C

// Las pantallas se diseñan sobre una grilla de 480 x 320 y se escalan al modelo
static int X(int v) { return (int32_t)v * HMI_ANCHO / 480; }
static int Y(int v) { return (int32_t)v * HMI_ALTO / 320; }

// ─── Texto ───────────────────────────────────────────────────────────────────

// Los textos del firmware y de la cédula son UTF-8; las fuentes de la Nextion,
// ISO-8859-1. Lo que no existe en Latin-1 sale como '?'. Las comillas se cambian
// por apóstrofos y se quitan los 0xFF, que terminarían el comando.
static String aLatin1(const String& s) {
    String out;
    out.reserve(s.length());
    for (unsigned int i = 0; i < s.length(); i++) {
        uint8_t c = (uint8_t)s[i];
        if (c < 0x80) {
            if (c == '"') out += '\'';
            else if (c >= 32 && c != 127) out += (char)c;
            continue;
        }
        if ((c == 0xC2 || c == 0xC3) && i + 1 < s.length() && ((uint8_t)s[i + 1] & 0xC0) == 0x80) {
            uint8_t latin = (uint8_t)(((c & 0x03) << 6) | ((uint8_t)s[i + 1] & 0x3F));
            i++;
            if (latin >= 0xA0 && latin != 0xFF) out += (char)latin;
            continue;
        }
        while (i + 1 < s.length() && ((uint8_t)s[i + 1] & 0xC0) == 0x80) i++;
        out += '?';
    }
    return out;
}

// Caracteres visibles de un texto UTF-8
static int largoVisible(const String& s) {
    int n = 0;
    for (unsigned int i = 0; i < s.length(); i++) {
        if (((uint8_t)s[i] & 0xC0) != 0x80) n++;
    }
    return n;
}

// La fuente más grande (hasta 'mayor') con la que el texto cabe en 'ancho' (grilla de
// 480): en estas fuentes un carácter mide en promedio la mitad de su alto
static uint8_t fuenteQueCabe(const String& t, int ancho, uint8_t mayor) {
    static const uint8_t alto[] = { 16, 24, 32, 56 };
    for (int f = mayor; f > FUENTE_CHICA; f--) {
        if (largoVisible(t) * alto[f] / 2 <= X(ancho)) return (uint8_t)f;
    }
    return FUENTE_CHICA;
}

void hmiSendCmd(const String& cmd) {
    Serial2.print(cmd);
    Serial2.write(0xFF);
    Serial2.write(0xFF);
    Serial2.write(0xFF);
}

// Texto en una caja con fondo sólido. Alineación: 0 izquierda, 1 centro, 2 derecha
static void texto(int x, int y, int w, int h, uint8_t fuente, uint16_t color, uint16_t fondo,
                  uint8_t alineacion, const String& t) {
    hmiSendCmd("xstr " + String(X(x)) + "," + String(Y(y)) + "," + String(X(w)) + "," + String(Y(h)) + "," +
               String(fuente) + "," + String(color) + "," + String(fondo) + "," + String(alineacion) +
               ",1,1,\"" + aLatin1(t) + "\"");
}

// ─── Formas ──────────────────────────────────────────────────────────────────

static void rect(int x, int y, int w, int h, uint16_t color) {
    hmiSendCmd("fill " + String(X(x)) + "," + String(Y(y)) + "," + String(X(w)) + "," + String(Y(h)) + "," +
               String(color));
}

static void lineaGruesa(int x0, int y0, int x1, int y1, uint16_t color) {
    for (int d = -2; d <= 2; d++) {
        hmiSendCmd("line " + String(X(x0)) + "," + String(Y(y0) + d) + "," + String(X(x1)) + "," +
                   String(Y(y1) + d) + "," + String(color));
    }
}

// Círculo con visto bueno o cruz, centrado en la parte alta de la pantalla
static void simboloResultado(bool ok, uint16_t color) {
    hmiSendCmd("cirs " + String(X(240)) + "," + String(Y(100)) + "," + String(Y(56)) + "," + String(color));
    if (ok) {
        lineaGruesa(212, 100, 232, 122, C_TEXTO);
        lineaGruesa(232, 122, 270, 78, C_TEXTO);
    } else {
        lineaGruesa(216, 76, 264, 124, C_TEXTO);
        lineaGruesa(216, 124, 264, 76, C_TEXTO);
    }
}

// Una tarjeta con su chip, para las pantallas que piden acercarla
static void dibujoTarjeta(uint16_t color) {
    rect(180, 128, 120, 76, color);
    rect(196, 150, 26, 20, C_AMBAR);
}

// ─── Pantallas y botones ─────────────────────────────────────────────────────

struct Boton {
    int16_t x, y, w, h;   // en píxeles de la pantalla
    HmiTouchEvent evento;
};

static Boton botones[8];
static uint8_t nBotones = 0;
static bool zonaOculta = false;   // la pantalla actual abre el menú oculto con una pulsación larga

static void nuevaPantalla(uint16_t fondo, bool conZonaOculta) {
    nBotones = 0;
    zonaOculta = conZonaOculta;
    hmiSendCmd("cls " + String(fondo));
}

static void boton(int x, int y, int w, int h, uint16_t fondo, const String& etiqueta,
                  HmiTouchEvent evento, uint8_t fuente) {
    rect(x, y, w, h, fondo);
    texto(x, y, w, h, fuente, C_TEXTO, fondo, 1, etiqueta);
    if (nBotones < sizeof(botones) / sizeof(botones[0])) {
        botones[nBotones++] = { (int16_t)X(x), (int16_t)Y(y), (int16_t)X(w), (int16_t)Y(h), evento };
    }
}

static void cabecera(const String& titulo, uint16_t fondo, uint16_t color) {
    rect(0, 0, 480, 44, fondo);
    texto(16, 0, 330, 44, fuenteQueCabe(titulo, 330, FUENTE_MEDIA), color, fondo, 0, titulo);
    texto(350, 0, 114, 44, FUENTE_MEDIA, color, fondo, 2, "sIsA");
}

static void configurarPantalla() {
    hmiSendCmd("bkcmd=2");    // La Nextion responde solo los errores
    hmiSendCmd("sendxy=1");   // Toques como coordenadas (eventos 0x67)
    hmiSendCmd("dim=100");
    hmiSendCmd("page 0");
}

void hmiInit() {
    // Configurar Serial2: RX en GPIO 16, TX en GPIO 17. Con un búfer de salida grande,
    // dibujar una pantalla (~500 B a 9600 baudios) no frena el loop()
    Serial2.setTxBufferSize(2048);
    Serial2.begin(NEXTION_BAUD, SERIAL_8N1, PIN_NEXTION_RX, PIN_NEXTION_TX);
    delay(200);

    // Enviar ráfaga de 0xFF para cerrar cualquier comando previo que haya quedado a medias
    Serial2.write(0xFF);
    Serial2.write(0xFF);
    Serial2.write(0xFF);
    delay(50);

    configurarPantalla();
    linkSendLog("Nextion Serial2 inicializado a " + String(NEXTION_BAUD) + " baudios (RX:16, TX:17)");
}

void hmiTestHello() {
    nuevaPantalla(C_FONDO, false);
    texto(0, 40, 480, 70, FUENTE_ENORME, C_TEXTO, C_FONDO, 1, "sIsA");
    texto(0, 116, 480, 40, FUENTE_GRANDE, C_TEXTO, C_FONDO, 1, "Fuente 2: Cédula");
    texto(0, 160, 480, 30, FUENTE_MEDIA, C_TENUE, C_FONDO, 1, "Fuente 1: áéíóú ñ ¿¡ ü");
    texto(0, 194, 480, 24, FUENTE_CHICA, C_TENUE, C_FONDO, 1, "Fuente 0: si ve tildes, la codificación está bien");
    const uint16_t colores[] = { C_VERDE, C_ROJO, C_AMBAR, C_ACENTO, C_BOTON, C_TENUE };
    for (int i = 0; i < 6; i++) {
        rect(36 + i * 72, 240, 56, 56, colores[i]);
    }
    linkSendLog("Nextion: pantalla de prueba enviada (fuentes 0 a 3, colores y tildes)");
}

void hmiShowSinConexion() {
    nuevaPantalla(C_FONDO, true);
    cabecera("Terminal", C_BANDA, C_TENUE);
    texto(0, 110, 480, 48, FUENTE_GRANDE, C_TEXTO, C_FONDO, 1, "Conectando con el sistema...");
    texto(0, 170, 480, 30, FUENTE_CHICA, C_TENUE, C_FONDO, 1, "Sin la hora del gateway no se verifican cédulas");
}

void hmiShowSolicitud(const String& local, uint8_t atributo) {
    nuevaPantalla(C_FONDO, true);
    cabecera(local, C_BANDA, C_TEXTO);
    if (atributo == 0) {
        texto(0, 120, 480, 48, FUENTE_GRANDE, C_TEXTO, C_FONDO, 1, "Este terminal no pide ningún dato");
        return;
    }

    String dato = nombreAtributo(atributo);
    texto(0, 60, 480, 34, FUENTE_MEDIA, C_TENUE, C_FONDO, 1, "Este terminal le pide:");
    texto(0, 96, 480, 70, fuenteQueCabe(dato, 448, FUENTE_ENORME), C_TEXTO, C_FONDO, 1, dato);
    texto(0, 170, 480, 30, FUENTE_CHICA, C_TENUE, C_FONDO, 1, "Solo se comparte este dato, nada más.");
    boton(16, 216, 216, 80, C_BOTON, "Rechazar", TOUCH_RECHAZAR, FUENTE_GRANDE);
    boton(248, 216, 216, 80, C_VERDE, "Aceptar", TOUCH_ACEPTAR, FUENTE_GRANDE);
}

void hmiShowAcerque(const String& local) {
    nuevaPantalla(C_FONDO, false);
    cabecera(local, C_BANDA, C_TEXTO);
    texto(0, 64, 480, 48, fuenteQueCabe("Acerque su cédula al lector", 448, FUENTE_GRANDE),
          C_TEXTO, C_FONDO, 1, "Acerque su cédula al lector");
    dibujoTarjeta(C_ACENTO);
    texto(0, 214, 480, 28, FUENTE_CHICA, C_TENUE, C_FONDO, 1, "Manténgala quieta hasta ver el resultado");
    boton(160, 252, 160, 56, C_BOTON, "Cancelar", TOUCH_CANCELAR, FUENTE_MEDIA);
}

void hmiShowVerificando(const String& titulo) {
    nuevaPantalla(C_FONDO, false);
    cabecera(titulo, C_BANDA, C_TEXTO);
    texto(0, 120, 480, 48, FUENTE_GRANDE, C_TEXTO, C_FONDO, 1, "Verificando...");
    texto(0, 176, 480, 34, FUENTE_MEDIA, C_TENUE, C_FONDO, 1, "No retire la cédula");
}

static const char* tituloResultado(const char* res, bool emergencia) {
    if (strcmp(res, RES_APROBADO) == 0) return emergencia ? "Acceso de emergencia" : "Aprobado";
    if (strcmp(res, RES_FIRMA_FALSA) == 0) return "Cédula no válida";
    if (strcmp(res, RES_REVOCADO) == 0) return "Cédula bloqueada";
    if (strcmp(res, RES_VENCIDO) == 0) return "Cédula vencida";
    if (strcmp(res, RES_SIN_PERMISO) == 0) return "Sin permiso";
    if (strcmp(res, RES_RECHAZADO) == 0) return "No se compartió";
    return "Error de lectura";
}

static String detalleResultado(const char* res, uint8_t atributo) {
    String dato = nombreAtributo(atributo);
    if (strcmp(res, RES_APROBADO) == 0) return "Se compartió: " + dato;
    if (strcmp(res, RES_FIRMA_FALSA) == 0) return "La firma del emisor no coincide";
    if (strcmp(res, RES_REVOCADO) == 0) return "Su titular la bloqueó";
    if (strcmp(res, RES_VENCIDO) == 0) return "Debe renovarla";
    if (strcmp(res, RES_SIN_PERMISO) == 0) return "Este terminal no puede leer: " + dato;
    if (strcmp(res, RES_RECHAZADO) == 0) return "Usted no compartió ningún dato";
    return "";
}

void hmiShowResultado(const char* resCode, uint8_t atributo, bool emergencia) {
    bool aprobado = strcmp(resCode, RES_APROBADO) == 0;
    uint16_t color = aprobado ? (emergencia ? C_AMBAR : C_VERDE) : C_ROJO;

    nuevaPantalla(C_FONDO, false);
    simboloResultado(aprobado, color);
    String titulo = tituloResultado(resCode, emergencia);
    String detalle = detalleResultado(resCode, atributo);
    texto(0, 170, 480, 48, fuenteQueCabe(titulo, 448, FUENTE_GRANDE), color, C_FONDO, 1, titulo);
    texto(0, 222, 480, 34, fuenteQueCabe(detalle, 448, FUENTE_MEDIA), C_TEXTO, C_FONDO, 1, detalle);
    if (aprobado) {
        texto(0, 266, 480, 28, FUENTE_CHICA, C_TENUE, C_FONDO, 1, "Este uso queda en su historial de sIsA");
    }
}

void hmiShowCodigo(const String& codigo6) {
    nuevaPantalla(C_FONDO, false);
    cabecera("Vincular billetera", C_ACENTO, C_TEXTO);
    String legible = codigo6.length() == 6 ? codigo6.substring(0, 3) + " " + codigo6.substring(3) : codigo6;
    texto(0, 64, 480, 34, FUENTE_MEDIA, C_TENUE, C_FONDO, 1, "Su código de vinculación");
    texto(0, 104, 480, 80, FUENTE_ENORME, C_TEXTO, C_FONDO, 1, legible);
    texto(0, 196, 480, 30, FUENTE_MEDIA, C_TEXTO, C_FONDO, 1, "Escríbalo en su billetera");
    texto(0, 230, 480, 28, FUENTE_CHICA, C_TENUE, C_FONDO, 1, "Vence en 2 minutos");
}

void hmiShowMenuOculto() {
    nuevaPantalla(C_FONDO, false);
    cabecera("Demo: elegir terminal", C_BANDA, C_TEXTO);
    boton(16, 58, 140, 100, C_BOTON, "Bar", TOUCH_MENU_COMERCIO, FUENTE_MEDIA);
    boton(170, 58, 140, 100, C_BOTON, "Clínica", TOUCH_MENU_SALUD, FUENTE_MEDIA);
    boton(324, 58, 140, 100, C_BOTON, "Ambulancia", TOUCH_MENU_EMERGENCIA, FUENTE_MEDIA);
    boton(16, 170, 140, 100, C_BOTON, "Tránsito", TOUCH_MENU_TRANSITO, FUENTE_MEDIA);
    boton(170, 170, 140, 100, C_BOTON, "Registro", TOUCH_MENU_REGISTRO, FUENTE_MEDIA);
    boton(324, 170, 140, 100, C_BANDA, "Volver", TOUCH_MENU_VOLVER, FUENTE_MEDIA);
    texto(0, 282, 480, 30, FUENTE_CHICA, C_TENUE, C_FONDO, 1, "El gateway cambia el terminal y su configuración");
}

void hmiShowBloqueado() {
    nuevaPantalla(C_ROJO, false);
    texto(0, 70, 480, 70, FUENTE_ENORME, C_TEXTO, C_ROJO, 1, "Terminal");
    texto(0, 140, 480, 70, FUENTE_ENORME, C_TEXTO, C_ROJO, 1, "bloqueado");
    texto(0, 230, 480, 34, FUENTE_MEDIA, C_TEXTO, C_ROJO, 1, "Se detectó una manipulación");
}

void hmiShowEmergencia(const String& sangre, const String& alergias, const String& contacto) {
    nuevaPantalla(C_FONDO, false);
    cabecera("Datos de emergencia", C_AMBAR, C_FONDO);
    String a = alergias.length() > 0 ? alergias : String("Ninguna registrada");
    String c = contacto.length() > 0 ? contacto : String("Sin dato");
    texto(16, 54, 200, 28, FUENTE_MEDIA, C_TENUE, C_FONDO, 0, "Sangre");
    texto(16, 82, 448, 64, FUENTE_ENORME, C_TEXTO, C_FONDO, 0, sangre.length() > 0 ? sangre : String("Sin dato"));
    texto(16, 150, 200, 28, FUENTE_MEDIA, C_TENUE, C_FONDO, 0, "Alergias");
    texto(16, 178, 448, 40, fuenteQueCabe(a, 448, FUENTE_GRANDE), C_TEXTO, C_FONDO, 0, a);
    texto(16, 226, 200, 28, FUENTE_MEDIA, C_TENUE, C_FONDO, 0, "Contacto");
    texto(16, 254, 448, 40, fuenteQueCabe(c, 448, FUENTE_GRANDE), C_TEXTO, C_FONDO, 0, c);
}

void hmiShowEmergenciaEspera() {
    nuevaPantalla(C_FONDO, true);
    cabecera("Modo emergencia", C_AMBAR, C_FONDO);
    texto(0, 66, 480, 48, fuenteQueCabe("Acerque la cédula del paciente", 448, FUENTE_GRANDE),
          C_TEXTO, C_FONDO, 1, "Acerque la cédula del paciente");
    dibujoTarjeta(C_AMBAR);
    texto(0, 222, 480, 26, FUENTE_CHICA, C_TENUE, C_FONDO, 1, "Se leen sangre, alergias y contacto sin pedir permiso.");
    texto(0, 250, 480, 26, FUENTE_CHICA, C_TENUE, C_FONDO, 1, "La lectura queda en el historial del titular.");
}

void hmiShowVinculacionEspera() {
    nuevaPantalla(C_FONDO, true);
    cabecera("Vincular billetera", C_ACENTO, C_TEXTO);
    texto(0, 66, 480, 48, FUENTE_GRANDE, C_TEXTO, C_FONDO, 1, "Acerque su cédula");
    dibujoTarjeta(C_ACENTO);
    texto(0, 226, 480, 30, FUENTE_CHICA, C_TENUE, C_FONDO, 1, "Recibirá un código para su billetera digital");
}

void hmiShowEmisionEspera() {
    nuevaPantalla(C_FONDO, true);
    cabecera("Emisión de cédulas", C_ACENTO, C_TEXTO);
    texto(0, 66, 480, 48, fuenteQueCabe("Acerque una tarjeta virgen", 448, FUENTE_GRANDE),
          C_TEXTO, C_FONDO, 1, "Acerque una tarjeta virgen");
    dibujoTarjeta(C_BOTON);
    texto(0, 222, 480, 26, FUENTE_CHICA, C_TENUE, C_FONDO, 1, "Pida antes la emisión en el panel del emisor.");
    texto(0, 250, 480, 26, FUENTE_CHICA, C_TENUE, C_FONDO, 1, "La solicitud vale 10 minutos.");
}

void hmiShowEmisionGrabando() {
    nuevaPantalla(C_FONDO, false);
    cabecera("Emisión de cédulas", C_ACENTO, C_TEXTO);
    texto(0, 110, 480, 48, FUENTE_GRANDE, C_TEXTO, C_FONDO, 1, "Grabando la cédula...");
    texto(0, 170, 480, 34, FUENTE_MEDIA, C_AMBAR, C_FONDO, 1, "No retire la tarjeta");
}

void hmiShowEmisionFin(bool ok) {
    uint16_t color = ok ? C_VERDE : C_ROJO;
    nuevaPantalla(C_FONDO, false);
    simboloResultado(ok, color);
    texto(0, 170, 480, 48, FUENTE_GRANDE, color, C_FONDO, 1, ok ? "Cédula emitida" : "No se pudo emitir");
    texto(0, 222, 480, 34, FUENTE_MEDIA, C_TEXTO, C_FONDO, 1,
          ok ? "Datos firmados y tarjeta sellada" : "Revise el gateway y vuelva a intentarlo");
}

void hmiShowAviso(const String& titulo, const String& detalle) {
    nuevaPantalla(C_FONDO, false);
    cabecera("Terminal", C_BANDA, C_TENUE);
    texto(0, 110, 480, 48, fuenteQueCabe(titulo, 448, FUENTE_GRANDE), C_TEXTO, C_FONDO, 1, titulo);
    texto(0, 170, 480, 34, fuenteQueCabe(detalle, 448, FUENTE_MEDIA), C_TENUE, C_FONDO, 1, detalle);
}

// ─── Toques y respuestas de la Nextion ───────────────────────────────────────

static uint8_t trama[16];
static uint8_t tramaLargo = 0;
static uint8_t finales = 0;   // 0xFF seguidos al final de la trama

static bool presionado = false;
static int16_t pressX = 0, pressY = 0;
static uint32_t pressMs = 0;

// Un botón responde al soltar, en el punto donde empezó el toque
static HmiTouchEvent tocar(int16_t x, int16_t y, bool presion) {
    if (presion) {
        presionado = true;
        pressX = x;
        pressY = y;
        pressMs = millis();
        return TOUCH_NONE;
    }
    if (!presionado) {
        pressX = x;
        pressY = y;
        pressMs = millis();
    }
    presionado = false;

    if (zonaOculta && pressX < X(MENU_ZONA_PX) && pressY < Y(MENU_ZONA_PX) &&
        millis() - pressMs >= MENU_HOLD_MS) {
        return TOUCH_MENU_HIDDEN;
    }
    for (uint8_t i = 0; i < nBotones; i++) {
        const Boton& b = botones[i];
        if (pressX >= b.x && pressX < b.x + b.w && pressY >= b.y && pressY < b.y + b.h) {
            return b.evento;
        }
    }
    return TOUCH_NONE;
}

// Informa los errores de la Nextion (bkcmd=2), a lo sumo uno igual cada 5 s
static void avisarError(uint8_t codigo) {
    static uint32_t ultimo = 0;
    static uint8_t ultimoCodigo = 0xFF;
    if (codigo == ultimoCodigo && millis() - ultimo < 5000) return;
    ultimo = millis();
    ultimoCodigo = codigo;

    const char* que = "error";
    switch (codigo) {
        case 0x00: que = "instrucción inválida"; break;
        case 0x05: que = "fuente inexistente: revisar las fuentes del proyecto base"; break;
        case 0x1A: que = "variable o componente inexistente"; break;
        case 0x1E: que = "cantidad de parámetros inválida"; break;
        case 0x24: que = "búfer de la Nextion lleno"; break;
    }
    char buf[112];
    snprintf(buf, sizeof(buf), "Nextion devolvió 0x%02X (%s)", codigo, que);
    linkSendLog(String(buf));
}

static HmiTouchEvent interpretarTrama() {
    uint8_t tipo = trama[0];

    // Coordenadas de un toque (sendxy=1): 0x67 xH xL yH yL evento FF FF FF
    if (tipo == 0x67 || tipo == 0x68) {
        if (trama[6] != 0xFF || trama[7] != 0xFF || trama[8] != 0xFF) return TOUCH_NONE;
        int16_t x = (int16_t)((trama[1] << 8) | trama[2]);
        int16_t y = (int16_t)((trama[3] << 8) | trama[4]);
        return tocar(x, y, trama[5] == 0x01);
    }

    // La Nextion terminó de arrancar: se reinició por su cuenta y perdió sendxy
    if (tipo == 0x88) {
        configurarPantalla();
        return TOUCH_REINICIO;
    }

    // Códigos de retorno estándar (4 bytes: <code> 0xFF 0xFF 0xFF)
    if (tramaLargo == 4 && tipo <= 0x24 && tipo != 0x01) {
        avisarError(tipo);
    }
    return TOUCH_NONE;
}

HmiTouchEvent hmiCheckTouch() {
    while (Serial2.available()) {
        uint8_t b = (uint8_t)Serial2.read();
        if (tramaLargo >= sizeof(trama)) {
            tramaLargo = 0; // Trama demasiado larga: basura, resincronizar
            finales = 0;
        }
        trama[tramaLargo++] = b;
        finales = (b == 0xFF) ? finales + 1 : 0;

        // Las coordenadas pueden traer bytes 0xFF: esas tramas se cortan por largo (9 B)
        bool coordenadas = trama[0] == 0x67 || trama[0] == 0x68;
        bool completa = coordenadas ? (tramaLargo == 9) : (finales >= 3);
        if (!completa) continue;

        HmiTouchEvent ev = interpretarTrama();
        tramaLargo = 0;
        finales = 0;
        if (ev != TOUCH_NONE) return ev;
    }
    return TOUCH_NONE;
}

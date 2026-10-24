#include <Arduino.h>
#include <Adafruit_GFX.h>
#include <MCUFRIEND_kbv.h>
#include <ArduinoJson.h>

/*
 * Pantalla del comercio.
 *
 * Recibe del lector una línea JSON por Serial (pin 0, 9600 baudios) y muestra
 * el resultado. Mensajes: docs/contrato.md, sección 3.
 * Para probar sin el lector: monitor serie de PlatformIO y escribir el JSON.
 * Para subir firmware: desconectar el cable del pin 0.
 *
 * Texto con la fuente clásica de Adafruit GFX en CP437:
 * \x82 = é, \xA0 = á, \xA1 = í, \xA2 = ó, \xA3 = ú.
 */

#define NEGRO   0x0000
#define BLANCO  0xFFFF
#define GRIS    0x8410
#define VERDE   0x05E0
#define ROJO    0xC000
#define AMBAR   0xFC00

const uint32_t MOSTRAR_MS = 6000;   // tiempo del resultado antes de volver a espera

MCUFRIEND_kbv tft;

char linea[96];
uint8_t largo = 0;
bool bloqueado = false;
uint32_t resultado_desde = 0;       // 0 = no hay resultado en pantalla

void centrado(const __FlashStringHelper *texto, int y, uint8_t tam, uint16_t color)
{
    int ancho = strlen_P((PGM_P)texto) * 6 * tam;
    tft.setTextSize(tam);
    tft.setTextColor(color);
    tft.setCursor((tft.width() - ancho) / 2, y);
    tft.print(texto);
}

void lineaGruesa(int x0, int y0, int x1, int y1, uint16_t color)
{
    for (int d = -6; d <= 6; d++)
    {
        tft.drawLine(x0, y0 + d, x1, y1 + d, color);
    }
}

const __FlashStringHelper *nombreAtributo(const char *a)
{
    if (!strcmp(a, "A1")) return F("Mayor de 18");
    if (!strcmp(a, "A2")) return F("Mayor de 21");
    if (!strcmp(a, "A3")) return F("Identidad");
    if (!strcmp(a, "A4")) return F("Sangre y alergias");
    if (!strcmp(a, "A5")) return F("Contacto de emergencia");
    if (!strcmp(a, "A6")) return F("Donante y licencia");
    if (!strcmp(a, "A7")) return F("Carnet universitario");
    return F("Dato desconocido");
}

const __FlashStringHelper *textoResultado(const char *res, bool emg)
{
    if (!strcmp(res, "01")) return emg ? F("Acceso de emergencia") : F("Aprobado");
    if (!strcmp(res, "02")) return F("Firma inv\xA0lida");
    if (!strcmp(res, "03")) return F("C\x82" "dula revocada");
    if (!strcmp(res, "04")) return F("C\x82" "dula vencida");
    if (!strcmp(res, "05")) return F("Sin permiso");
    if (!strcmp(res, "06")) return F("Rechazado por el titular");
    if (!strcmp(res, "07")) return F("QR caducado o repetido");
    if (!strcmp(res, "08")) return F("QR no reconocido");
    return F("Error de lectura");
}

void pie()
{
    centrado(F("0 datos personales guardados"), 290, 2, GRIS);
}

void pantallaEspera()
{
    bloqueado = false;
    resultado_desde = 0;
    tft.fillScreen(NEGRO);
    centrado(F("sIsA"), 40, 8, BLANCO);
    centrado(F("Esperando c\x82" "dula..."), 150, 3, GRIS);
    pie();
}

void pantallaResultado(const char *res, const char *a, bool emg)
{
    bool aprobado = !strcmp(res, "01");
    uint16_t color = aprobado ? (emg ? AMBAR : VERDE) : ROJO;
    int cx = tft.width() / 2;
    int cy = 90;

    tft.fillScreen(NEGRO);
    tft.fillCircle(cx, cy, 65, color);
    if (aprobado)
    {
        lineaGruesa(cx - 35, cy, cx - 10, cy + 25, BLANCO);   // ✓
        lineaGruesa(cx - 10, cy + 25, cx + 35, cy - 25, BLANCO);
    }
    else
    {
        lineaGruesa(cx - 28, cy - 28, cx + 28, cy + 28, BLANCO);   // ✗
        lineaGruesa(cx - 28, cy + 28, cx + 28, cy - 28, BLANCO);
    }

    centrado(textoResultado(res, emg), 180, 3, color);
    centrado(nombreAtributo(a), 230, 3, BLANCO);
    pie();
    resultado_desde = millis() | 1;   // nunca 0
}

void pantallaBloqueo()
{
    bloqueado = true;
    resultado_desde = 0;
    tft.fillScreen(ROJO);
    centrado(F("TERMINAL"), 90, 6, BLANCO);
    centrado(F("BLOQUEADO"), 160, 6, BLANCO);
    centrado(F("Se detect\xA2 una manipulaci\xA2n"), 250, 2, BLANCO);
}

void procesar(const char *texto)
{
    JsonDocument doc;
    if (deserializeJson(doc, texto))
    {
        return;   // línea que no es JSON: se ignora
    }

    const char *t = doc["t"] | "";
    if (!strcmp(t, "lock"))
    {
        pantallaBloqueo();
    }
    else if (!strcmp(t, "idle"))
    {
        pantallaEspera();
    }
    else if (!strcmp(t, "res") && !bloqueado)
    {
        pantallaResultado(doc["res"] | "", doc["a"] | "", doc["emg"] | false);
    }
}

void setup()
{
    Serial.begin(9600);

    uint16_t id = tft.readID();
    if (id == 0xD3D3)
    {
        id = 0x9486;   // shield de solo escritura: se asume ILI9486
    }
    tft.begin(id);
    tft.setRotation(1);   // horizontal, 480 x 320
    tft.cp437(true);

    Serial.print(F("ID pantalla: 0x"));
    Serial.println(id, HEX);
    pantallaEspera();
}

void loop()
{
    while (Serial.available())
    {
        char c = Serial.read();
        if (c == '\n')
        {
            linea[largo] = '\0';
            procesar(linea);
            largo = 0;
        }
        else if (c != '\r')
        {
            if (largo < sizeof(linea) - 1)
            {
                linea[largo++] = c;
            }
            else
            {
                largo = 0;   // línea demasiado larga: se descarta
            }
        }
    }

    if (resultado_desde && millis() - resultado_desde >= MOSTRAR_MS)
    {
        pantallaEspera();
    }
}

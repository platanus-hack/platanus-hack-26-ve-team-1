// =============================================================================
// src/hmi.cpp — Implementación de comunicación con pantalla Nextion (Serial2)
// =============================================================================

#include "hmi.h"
#include "link.h"

void hmiSendCmd(const String& cmd) {
    Serial2.print(cmd);
    Serial2.write(0xFF);
    Serial2.write(0xFF);
    Serial2.write(0xFF);
}

void hmiInit() {
    // Configurar Serial2: RX en GPIO 16, TX en GPIO 17 a 9600 baudios
    Serial2.begin(NEXTION_BAUD, SERIAL_8N1, PIN_NEXTION_RX, PIN_NEXTION_TX);
    delay(200);

    // Enviar ráfaga de 0xFF para despertar a la Nextion y limpiar cualquier comando previo
    Serial2.write(0xFF);
    Serial2.write(0xFF);
    Serial2.write(0xFF);
    delay(50);

    // Asegurar brillo al 100%
    hmiSendCmd("dim=100");
    delay(50);

    // Mostrar página de espera inicial (p0 / page 0)
    hmiShowEspera();
    linkSendLog("Nextion Serial2 inicializado a 9600 baudios (RX:16, TX:17)");
}

void hmiTestHello() {
    // Limpiar buffer y forzar página 0
    Serial2.write(0xFF);
    Serial2.write(0xFF);
    Serial2.write(0xFF);
    delay(30);

    hmiSendCmd("dim=100");
    delay(30);

    hmiSendCmd("page 0");
    delay(60);

    hmiSendCmd("t0.txt=\"SoloLoJusto OK!\"");
    linkSendLog("Nextion: Enviado 'page 0' y 't0.txt=\"SoloLoJusto OK!\"'");
}

void hmiShowEspera() {
    hmiSendCmd("page 0");
    delay(40);
    hmiSendCmd("t0.txt=\"Acerque su cedula cuando se le pida\"");
}

void hmiShowSolicitud(const String& local, const String& atributo) {
    hmiSendCmd("page 1");
    delay(40);
    hmiSendCmd("tLocal.txt=\"" + local + "\"");
    hmiSendCmd("tAttr.txt=\"" + atributo + "\"");
    hmiSendCmd("t0.txt=\"" + local + " pide: " + atributo + "\"");
}

void hmiShowAcerque() {
    hmiSendCmd("page 2");
}

void hmiShowVerificando() {
    hmiSendCmd("page 3");
}

void hmiShowResultado(bool aprobado, const String& codigoRes) {
    hmiSendCmd("page 4");
    delay(40);
    hmiSendCmd("tIcon.txt=\"" + String(aprobado ? "OK" : "X") + "\"");
    hmiSendCmd("tCod.txt=\"Cod: " + codigoRes + "\"");
}

void hmiShowCodigo(const String& codigo6) {
    hmiSendCmd("page 5");
    delay(40);
    hmiSendCmd("tCode.txt=\"" + codigo6 + "\"");
}

void hmiShowMenuOculto() {
    hmiSendCmd("page 6");
}

void hmiShowBloqueado() {
    hmiSendCmd("page 7");
}

void hmiShowEmergencia(const String& sangre, const String& alergias, const String& contacto) {
    hmiSendCmd("page 8");
    delay(40);
    hmiSendCmd("tSangre.txt=\"" + sangre + "\"");
    hmiSendCmd("tAlergias.txt=\"" + alergias + "\"");
    hmiSendCmd("tContacto.txt=\"" + contacto + "\"");
}

HmiTouchEvent hmiCheckTouch() {
    if (!Serial2.available()) return TOUCH_NONE;

    // Procesar tramas de respuesta y eventos táctiles de la Nextion
    while (Serial2.available()) {
        uint8_t firstByte = Serial2.peek();

        // 1. Trama de Evento Táctil (7 bytes: 0x65 <page> <comp> <event> 0xFF 0xFF 0xFF)
        if (firstByte == 0x65) {
            if (Serial2.available() < 7) {
                // Esperar a que lleguen los 7 bytes completos
                break;
            }

            uint8_t frame[7];
            Serial2.readBytes(frame, 7);

            if (frame[4] == 0xFF && frame[5] == 0xFF && frame[6] == 0xFF) {
                uint8_t page = frame[1];
                uint8_t comp = frame[2];
                uint8_t ev   = frame[3];

                char logBuf[64];
                snprintf(logBuf, sizeof(logBuf), "Nextion touch: p=%u c=%u ev=%u", page, comp, ev);
                linkSendLog(String(logBuf));

                // Despacho de eventos según página
                if (page == 1 || page == 0x01) {
                    if (comp == 1 || comp == 2) return TOUCH_ACEPTAR;
                    if (comp == 3 || comp == 4) return TOUCH_RECHAZAR;
                } else if (page == 0 && comp == 99) {
                    return TOUCH_MENU_HIDDEN;
                } else if (page == 6 || page == 0x06) {
                    switch (comp) {
                        case 1: return TOUCH_ROL_BAR;
                        case 2: return TOUCH_ROL_HOTEL;
                        case 3: return TOUCH_ROL_HOSPITAL;
                        case 4: return TOUCH_ROL_PARAMEDICO;
                        case 5: return TOUCH_MODO_EMITIR;
                        case 6: return TOUCH_MODO_VINCULAR;
                    }
                }
            }
        }
        // 2. Códigos de retorno estándar de Nextion (4 bytes: <code> 0xFF 0xFF 0xFF)
        else if (firstByte <= 0x24) {
            if (Serial2.available() < 4) break;

            uint8_t resFrame[4];
            Serial2.readBytes(resFrame, 4);

            if (resFrame[1] == 0xFF && resFrame[2] == 0xFF && resFrame[3] == 0xFF) {
                if (resFrame[0] == 0x00) linkSendLog("Nextion err: Instruccion invalida (0x00)");
                else if (resFrame[0] == 0x03) linkSendLog("Nextion err: ID de pagina invalida (0x03)");
                else if (resFrame[0] == 0x1A) linkSendLog("Nextion err: Nombre de componente inexistente (0x1A)");
            }
        }
        // Descartar bytes sueltos o ruido
        else {
            Serial2.read();
        }
    }

    return TOUCH_NONE;
}

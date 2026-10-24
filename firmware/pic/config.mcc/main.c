#include "mcc_generated_files/system/system.h"
#include <stdbool.h>
#include <stdint.h>

/*
 * Guardián del terminal.
 *
 * El latch vive en CLC1 (modo S-R latch, hardware, asíncrono):
 *   SET   = TAPA_IN | FW_SET
 *   RESET = NOR(REARME_IN, TAPA_IN, FW_SET)  -> rearme solo si no hay manipulación
 *   Salida de CLC1 invertida (= ~q) por PPS a HAB_LECTOR (RB6) y LED0 (RC2).
 * FW_SET (RC0) es una salida del firmware que vuelve a CLC1 por CLCIN2PPS:
 * se activa con timeout de latido o con SW0. BLOQ_OUT lo copia el firmware de CLC1.
 *
 * El CLC1 y su PPS se configuran aquí por registros (MCC no genera el CLC).
 *
 * Respaldo: LATCH_POR_FIRMWARE hace lo mismo por software, con HAB_LECTOR,
 * BLOQ_OUT y LED0 como GPIO de salida en MCC (sin CLC).
 */
// #define LATCH_POR_FIRMWARE

#define TICK_MS          10
#define HB_TIMEOUT_TICKS (10000 / TICK_MS)   // 10 s sin flanco = bloqueo

#ifndef LATCH_POR_FIRMWARE
// Códigos PPS de entrada: puerto A = 0x00-0x07, B = 0x08-0x0F, C = 0x10-0x17
#define PPS_IN_RA2   0x02
#define PPS_IN_RA5   0x05
#define PPS_IN_RC0   0x10
#define PPS_OUT_CLC1 0x01

static void GUARDIAN_CLC1_Initialize(void)
{
    CLCSELECT = 0x00;       // los registros CLCn* apuntan a CLC1
    CLCnCON   = 0x00;       // apagado mientras se configura

    CLCnSEL0  = 0x00;       // D1 = CLCIN0 = TAPA_IN   (ALTO = precinto abierto)
    CLCnSEL1  = 0x01;       // D2 = CLCIN1 = REARME_IN (BAJO = rearme pulsado)
    CLCnSEL2  = 0x02;       // D3 = CLCIN2 = FW_SET    (ALTO = timeout o SW0)
    CLCnSEL3  = 0x00;       // D4 sin usar

    CLCnGLS0  = 0x22;       // G1 = D1 | D3            -> S
    CLCnGLS1  = 0x00;       // G2 = 0
    CLCnGLS2  = 0x2A;       // G3 = D1 | D2 | D3 (invertida abajo) -> R
    CLCnGLS3  = 0x00;       // G4 = 0

    CLCnPOL   = 0x84;       // salida invertida (= ~q) y G3 invertida (NOR)
    CLCnCON   = 0x83;       // EN, modo 3 = S-R latch

    CLCIN0PPS = PPS_IN_RA5;
    CLCIN1PPS = PPS_IN_RA2;
    CLCIN2PPS = PPS_IN_RC0;
    RB6PPS    = PPS_OUT_CLC1;   // HAB_LECTOR
    RC2PPS    = PPS_OUT_CLC1;   // LED0

    // Bloquea el PPS hasta el próximo reset (PPS1WAY = ON)
    uint8_t gie = INTCONbits.GIE;
    INTCONbits.GIE = 0;
    PPSLOCK = 0x55;
    PPSLOCK = 0xAA;
    PPSLOCKbits.PPSLOCKED = 1;
    INTCONbits.GIE = gie;
}
#endif

int main(void)
{
    SYSTEM_Initialize();
#ifndef LATCH_POR_FIRMWARE
    FW_SET_SetLow();
    GUARDIAN_CLC1_Initialize();
#endif

    // Se fuerzan aquí por si MCC los pierde al regenerar
    TAPA_IN_SetPullup();
    REARME_IN_SetPullup();
    SW0_SetPullup();
    HB_IN_ResetPullup();      // el ESP32 es de 3,3 V: no subirle 5 V
    BLOQ_OUT_SetOpenDrain();  // nunca saca 5 V hacia el ESP32
    BLOQ_OUT_ResetPullup();

    uint8_t  hb_anterior = HB_IN_GetValue();
    uint16_t ticks_sin_hb = 0;
#ifdef LATCH_POR_FIRMWARE
    bool bloqueado = false;
#endif

    while (1)
    {
        __delay_ms(TICK_MS);

        // Cada flanco del latido (subida o bajada) reinicia la cuenta
        uint8_t hb = HB_IN_GetValue();
        if (hb != hb_anterior)
        {
            hb_anterior = hb;
            ticks_sin_hb = 0;
        }
        else if (ticks_sin_hb < HB_TIMEOUT_TICKS)
        {
            ticks_sin_hb++;
        }

        bool rearme = (REARME_IN_GetValue() == 0);   // botón a GND
        if (rearme)
        {
            ticks_sin_hb = 0;   // al rearmar, el latido tiene otros 10 s para aparecer
        }

        bool timeout_hb = (ticks_sin_hb >= HB_TIMEOUT_TICKS);
        bool sw0        = (SW0_GetValue() == 0);       // pruebas: simula manipulación

#ifndef LATCH_POR_FIRMWARE
        if (timeout_hb || sw0)
        {
            FW_SET_SetHigh();
        }
        else
        {
            FW_SET_SetLow();
        }

        bool bloqueado = (CLCDATAbits.CLC1OUT == 0);   // la salida de CLC1 es ~q
#else
        bool tapa = (TAPA_IN_GetValue() == 1);       // precinto abierto
        bool set  = tapa || timeout_hb || sw0;

        if (set)
        {
            bloqueado = true;   // gana SET: no se rearma con manipulación activa
        }
        else if (rearme)
        {
            bloqueado = false;
        }

        if (bloqueado)
        {
            HAB_LECTOR_SetLow();    // RC522 sin VCC
            LED0_SetLow();          // LED0 activo en BAJO
        }
        else
        {
            HAB_LECTOR_SetHigh();
            LED0_SetHigh();
        }
#endif

        if (bloqueado)
        {
            BLOQ_OUT_SetHigh();     // drenador abierto: suelta la línea -> 3,3 V en el ESP32
        }
        else
        {
            BLOQ_OUT_SetLow();
        }
    }
}

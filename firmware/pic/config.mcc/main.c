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
 * Respaldo: LATCH_POR_FIRMWARE hace lo mismo por software, con HAB_LECTOR,
 * BLOQ_OUT y LED0 como GPIO de salida en MCC (sin CLC).
 */
#define LATCH_POR_FIRMWARE

#define TICK_MS          10
#define HB_TIMEOUT_TICKS (10000 / TICK_MS)   // 10 s sin flanco = bloqueo

int main(void)
{
    SYSTEM_Initialize();

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
            HAB_LECTOR_SetLow();    // PN532 sin VCC
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

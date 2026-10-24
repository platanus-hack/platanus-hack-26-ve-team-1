#include "mcc_generated_files/system/system.h"

int main(void)
{
    SYSTEM_Initialize();          // reloj, pines y bits de config que pusiste en MCC

    while (1)
    {
        if (SW0_GetValue() == 0)  // botón pulsado (pull-up -> 0)
        {
            LED0_SetHigh();       // LED fijo mientras pulsas
        }
        else
        {
            LED0_Toggle();        // si no, parpadea
            __delay_ms(250);
        }
    }
}
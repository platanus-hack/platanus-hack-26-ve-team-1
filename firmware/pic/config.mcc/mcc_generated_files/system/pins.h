/**
 * Generated Pins header File
 * 
 * @file pins.h
 * 
 * @defgroup  pinsdriver Pins Driver
 * 
 * @brief This is generated driver header for pins. 
 *        This header file provides APIs for all pins selected in the GUI.
 *
 * @version Driver Version  3.0.0
*/

/*
© [2026] Microchip Technology Inc. and its subsidiaries.

    Subject to your compliance with these terms, you may use Microchip 
    software and any derivatives exclusively with Microchip products. 
    You are responsible for complying with 3rd party license terms  
    applicable to your use of 3rd party software (including open source  
    software) that may accompany Microchip software. SOFTWARE IS ?AS IS.? 
    NO WARRANTIES, WHETHER EXPRESS, IMPLIED OR STATUTORY, APPLY TO THIS 
    SOFTWARE, INCLUDING ANY IMPLIED WARRANTIES OF NON-INFRINGEMENT,  
    MERCHANTABILITY, OR FITNESS FOR A PARTICULAR PURPOSE. IN NO EVENT 
    WILL MICROCHIP BE LIABLE FOR ANY INDIRECT, SPECIAL, PUNITIVE, 
    INCIDENTAL OR CONSEQUENTIAL LOSS, DAMAGE, COST OR EXPENSE OF ANY 
    KIND WHATSOEVER RELATED TO THE SOFTWARE, HOWEVER CAUSED, EVEN IF 
    MICROCHIP HAS BEEN ADVISED OF THE POSSIBILITY OR THE DAMAGES ARE 
    FORESEEABLE. TO THE FULLEST EXTENT ALLOWED BY LAW, MICROCHIP?S 
    TOTAL LIABILITY ON ALL CLAIMS RELATED TO THE SOFTWARE WILL NOT 
    EXCEED AMOUNT OF FEES, IF ANY, YOU PAID DIRECTLY TO MICROCHIP FOR 
    THIS SOFTWARE.
*/

#ifndef PINS_H
#define PINS_H

#include <xc.h>

#define INPUT   1
#define OUTPUT  0

#define HIGH    1
#define LOW     0

#define ANALOG      1
#define DIGITAL     0

#define PULL_UP_ENABLED      1
#define PULL_UP_DISABLED     0

// get/set IO_RA0 aliases
#define HB_IN_TRIS                 TRISAbits.TRISA0
#define HB_IN_LAT                  LATAbits.LATA0
#define HB_IN_PORT                 PORTAbits.RA0
#define HB_IN_WPU                  WPUAbits.WPUA0
#define HB_IN_OD                   ODCONAbits.ODCA0
#define HB_IN_ANS                  ANSELAbits.ANSA0
#define HB_IN_SetHigh()            do { LATAbits.LATA0 = 1; } while(0)
#define HB_IN_SetLow()             do { LATAbits.LATA0 = 0; } while(0)
#define HB_IN_Toggle()             do { LATAbits.LATA0 = ~LATAbits.LATA0; } while(0)
#define HB_IN_GetValue()           PORTAbits.RA0
#define HB_IN_SetDigitalInput()    do { TRISAbits.TRISA0 = 1; } while(0)
#define HB_IN_SetDigitalOutput()   do { TRISAbits.TRISA0 = 0; } while(0)
#define HB_IN_SetPullup()          do { WPUAbits.WPUA0 = 1; } while(0)
#define HB_IN_ResetPullup()        do { WPUAbits.WPUA0 = 0; } while(0)
#define HB_IN_SetPushPull()        do { ODCONAbits.ODCA0 = 0; } while(0)
#define HB_IN_SetOpenDrain()       do { ODCONAbits.ODCA0 = 1; } while(0)
#define HB_IN_SetAnalogMode()      do { ANSELAbits.ANSA0 = 1; } while(0)
#define HB_IN_SetDigitalMode()     do { ANSELAbits.ANSA0 = 0; } while(0)
// get/set IO_RA1 aliases
#define TAPA_IN_TRIS                 TRISAbits.TRISA1
#define TAPA_IN_LAT                  LATAbits.LATA1
#define TAPA_IN_PORT                 PORTAbits.RA1
#define TAPA_IN_WPU                  WPUAbits.WPUA1
#define TAPA_IN_OD                   ODCONAbits.ODCA1
#define TAPA_IN_ANS                  ANSELAbits.ANSA1
#define TAPA_IN_SetHigh()            do { LATAbits.LATA1 = 1; } while(0)
#define TAPA_IN_SetLow()             do { LATAbits.LATA1 = 0; } while(0)
#define TAPA_IN_Toggle()             do { LATAbits.LATA1 = ~LATAbits.LATA1; } while(0)
#define TAPA_IN_GetValue()           PORTAbits.RA1
#define TAPA_IN_SetDigitalInput()    do { TRISAbits.TRISA1 = 1; } while(0)
#define TAPA_IN_SetDigitalOutput()   do { TRISAbits.TRISA1 = 0; } while(0)
#define TAPA_IN_SetPullup()          do { WPUAbits.WPUA1 = 1; } while(0)
#define TAPA_IN_ResetPullup()        do { WPUAbits.WPUA1 = 0; } while(0)
#define TAPA_IN_SetPushPull()        do { ODCONAbits.ODCA1 = 0; } while(0)
#define TAPA_IN_SetOpenDrain()       do { ODCONAbits.ODCA1 = 1; } while(0)
#define TAPA_IN_SetAnalogMode()      do { ANSELAbits.ANSA1 = 1; } while(0)
#define TAPA_IN_SetDigitalMode()     do { ANSELAbits.ANSA1 = 0; } while(0)
// get/set IO_RA2 aliases
#define REARME_IN_TRIS                 TRISAbits.TRISA2
#define REARME_IN_LAT                  LATAbits.LATA2
#define REARME_IN_PORT                 PORTAbits.RA2
#define REARME_IN_WPU                  WPUAbits.WPUA2
#define REARME_IN_OD                   ODCONAbits.ODCA2
#define REARME_IN_ANS                  ANSELAbits.ANSA2
#define REARME_IN_SetHigh()            do { LATAbits.LATA2 = 1; } while(0)
#define REARME_IN_SetLow()             do { LATAbits.LATA2 = 0; } while(0)
#define REARME_IN_Toggle()             do { LATAbits.LATA2 = ~LATAbits.LATA2; } while(0)
#define REARME_IN_GetValue()           PORTAbits.RA2
#define REARME_IN_SetDigitalInput()    do { TRISAbits.TRISA2 = 1; } while(0)
#define REARME_IN_SetDigitalOutput()   do { TRISAbits.TRISA2 = 0; } while(0)
#define REARME_IN_SetPullup()          do { WPUAbits.WPUA2 = 1; } while(0)
#define REARME_IN_ResetPullup()        do { WPUAbits.WPUA2 = 0; } while(0)
#define REARME_IN_SetPushPull()        do { ODCONAbits.ODCA2 = 0; } while(0)
#define REARME_IN_SetOpenDrain()       do { ODCONAbits.ODCA2 = 1; } while(0)
#define REARME_IN_SetAnalogMode()      do { ANSELAbits.ANSA2 = 1; } while(0)
#define REARME_IN_SetDigitalMode()     do { ANSELAbits.ANSA2 = 0; } while(0)
// get/set IO_RB6 aliases
#define CORTE_OUT_TRIS                 TRISBbits.TRISB6
#define CORTE_OUT_LAT                  LATBbits.LATB6
#define CORTE_OUT_PORT                 PORTBbits.RB6
#define CORTE_OUT_WPU                  WPUBbits.WPUB6
#define CORTE_OUT_OD                   ODCONBbits.ODCB6
#define CORTE_OUT_ANS                  ANSELBbits.ANSB6
#define CORTE_OUT_SetHigh()            do { LATBbits.LATB6 = 1; } while(0)
#define CORTE_OUT_SetLow()             do { LATBbits.LATB6 = 0; } while(0)
#define CORTE_OUT_Toggle()             do { LATBbits.LATB6 = ~LATBbits.LATB6; } while(0)
#define CORTE_OUT_GetValue()           PORTBbits.RB6
#define CORTE_OUT_SetDigitalInput()    do { TRISBbits.TRISB6 = 1; } while(0)
#define CORTE_OUT_SetDigitalOutput()   do { TRISBbits.TRISB6 = 0; } while(0)
#define CORTE_OUT_SetPullup()          do { WPUBbits.WPUB6 = 1; } while(0)
#define CORTE_OUT_ResetPullup()        do { WPUBbits.WPUB6 = 0; } while(0)
#define CORTE_OUT_SetPushPull()        do { ODCONBbits.ODCB6 = 0; } while(0)
#define CORTE_OUT_SetOpenDrain()       do { ODCONBbits.ODCB6 = 1; } while(0)
#define CORTE_OUT_SetAnalogMode()      do { ANSELBbits.ANSB6 = 1; } while(0)
#define CORTE_OUT_SetDigitalMode()     do { ANSELBbits.ANSB6 = 0; } while(0)
// get/set IO_RB7 aliases
#define BLOQ_OUT_TRIS                 TRISBbits.TRISB7
#define BLOQ_OUT_LAT                  LATBbits.LATB7
#define BLOQ_OUT_PORT                 PORTBbits.RB7
#define BLOQ_OUT_WPU                  WPUBbits.WPUB7
#define BLOQ_OUT_OD                   ODCONBbits.ODCB7
#define BLOQ_OUT_ANS                  ANSELBbits.ANSB7
#define BLOQ_OUT_SetHigh()            do { LATBbits.LATB7 = 1; } while(0)
#define BLOQ_OUT_SetLow()             do { LATBbits.LATB7 = 0; } while(0)
#define BLOQ_OUT_Toggle()             do { LATBbits.LATB7 = ~LATBbits.LATB7; } while(0)
#define BLOQ_OUT_GetValue()           PORTBbits.RB7
#define BLOQ_OUT_SetDigitalInput()    do { TRISBbits.TRISB7 = 1; } while(0)
#define BLOQ_OUT_SetDigitalOutput()   do { TRISBbits.TRISB7 = 0; } while(0)
#define BLOQ_OUT_SetPullup()          do { WPUBbits.WPUB7 = 1; } while(0)
#define BLOQ_OUT_ResetPullup()        do { WPUBbits.WPUB7 = 0; } while(0)
#define BLOQ_OUT_SetPushPull()        do { ODCONBbits.ODCB7 = 0; } while(0)
#define BLOQ_OUT_SetOpenDrain()       do { ODCONBbits.ODCB7 = 1; } while(0)
#define BLOQ_OUT_SetAnalogMode()      do { ANSELBbits.ANSB7 = 1; } while(0)
#define BLOQ_OUT_SetDigitalMode()     do { ANSELBbits.ANSB7 = 0; } while(0)
// get/set IO_RC2 aliases
#define LED0_TRIS                 TRISCbits.TRISC2
#define LED0_LAT                  LATCbits.LATC2
#define LED0_PORT                 PORTCbits.RC2
#define LED0_WPU                  WPUCbits.WPUC2
#define LED0_OD                   ODCONCbits.ODCC2
#define LED0_ANS                  ANSELCbits.ANSC2
#define LED0_SetHigh()            do { LATCbits.LATC2 = 1; } while(0)
#define LED0_SetLow()             do { LATCbits.LATC2 = 0; } while(0)
#define LED0_Toggle()             do { LATCbits.LATC2 = ~LATCbits.LATC2; } while(0)
#define LED0_GetValue()           PORTCbits.RC2
#define LED0_SetDigitalInput()    do { TRISCbits.TRISC2 = 1; } while(0)
#define LED0_SetDigitalOutput()   do { TRISCbits.TRISC2 = 0; } while(0)
#define LED0_SetPullup()          do { WPUCbits.WPUC2 = 1; } while(0)
#define LED0_ResetPullup()        do { WPUCbits.WPUC2 = 0; } while(0)
#define LED0_SetPushPull()        do { ODCONCbits.ODCC2 = 0; } while(0)
#define LED0_SetOpenDrain()       do { ODCONCbits.ODCC2 = 1; } while(0)
#define LED0_SetAnalogMode()      do { ANSELCbits.ANSC2 = 1; } while(0)
#define LED0_SetDigitalMode()     do { ANSELCbits.ANSC2 = 0; } while(0)
// get/set IO_RC3 aliases
#define SW0_TRIS                 TRISCbits.TRISC3
#define SW0_LAT                  LATCbits.LATC3
#define SW0_PORT                 PORTCbits.RC3
#define SW0_WPU                  WPUCbits.WPUC3
#define SW0_OD                   ODCONCbits.ODCC3
#define SW0_ANS                  ANSELCbits.ANSC3
#define SW0_SetHigh()            do { LATCbits.LATC3 = 1; } while(0)
#define SW0_SetLow()             do { LATCbits.LATC3 = 0; } while(0)
#define SW0_Toggle()             do { LATCbits.LATC3 = ~LATCbits.LATC3; } while(0)
#define SW0_GetValue()           PORTCbits.RC3
#define SW0_SetDigitalInput()    do { TRISCbits.TRISC3 = 1; } while(0)
#define SW0_SetDigitalOutput()   do { TRISCbits.TRISC3 = 0; } while(0)
#define SW0_SetPullup()          do { WPUCbits.WPUC3 = 1; } while(0)
#define SW0_ResetPullup()        do { WPUCbits.WPUC3 = 0; } while(0)
#define SW0_SetPushPull()        do { ODCONCbits.ODCC3 = 0; } while(0)
#define SW0_SetOpenDrain()       do { ODCONCbits.ODCC3 = 1; } while(0)
#define SW0_SetAnalogMode()      do { ANSELCbits.ANSC3 = 1; } while(0)
#define SW0_SetDigitalMode()     do { ANSELCbits.ANSC3 = 0; } while(0)
/**
 * @ingroup  pinsdriver
 * @brief GPIO and peripheral I/O initialization
 * @param none
 * @return none
 */
void PIN_MANAGER_Initialize (void);

/**
 * @ingroup  pinsdriver
 * @brief Interrupt on Change Handling routine
 * @param none
 * @return none
 */
void PIN_MANAGER_IOC(void);


#endif // PINS_H
/**
 End of File
*/
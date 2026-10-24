# AGENTS.md — Contexto del Proyecto: SoloLoJusto (P2 Terminal Firmware)

## Rol y Objetivo
- Eres un agente de IA experto en C++, PlatformIO y sistemas embebidos.
- Estás programando el firmware del terminal (Rol P2) basado en un ESP32 para una cédula electrónica descentralizada en un hackathon de 36 horas.

## Arquitectura y Hardware
- **Placa:** ESP32 (32 bits, 240 MHz, mbedTLS integrado).
- **Lector NFC:** Adafruit_PN532 conectado por SPI (`SCK:18`, `MISO:19`, `MOSI:23`, `SS:5`).
- **Pantalla Ciudadano:** Nextion conectada por UART1 (`RX1:16`, `TX1:17`).
- **Pantalla Comercio:** Arduino UNO conectado por UART2 (`TX2:25`).
- **Guardián Físico:** PIC16F13145 (latido en `GPIO 26`, tamper en `GPIO 27`).
- **Gateway (PC de P3):** Comunicación bidireccional por USB Serial a 115200 baudios usando mensajes JSON en flujo (una línea JSON por mensaje).

## Contrato Técnico Clave
1. **Verificación Criptográfica:** El emisor firma digitalmente los atributos con **ECDSA P-256** sobre la curva secp256r1. El ESP32 debe calcular el SHA-256 del payload (`UID ‖ versión ‖ emisor ‖ atributo ‖ largo ‖ vence ‖ handle ‖ valor`) y verificarlo usando `micro-ecc` o `mbedTLS` en menos de 50 ms.
2. **Estructura de Comandos del Gateway (JSON):**
   - `{"type":"cfg", "rol":"bar", "atributo":"A1"}` -> Configuración de rol.
   - `{"type":"time", "timestamp":1710000000}` -> Sincronización de reloj.
   - `{"type":"rev", "handles":[...]}` -> Lista de revocación guardada en NVS (Preferences).
3. **Flujo del Terminal:**
   - Estado por defecto: `STATE_IDLE` esperando tarjeta NFC.
   - Lectura por roles según sectores de la tarjeta MIFARE 1K.
   - Envío de resultados y latidos (`hb`) al Gateway y al PIC por hardware.


   ## Plan de Acción Inmediato (Tareas actuales de P2)

El agente debe ayudar a implementar y probar las siguientes 4 tareas de desarrollo independiente en C++ para el ESP32:

### Tarea 1: Simular tarjetas NFC (Mocking)
- Crear una estructura de datos en C++ (`struct` o arreglo de bytes) que simule una "tarjeta virtual de prueba" basada en el contrato técnico:
  - Bytes de UID.
  - Sectores de atributos (Versión, Emisor, Atributo, Largo, Vence, Handle, Valor)[cite: 1].
  - Firma ECDSA simulada de 64 bytes.
- Permitir que la máquina de estados lea de esta tarjeta virtual cuando no haya un lector PN532 físico conectado.

### Tarea 2: Implementar y probar `micro-ecc` en el ESP32
- Asegurar que `micro-ecc` esté configurado en `platformio.ini`.
- Escribir una función de prueba que tome una clave pública P-256 de 64 bytes y un payload, ejecute `uECC_verify()`, y mida con `millis()` los milisegundos exactos que tarda en validar la firma (demostrando el rendimiento del ESP32 frente al Mega original).

### Tarea 3: Simular el flujo de pantallas Nextion (vía Serial)
- Escribir funciones para el control de la pantalla:
  - `mostrarEspera()`
  - `mostrarSolicitud(const char* nombreLocal, const char* atributo)`[cite: 1]
  - `mostrarResultado(bool aprobado)`[cite: 1]
- **Truco de desarrollo:** Mientras no esté conectada la pantalla física por `Serial1`, las funciones deben hacer un `Serial.println()` con el mismo texto para verificar el flujo visual directamente en el Monitor Serial de la PC.

### Tarea 4: Consolidar la Máquina de Estados Finitos (FSM)
- Implementar los 4 modos operativos del terminal dentro del `switch/case` del bucle principal:
  1. `MODE_VERIFY`: Verificación normal con consentimiento en pantalla[cite: 1].
  2. `MODE_EMERGENCY`: Lectura directa de salud sin consentimiento[cite: 1].
  3. `MODE_EMISSION`: Escritura de bloques firmados en tarjetas vírgenes[cite: 1].
  4. `MODE_LINKING`: Lectura de handle y despliegue del código de vinculación de 6 dígitos[cite: 1].
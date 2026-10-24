# Firmware ESP32 · Terminal SoloLoJusto (P2)

Firmware oficial del terminal de verificación descentralizado para **ESP32 DevKit V1** (ESP32-WROOM-32), desarrollado para el hackathon **SoloLoJusto**.

---

## 📌 Funcionalidades Principales

1. **Lectura NFC/RFID (MFRC522 por VSPI):**
   * Lectura de tarjetas físicas MIFARE Classic 1K por sectores según el rol activo.
   * Manejo de llaves de rol (Comercio, Salud, Registro, Tránsito).
   * Detección automática y recuperación tras corte de energía del Guardián (`nfcReinit`).
2. **Criptografía Asimétrica en Tiempo Real (<50 ms):**
   * Hash SHA-256 por hardware (`mbedTLS`).
   * Verificación de firmas **ECDSA P-256** (curva `secp256r1`, IEEE P1363 `r||s`) con `micro-ecc` contra la clave pública oficial de la Autoridad Nacional (`keys.h`).
   * Soporte de vectores oficiales de P1 (Ana válida, alterada, vencida).
3. **Interfaz Ciudadano (Nextion HMI por Serial2):**
   * Pantallas `page 0` a `page 8` (Espera, Solicitud, Verificando, Resultado, Menú de roles, etc.).
   * Detección de eventos táctiles (Aceptar/Rechazar) y menú oculto de configuración.
4. **Pantalla Comercio (Arduino UNO por Serial1 en GPIO 25):**
   * Protocolo JSON a 9600 baudios:
     * `{"t":"res","res":"01","a":"A1","emg":false}` tras cada lectura.
     * `{"t":"lock"}` con reenvío periódico cada 2.5 s en estado de bloqueo.
     * `{"t":"idle"}` en estado de espera ("sIsA / Esperando cédula...").
5. **Guardián Físico (PIC16F13145):**
   * Pulso de latido (*heartbeat*) cada 5 s en **GPIO 26**.
   * Detección de manipulación (*tamper*) en **GPIO 27** (`BLOQ_IN`).
6. **Almacenamiento Seguro NVS (`Preferences`):**
   * Lista negra de revocación en memoria no volátil (namespace `"slj"`).
7. **Gateway USB Serial (115200 baudios):**
   * Protocolo JSON estricto por línea (`hb`, `verif`, `tamper`, `uid`, `log`, etc.).

---

## 🔌 Conexiones de Hardware (Pinout)

| Periférico | Pin ESP32 | Función / Notas |
|---|---|---|
| **RC522 (VSPI)** | GPIO 18 | SCK |
| | GPIO 19 | MISO |
| | GPIO 23 | MOSI |
| | GPIO 5 | SDA / SS |
| | GPIO 4 | RST |
| | 3.3V & GND | **Solo 3.3V** (¡a 5V se daña!) |
| **Nextion HMI** | GPIO 16 (RX2) | RX de ESP32 $\leftarrow$ TX de Nextion |
| | GPIO 17 (TX2) | TX de ESP32 $\rightarrow$ RX de Nextion |
| | 5V & GND | Alimentación externa de 5V (GND compartido) |
| **Arduino UNO** | GPIO 25 (TX1) | TX de ESP32 $\rightarrow$ RX (Pin 0) de Arduino UNO |
| | GPIO 35 (RX1) | Dummy libre (input-only) |
| | GND | GND común |
| **PIC16F13145** | GPIO 26 | Pulso de latido hacia el PIC |
| | GPIO 27 | Entrada de bloqueo / tamper (`BLOQ_IN`) |

---

## 🚀 Compilación y Grabación

Con **PlatformIO CLI**:

```bash
# Compilar
pio run -d firmware/esp32

# Grabar en ESP32
pio run -d firmware/esp32 --target upload

# Abrir monitor serial
pio device monitor -d firmware/esp32
```

# P2 · Firmware del terminal (ESP32) — Hoja de ruta para codear

> **Para el asistente de código (Claude en VS Code):** este archivo es el contexto de trabajo de P2 en SoloLoJusto. Léelo completo antes de proponer código. Reglas:
> 1. Trabaja solo en `firmware/esp32/` y en los archivos de P2 dentro de `docs/`. No edites `app/`, `gateway/`, `firmware/pic/`, `firmware/uno/` ni `supabase/` sin que se pida: son de otras personas.
> 2. Placa: **ESP32 DevKit (ESP32-WROOM-32, 240 MHz, lógica de 3,3 V)**, framework Arduino en PlatformIO. Si la placa resulta ser ESP32-S3 o C3, **detente y avisa**: los pines cambian.
> 3. **Ningún pin del ESP32 recibe 5 V.** Todo lo que entra al ESP32 debe ser de 3,3 V.
> 4. **Nunca bloquees el `loop()` más de ~8 s**: el PIC guardián bloquea el terminal si no recibe el latido en 10 s (y el watchdog del ESP32 reinicia si una tarea se cuelga).
> 5. Wi-Fi y Bluetooth **apagados**: el terminal habla con la nube solo a través del gateway por USB. No cambiar esa arquitectura sin acuerdo del equipo.
> 6. El contrato técnico manda (`docs/contrato.md`). Si algo aquí choca, gana el contrato y se avisa.
> 7. Al terminar cada paso, di cómo validarlo (sección ✅).
> 8. Advertencia: **escribir mal un trailer de sector MIFARE lo bloquea para siempre.** Todo código que escriba trailers debe probarse primero en una tarjeta de sacrificio y tener una confirmación explícita.

Diagrama de conexiones: [`p2-circuito.svg`](p2-circuito.svg).

---

## 1. Contexto

SoloLoJusto es el prototipo de una **cédula electrónica que sabe más y muestra menos**. La "cédula" es una MIFARE Classic 1K; cada atributo (mayor de 18, identidad, sangre…) está firmado por el emisor con **ECDSA P-256** y guardado en sectores cuya **clave A depende del rol** del terminal. El terminal verifica la firma **sin internet**.

> En el roadmap general el terminal figura como "Arduino Mega". **Lo reemplaza un ESP32**: mismas funciones, mismo protocolo con el gateway; cambian pines, niveles (3,3 V), cripto (mucho más rápida) y almacenamiento (NVS en vez de EEPROM).

**Entregable de P2:** el ESP32 que lee la cédula, verifica la firma y decide, con **cuatro modos**: verificar, emergencia, emitir y vincular; más la HMI del ciudadano en la **Nextion**.

| Pieza | Dueño | Relación con P2 |
|---|---|---|
| Backend, firmas, claves, tarjetas de prueba | P1 | Te da claves públicas en arreglo C, vectores de prueba en hex y los bloques de emisión |
| Gateway Node.js, `mega-sim.js`, PIC, UNO, carcasa, fuente | P3 | Habla contigo por USB-serial; te manda `time`, `rev`, `cfg`, `write`, `code`; el PIC recibe tu latido |
| PWAs | P4 | La vinculación usa tu modo vincular |
| Portal, matriz de pruebas | P5 | Desde H24 te reporta fallas |

---

## 2. Conexiones (ver diagrama)

| ESP32 | Va a | Notas |
|---|---|---|
| GPIO19 MISO, GPIO23 MOSI, GPIO18 SCK, GPIO5 SDA(SS), GPIO4 RST | **RC522** (SPI) | SPI por hardware (VSPI). `MFRC522 rfid(5, 4);` IRQ sin conectar |
| 3V3 → (Q1 de P3) → 3.3V del RC522 | RC522 | **Solo 3,3 V.** Sin Q1 listo, directo al 3V3 |
| GPIO17 TX2 → RX · GPIO16 RX2 ← TX | Nextion | `Serial2`, 9600 (valor de fábrica de la Nextion) |
| GPIO25 TX1 → RX (A5) | UNO del comercio (P3) | `Serial1.begin(9600, SERIAL_8N1, 35, 25)` (RX en GPIO35, sin conectar). Solo envío |
| GPIO26 (salida) | PIC `HB_IN` | Latido: cambiar de estado cada 5 s *(propuesta)* |
| GPIO27 (entrada, `INPUT_PULLDOWN`) | PIC `BLOQ_OUT` | ALTO = terminal bloqueado *(propuesta)* |
| USB | Laptop (gateway) | `Serial`, 115200, una línea JSON por mensaje |
| GND | GND común | Todas las tierras unidas |

Niveles de voltaje (lo más importante del cambio a ESP32):
- **PIC a 3,3 V** (P3 lo configura en la Curiosity Nano). Así latido y BLOQ quedan a 3,3 V en ambos sentidos.
- **Nextion**: se alimenta a 5 V, pero su TX sale a 3,3 V en los modelos habituales → directo al GPIO16. **Medirlo** con el multímetro antes de conectar; si da 5 V, poner un divisor (p. ej. 1 kΩ / 2 kΩ).
- **RC522**: trabaja a 3,3 V igual que el ESP32 → compatible directo. **A 5 V se daña.** Se alimenta del pin 3V3 del ESP32 pasando por Q1 (el corte del guardián de P3).
- **UNO**: recibe 3,3 V del GPIO25; el UNO lo lee como ALTO (umbral ~3 V). Si falla, P3 añade un transistor o un adaptador de nivel.
- Pines a evitar: GPIO 6–11 (flash), 0, 2, 12, 15 (arranque), 34–39 solo entrada y sin pull-up.
- El ESP32 se alimenta **solo por USB**; su pin 5V/VIN **no** se une al riel externo.
- Lector: **RC522** (no se consiguió el PN532). Alcance de 2–3 cm: la antena va pegada a la pared de la carcasa y la tarjeta se apoya, no se "acerca". Cables SPI cortos (<15 cm). Tener un RC522 de repuesto si hay.

---

## 3. Entorno

- **VS Code + PlatformIO** (perfil "Arduino" si comparte máquina con MPLAB).
- **Driver USB-serial** del ESP32 en Windows: CP210x (Silicon Labs) o CH340 (WCH), según el chip de la placa.
- COM fijo en el Administrador de dispositivos: **ESP32 = COM10** (el que espera el gateway de P3).
- **Nextion Editor** (solo Windows) para las pantallas y el `.tft`; subirlo por microSD (más simple) o por un adaptador USB-TTL.
- **Auto-reset por DTR/RTS:** las placas ESP32 se reinician (o entran en modo de grabación) si el programa que abre el puerto activa DTR/RTS. Por eso:
  - En `platformio.ini`: `monitor_dtr = 0` y `monitor_rts = 0`.
  - P3 debe hacer `port.set({ dtr: false, rts: false })` al abrir el puerto en el gateway.
- **Un solo programa por puerto**: cerrar el monitor antes de que el gateway abra COM10, y viceversa. Para grabar, el gateway debe estar cerrado.
- Si la grabación no arranca ("Connecting…"), mantener pulsado **BOOT** hasta que empiece.

`firmware/esp32/platformio.ini` (verificar nombres de librería en el registro de PlatformIO):
```ini
[env:esp32dev]
platform = espressif32
board = esp32dev
framework = arduino
monitor_speed = 115200
monitor_dtr = 0
monitor_rts = 0
upload_port = COM10
monitor_port = COM10
build_flags =
  -D CORE_DEBUG_LEVEL=1
lib_deps =
  miguelbalboa/MFRC522
  kmackay/micro-ecc
  bblanchon/ArduinoJson
; SHA-256 sale de mbedTLS, que ya viene con el ESP32 (no hace falta librería)
```

Estructura sugerida:
```
firmware/esp32/
  platformio.ini
  include/
    config.h        # pines, baudios, tiempos
    keys.h          # claves públicas de emisores (de P1) y claves A/B por rol
    contract.h      # códigos de atributo, de resultado, mapa de sectores
  src/
    main.cpp        # máquina de estados y modos
    nfc.cpp/.h      # RC522: UID, autenticar, leer/escribir bloques, reinit tras corte
    verify.cpp/.h   # parseo del atributo, SHA-256 (mbedTLS), uECC_verify, vigencia, revocación
    link.cpp/.h     # serial con el gateway (JSON por línea)
    hmi.cpp/.h      # Nextion: cambiar página, poner textos, leer toques
    store.cpp/.h    # NVS (Preferences): lista de revocación
    guard.cpp/.h    # latido y entrada BLOQ_IN
test/
  vectors.h         # tarjetas de prueba de P1 (válida, alterada, vencida)
```

En `setup()`:
```cpp
WiFi.mode(WIFI_OFF);            // #include <WiFi.h>
btStop();
Serial.setRxBufferSize(1024);   // antes de begin
Serial.begin(115200);
Serial2.begin(9600, SERIAL_8N1, 16, 17);   // Nextion
Serial1.begin(9600, SERIAL_8N1, 35, 25);   // UNO (solo TX)
pinMode(26, OUTPUT);            // HB
pinMode(27, INPUT_PULLDOWN);    // BLOQ_IN
```

RC522 con la librería MFRC522 (lo esencial):
```cpp
#include <SPI.h>
#include <MFRC522.h>
MFRC522 rfid(5, 4);                  // SS, RST
SPI.begin(18, 19, 23, 5);            // SCK, MISO, MOSI, SS
rfid.PCD_Init();

// leer un bloque de un sector con la clave A del rol
MFRC522::MIFARE_Key key;             // key.keyByte[0..5] = clave del rol
rfid.PCD_Authenticate(MFRC522::PICC_CMD_MF_AUTH_KEY_A, bloque, &key, &rfid.uid);
byte buf[18]; byte len = sizeof(buf);  // MIFARE_Read exige 18 B (16 datos + 2 CRC)
rfid.MIFARE_Read(bloque, buf, &len);
// escribir (modo emitir, con clave B o de fábrica): rfid.MIFARE_Write(bloque, data16, 16);

// SIEMPRE al terminar con la tarjeta:
rfid.PICC_HaltA();
rfid.PCD_StopCrypto1();
```
- Autenticar **una vez por sector** (vale para sus 3 bloques); al pasar a otro sector, autenticar de nuevo.
- `rfid.uid.uidByte` / `rfid.uid.size` dan el UID que entra en la firma.
- Si el ESP32 lee `BLOQ_IN` en BAJO después de un bloqueo (el RC522 estuvo sin corriente), llamar `rfid.PCD_Init()` antes de volver a leer.

---

## 4. Contrato técnico que implementa P2

### 4.1 Formato de cada atributo

| Campo | Tamaño | Contenido |
|---|---|---|
| versión | 1 B | `0x01` |
| emisor | 1 B | `0x01` autoridad nacional, `0x02` universidad |
| atributo | 1 B | código (tabla 4.3) |
| largo | 1 B | bytes útiles de `valor` |
| vence | 4 B | fecha Unix *(propuesta: big-endian; confirmar con P1)* |
| handle | 8 B | identificador seudónimo de la cédula |
| valor | 16 B (64 B en identidad) | el dato |
| firma | 64 B | ECDSA P-256 **r‖s crudo** sobre `UID ‖ todos los campos anteriores` |

Tamaños: atributo normal = 32 B + 64 B firma = **96 B = 2 sectores × 3 bloques**. Identidad = 80 + 64 = **144 B = 3 sectores**.

**Verificación:**
```cpp
#include "mbedtls/sha256.h"
#include <uECC.h>
uint8_t hash[32];
mbedtls_sha256(msg, msgLen, hash, 0);                 // msg = UID ‖ campos (sin firma)
bool ok = uECC_verify(pubEmisor64, hash, 32, firma64, uECC_secp256r1());
```
En el ESP32 esto tarda del orden de decenas a pocos cientos de ms (medirlo en el paso 3). Alternativa sin librería externa: `mbedtls_ecdsa_verify` (más código, mismo resultado).

Acordar con P1 (crítico):
- Firma en formato **IEEE P1363 (r‖s, 64 B)**, no DER. En Node: `crypto.sign('sha256', msg, { key, dsaEncoding: 'ieee-p1363' })`.
- Clave pública **sin comprimir de 64 B** (sin el prefijo `0x04`), como arreglo C.
- UID de 4 B (MIFARE Classic). Si alguna tarjeta trae UID de 7 B, se usan los bytes que devuelve la librería, tal cual.
- Si `largo` < tamaño de `valor`, el relleno es `0x00` y entra en la firma.

### 4.2 Mapa de la MIFARE 1K

Sector *n* = bloques `4n`, `4n+1`, `4n+2` (datos) y `4n+3` (trailer). Datos útiles por sector: 48 B.

| Sectores | Bloques de datos | Atributo | Clave A (lectura) |
|---|---|---|---|
| 1 | 4, 5, 6 | Índice: versión + handle | Todos los terminales |
| 2–3 | 8–10, 12–14 | A1 · Mayor de 18 | Comercio |
| 4–5 | 16–18, 20–22 | A2 · Mayor de 21 | Comercio |
| 6–8 | 24–26, 28–30, 32–34 | A3 · Identidad | Registro |
| 9–10 | 36–38, 40–42 | A4 · Sangre y alergias | Salud |
| 11–12 | 44–46, 48–50 | A5 · Contacto de emergencia | Salud |
| 13–14 | 52–54, 56–58 | A6 · Donante y licencia | Salud y tránsito (comparten clave A) |
| 15 | — | Reserva | — |

- **Clave B** (solo emisor) permite escribir. Paramédico = rol emergencia, usa la clave de salud.
- Trailer: `KeyA(6) ‖ bits de acceso(3) ‖ GPB(1) ‖ KeyB(6)`.
- **Bits de acceso recomendados: `78 77 88`** (+ GPB `69`): datos legibles con A o B y escribibles solo con B; en el trailer, claves y bits solo cambiables con B; Key B no legible. Comprobar con una calculadora de bits de acceso MIFARE antes de escribir.
- Tarjeta virgen: clave A y B = `FF FF FF FF FF FF`, bits `FF 07 80`.
- En el prototipo, todas las claves de rol viven en `keys.h` (necesario para el menú de cambio de rol en la demo). En el pitch se explica que un terminal real solo tendría las de su rol.

### 4.3 Códigos

**Atributos:** A1 Mayor de 18 · A2 Mayor de 21 · A3 Identidad · A4 Sangre y alergias · A5 Contacto · A6 Donante y licencia *(valor numérico exacto: `docs/contrato.md`)*.

**Resultado:** `01` aprobado · `02` firma inválida · `03` revocada · `04` vencida · `05` sin permiso para ese dato · `06` rechazado por el ciudadano.

| Situación | Código |
|---|---|
| Autenticación con la clave del rol falla en un sector del atributo | `05` |
| Firma no verifica (incluye tarjeta alterada o atributo copiado a otro UID) | `02` |
| Handle está en la lista de revocación | `03` |
| `vence` < hora actual | `04` |
| El ciudadano toca Rechazar o se agota el tiempo de consentimiento | `06` |
| Todo bien | `01` |

Orden de comprobación: permiso (05) → firma (02) → revocación (03) → vigencia (04).

### 4.4 Serial ESP32 ↔ gateway (115200, JSON por línea `\n`)

Nombres de campos = *propuesta* compartida con P3; confirmar y copiar a `docs/contrato.md`.

**ESP32 → gateway**

| `t` | Campos | Cuándo |
|---|---|---|
| `hb` | — | Cada 5 s |
| `verif` | `h` (hex 16), `a`, `res` ("01"…"06"), `ts`, `emg` (bool) | Tras cada verificación |
| `tamper` | — | Una vez al pasar GPIO27 a ALTO |
| `uid` | `uid` (hex) | Modo emitir: tarjeta virgen leída |
| `ack` | `b` (bloque), `ok` (bool) | Tras escribir cada bloque recibido |
| `written` | `ok` (bool) | Emisión terminada |
| `link` | `h` | Modo vincular: handle leído |
| `log` | `m` (texto corto) | Depuración (el gateway la ignora) |

**Gateway → ESP32**

| `t` | Campos | Qué hace el ESP32 |
|---|---|---|
| `time` | `ts` | Ajusta su reloj |
| `rev` | `hs` (lista hex), `reset` (bool en el primer trozo) | Reemplaza/añade a la lista en NVS |
| `cfg` | `rol`, `a`, `local`, `modo` | Cambia rol, atributo pedido, nombre del local y modo |
| `write` | `b` (bloque), `d` (32 caracteres hex) | Autentica con clave de fábrica, escribe, relee, responde `ack` |
| `code` | `c` (6 dígitos) | Muestra el código 2 min |

- Lectura: acumular en un búfer propio (512 B) hasta `\n`, luego `deserializeJson`. Línea inválida → `log` y seguir.
- **Toda salida por `Serial` debe ser JSON** (el gateway lee esa línea). Nada de `Serial.println("debug")` suelto: usar `{"t":"log","m":"..."}`. Los mensajes de arranque del ESP32 (bootloader) son basura que el gateway ya descarta.
- Con el ESP32, `rev` puede ir en trozos más grandes que con el Mega, pero mantener líneas < 512 B.

### 4.5 Nextion (HMI del ciudadano)

| Página | Contenido | Toques |
|---|---|---|
| `p0` espera | Logo genérico, "Acerque su cédula cuando se le pida". Zona invisible en una esquina | Pulsación larga en la zona invisible → `p6` |
| `p1` solicitud | "**{local}** pide: **{atributo}**" | `Aceptar`, `Rechazar` |
| `p2` acerque | "Acerque su cédula" + animación | — |
| `p3` verificando | Animación breve | — |
| `p4` resultado | ✓ / ✗ grande + texto del código | — (vuelve a `p0` a los 3 s) |
| `p5` código | Código de 6 dígitos + cuenta regresiva 2:00 | — |
| `p6` menú oculto | Bar · Hotel · Hospital · Paramédico · Emitir · Vincular | Cada botón |
| `p7` bloqueado | "Terminal bloqueado" en rojo | — |
| `p8` emergencia | Tipo de sangre, alergias, contacto (para el paramédico) | — (vuelve a `p0` a los 15 s) |

- ESP32 → Nextion (`Serial2`): comandos de texto terminados en `0xFF 0xFF 0xFF`, p. ej. `page 1`, `t0.txt="Bar La Esquina"`.
- Nextion → ESP32: activar **"Send Component ID"** en los botones; llega `0x65 página componente evento 0xFF 0xFF 0xFF`. Parsear esos 7 bytes con un helper propio.
- Generar la fuente en Nextion Editor incluyendo `áéíóúñ¿¡`.

### 4.6 Lista de revocación en NVS

- `Preferences` (espacio `"slj"`): clave `"rev"` = blob de handles de 8 B; clave `"revn"` = cantidad. 4 KB alcanzan para ~500 handles (de sobra para la demo).
- Escribir solo cuando el contenido cambie.
- Mantener una copia en RAM para buscar rápido.

### 4.7 Reloj

- Sin RTC: `ahora = ts_recibido + (millis() - millis_al_recibir) / 1000`.
- Hasta recibir el primer `time`: mandar `log` y **no** aprobar atributos con vencimiento (responder `04`), para no aprobar una cédula vencida tras un reinicio. Confirmar esta regla con el equipo.

---

## 5. Paso a paso (con validación)

### H0–H4 · Pruebas de riesgo

- [ ] **0. La placa.** Confirmar modelo (WROOM-32 DevKit), driver USB instalado, COM10 fijo. Grabar un "Blink" o un `Serial.println` desde PlatformIO.
  ✅ Se graba y el monitor muestra la salida; abrir/cerrar el monitor no deja la placa colgada en modo de grabación.
- [ ] **1. RC522 lee el UID.** Cableado según el diagrama (3.3V directo al 3V3 si el Q1 de P3 aún no está). Ejemplo `DumpInfo` de MFRC522 con `MFRC522 rfid(5, 4)`.
  ✅ `PCD_ReadRegister(VersionReg)` devuelve `0x91` o `0x92` (otros valores, como `0x00`/`0xFF`, = cableado o alimentación mal; algunos clones dan `0x88`/`0x12` y suelen funcionar); apoyar una tarjeta imprime su UID siempre igual.
- [ ] **2. Autenticar y leer un bloque** con la clave de fábrica (bloque 4).
  ✅ Se imprimen 16 B; autenticar con una clave incorrecta falla limpio, sin colgar.
- [ ] **3. Firma en el ESP32 + tiempo.** Sketch de prueba con el vector de P1 (`test/vectors.h`): SHA-256 con mbedTLS, `uECC_verify`, medir con `micros()`. Probar también el vector alterado.
  ✅ Válida → 1; alterada → 0. Anotar el tiempo para la reunión de H4 (se espera muy por debajo de 1 s; el riesgo de "micro-ecc lento" del roadmap queda resuelto con el ESP32).
- [ ] **4. Hola mundo en la Nextion** por `Serial2`: el ESP32 cambia un texto y un botón se detecta. **Medir antes el voltaje del TX de la Nextion.**
  ✅ Texto cambia desde el ESP32; tocar el botón imprime el frame `0x65…` (vía `log`).

**Punto de control H4:** UID leído, bloque leído, firma verificada con tiempo medido, Nextion bidireccional.

### H4–H12 · Lectura y pantallas

- [ ] **5. Módulo de lectura por rol** (`nfc.cpp`, `verify.cpp`): dado el atributo y la clave A del rol, autenticar cada sector, leer los bloques, armar `UID ‖ campos` y separar la firma.
  ✅ Con una tarjeta escrita a mano con el vector de P1 (clave de fábrica por ahora), el mensaje reconstruido es idéntico al de P1 (comparar en hex).
- [ ] **6. Protocolo serial** (`link.cpp`): búfer de línea, `deserializeJson`, despacho por `t`, `send()` de cada tipo, `hb` cada 5 s.
  ✅ Contra el `mega-sim`/gateway de P3 o a mano en el monitor: `cfg` cambia el estado, líneas basura no cuelgan, `hb` sale puntual, nada que no sea JSON sale por `Serial`.
- [ ] **7. Pantallas Nextion** `p0`–`p5` y `p7` en Nextion Editor + `hmi.cpp`.
  ✅ Recorrer todas las páginas desde el ESP32; Aceptar/Rechazar llegan.
- [ ] **8. Recibir `cfg`**: rol, atributo pedido, nombre del local y modo; `p1` muestra "{local} pide: {atributo}".
  ✅ Cambiar `cfg` desde el gateway cambia el texto sin reiniciar.

**Punto de control H12:** tarjeta de prueba verificada en el ESP32.

### H12–H18 · Criptografía y emisión

- [ ] **9. Verificación completa** con la máquina de estados del modo verificar:
  `ESPERA → (cfg) SOLICITUD → Aceptar → ACERQUE (15 s) → LEYENDO → VERIFICANDO → RESULTADO (3 s) → ESPERA`. Rechazar o tiempo agotado en SOLICITUD → `06`. Al terminar: `verif` al gateway + resultado al UNO por `Serial1` (*propuesta*: `{"res":"01","a":1}`).
  ✅ Vectores: válida → 01; alterada → 02; handle revocado → 03; vencida → 04; rol sin clave → 05; rechazar → 06.
- [ ] **10. Revocación en NVS** (`store.cpp`) alimentada por `rev`.
  ✅ Revocar a María → `03`; sobrevive a un reinicio del ESP32.
- [ ] **11. Modo emitir.** Leer UID de tarjeta virgen → `uid`. Por cada `write`: autenticar con clave de fábrica, escribir, releer y comparar, `ack`. **Los trailers (claves por rol + `78 77 88`) se escriben al final, sector por sector**, solo tras confirmar todos los datos; luego `written`.
  ⚠️ **Primero en una tarjeta de sacrificio.** Acordar con P1 si los trailers los manda el backend o los arma el ESP32 con `keys.h` (propuesta: los arma el ESP32).
  ✅ Tras emitir: la tarjeta verifica `01` con el rol correcto y `05` con un rol ajeno; la clave de fábrica ya no abre los sectores.
- [ ] **12. Resultado al UNO** por `Serial1` (GPIO25).
  ✅ La pantalla del comercio de P3 muestra ✓/✗ y el atributo.

**Punto de control H18:** una cédula emitida de verdad y su uso visible en la web (con P1 y P3).

### H18–H24 · Integración

- [ ] **13. Primera emisión y verificación de punta a punta** con P1 y P3 (gateway real, API real).
- [ ] **14. Modo vincular**: leer sector 1 (índice) → `link` con el handle → mostrar `code` en `p5` durante 2 min.
  ✅ P4 escribe el código en la billetera y la vinculación se completa.
- [ ] **15. Guardián** (`guard.cpp`): cambiar GPIO26 cada 5 s sin bloquear el `loop()`; si GPIO27 pasa a ALTO → `p7`, enviar `tamper` una sola vez y dejar de leer hasta que vuelva a BAJO.
  ✅ Con P3: abrir la tapa → `p7` + `tamper` + voz; desconectar GPIO26 → el PIC bloquea en 10–11 s.

**Punto de control H24:** flujo completo cédula → terminal → voz → WhatsApp → historial.

### H24–H30 · Diferenciadores

- [ ] **16. Modo emergencia**: sin `p1`, leer A4 y A5 con la clave de salud, verificar, mostrar `p8`, `verif` con `emg:true`.
  ✅ El paramédico ve sangre y contacto en menos de 2 s tras acercar; llega el WhatsApp de emergencia.
- [ ] **17. Menú oculto `p6`** para cambiar de rol en la demo (Bar, Hotel, Hospital, Paramédico) y entrar a Emitir/Vincular.
  ✅ Cambiar de rol sin reconectar nada.
- [ ] **18. Matriz de pruebas de P5** y correcciones: Ana en bar, Luis en bar, María revocada, tarjeta alterada, bar intentando leer salud, hotel, paramédico, carcasa abierta.

### H30–H36 · Demo

- [ ] **19. Tarjetas finales**: 4 emitidas y etiquetadas (Ana, Luis, María, alterada) + 2 vírgenes de repuesto.
- [ ] Firmware final etiquetado en git (`git tag demo-esp32`) y `.tft` de la Nextion guardado en el repo.

---

## 6. Diagnóstico rápido

| Síntoma | Causa probable |
|---|---|
| `VersionReg` = `0x00` o `0xFF` | Cableado SPI, SS/RST cambiados, RC522 sin 3,3 V o cortado por el PIC |
| Lee la primera vez y luego nunca más la misma tarjeta | Falta `PICC_HaltA()` + `PCD_StopCrypto1()` al terminar; para releer sin retirarla usar `PICC_WakeupA` |
| Autenticación falla aunque la clave es correcta | Sesión anterior abierta (falta `PCD_StopCrypto1()`), o tarjeta movida durante la lectura |
| Lecturas intermitentes | Tarjeta lejos (>2 cm), cables largos, metal cerca de la antena |
| Tras abrir y cerrar la tapa el lector no responde | El RC522 perdió la configuración al cortarse; hacer `PCD_Init()` de nuevo |
| El ESP32 se reinicia o queda en "waiting for download" al abrir el puerto | DTR/RTS activos: `monitor_dtr/rts = 0`; en el gateway `port.set({dtr:false, rts:false})` |
| "Connecting…" eterno al grabar | Mantener BOOT; el gateway o el monitor tienen el puerto abierto |
| `Brownout detector was triggered` | Alimentación débil: cable USB corto/bueno, Nextion en el riel externo, Wi-Fi apagado |
| Reinicios con "Task watchdog" | Algún bucle bloqueante sin `delay()`/`yield()` |
| Autenticación falla en una tarjeta antes buena | Trailer mal escrito (sector perdido) o clave de rol equivocada |
| Firma siempre inválida con el vector de P1 | Firma en DER y no r‖s; clave con `0x04`; orden o endianness de campos distinto; UID no incluido |
| El gateway recibe basura | Prints de depuración que no son JSON, o mensajes del bootloader al arrancar |
| GPIO27 lee ALTO sin motivo | Falta `INPUT_PULLDOWN` o el PIC no comparte GND |
| Nextion no responde | TX/RX no cruzados, baudios distintos (9600), o la Nextion sin 5 V propios |

---

## 7. Pendientes de confirmar

- [ ] Con P1: firma r‖s, clave sin `0x04`, endianness de `vence`, relleno de `valor`, valores numéricos de atributos, quién arma los trailers, claves A/B por rol.
- [ ] Con P3: GPIO26/27 y nivel activo de BLOQ, **PIC a 3,3 V**, DTR/RTS en el gateway, nombres de campos JSON, ack de `write`, formato del mensaje al UNO, que el UNO lea bien 3,3 V.
- [ ] Con P5: actualizar el roadmap general (Mega → ESP32, carpeta `firmware/esp32/`) y quitar el riesgo "micro-ecc lento".
- [ ] Con el equipo: regla de vigencia sin hora recibida.

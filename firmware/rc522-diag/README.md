# Diagnóstico del RC522: ¿lector, cableado o tarjeta?

Síntoma: el LED del RC522 prende, pero al apoyar la tarjeta no llega nada.

**El LED solo indica que hay *algo* de voltaje en el pin 3.3V.** No prueba que el SPI funcione, ni que el voltaje alcance para generar el campo de radio, ni siquiera que el PIC lo tenga encendido (ver causa B).

## Primero: por qué el firmware principal puede estar en silencio aunque todo funcione

En modo verificar, `firmware/esp32` **solo sondea el lector en el estado ACERQUE**, y para llegar a ese estado hacen falta tres cosas:

1. La hora del gateway (`{"t":"time","ts":...}`). Sin ella la máquina de estados queda en `STATE_SIN_HORA` y no lee tarjetas.
2. Una solicitud aceptada: `{"t":"consent","approved":true}` del gateway, o escribir `a` en el monitor.
3. Apoyar la tarjeta **antes de 10 s** después de aceptar.

Si se apoya la tarjeta sin esos pasos, no sale nada, y es lo esperado. Para probar el lector con el firmware principal existe el comando **`scan`** en el monitor serie, que no necesita hora ni consentimiento.

## Causas probables, en orden

| | Causa | Cómo se ve | Cómo descartarla |
|---|---|---|---|
| A | El firmware no está esperando una tarjeta | Ningún log al apoyarla | `scan`, o este sketch |
| B | **El PIC le cortó la corriente al RC522** | LED del RC522 prendido (alimentado por SS y RST del ESP32), sin campo | LED de la Curiosity Nano **encendido = bloqueado**. Medir el pin 3.3V del RC522 |
| C | **Voltaje bajo a través de Q1** (seguidor de emisor) | Versión OK, pero ninguna tarjeta aparece | Medir el pin 3.3V del RC522: debe dar ≥ 2,5 V (ideal ≥ 3,0 V) |
| D | Cableado SPI | Versión `0x00`/`0xFF` o cambiante | Paso 1 de este sketch |
| E | La tarjeta no es de 13,56 MHz | Versión OK, ninguna tarjeta | Probar con la tarjeta blanca o el llavero azul que vienen con el RC522 |
| F | Módulo o antena defectuosos | Todo lo anterior bien y aun así nada | Cambiar el módulo |

### B: el guardián bloquea y nadie se entera

- Con el latch del PIC activo, `HAB_LECTOR` (RB6) queda en BAJO y Q1 corta el 3,3 V del RC522. **El LED de la Curiosity Nano encendido indica bloqueo, no "todo bien".**
- **Grabar el ESP32 activa el bloqueo:** a `upload_speed = 115200` la grabación tarda más de 10 s, el latido se detiene y el PIC bloquea. Después de cada grabación hay que **pulsar REARME** (RA2 a GND) o resetear la Curiosity Nano.
- Con el precinto abierto (RA5 sin su lazo a GND), el PIC bloquea siempre y REARME no hace nada. En la mesa de pruebas: un puente de RA5 a GND.
- Si alguien puenteó GPIO27 a GND "para probar sin el PIC", el ESP32 cree que está desbloqueado y mantiene SS y RST en ALTO. Por esos pines le llega al RC522 justo lo suficiente para prender el LED y hasta para contestar la versión, pero no para alimentar la antena.

### C: el seguidor de emisor se come el voltaje

Q1 (S8050) recibe su base del PIC. El emisor queda unos 0,7–0,9 V por debajo de la base:

- **Si el PIC está a 3,3 V** (como dice el diagrama), el RC522 recibe ≈ 2,2–2,6 V. El MFRC522 pide **2,5 V como mínimo**, y la potencia de la antena cae mucho con el voltaje. El LED prende igual.
- **Si el PIC está a 5 V** (como dice `firmware/pic/docs/pines-pic.md`), Q1 se satura y el RC522 recibe ≈ 3,1 V.

Los dos documentos no coinciden. **Hay que medir el voltaje con el lector conectado y leyendo, no suponerlo.**

## Procedimiento (unos 15 minutos)

### Paso 0: dejar fuera al PIC

1. Desconectar el pin 3.3V del RC522 del emisor de Q1 y **conectarlo directo al pin 3V3 del ESP32**. GND del RC522 al GND del ESP32.
2. Comprobar que la tira de pines del RC522 esté **soldada**. Los módulos vienen con los pines sueltos, y metidos a presión no hacen contacto.
3. Cables SPI cortos (< 15 cm): SDA→GPIO5, SCK→GPIO18, MOSI→GPIO23, MISO→GPIO19, RST→GPIO4. IRQ sin conectar.
4. Cerrar el gateway (no puede tener el puerto abierto).

### Paso 1: grabar este sketch

```bash
pio run -d firmware/rc522-diag -t upload
```

```bash
pio device monitor -d firmware/rc522-diag
```

O desde VS Code: abrir la carpeta `firmware/rc522-diag` con PlatformIO → Upload → Monitor.

### Paso 2: leer lo que imprime

| Mensaje | Significa | Qué hacer |
|---|---|---|
| `[1] ... SIN RESPUESTA` (0x00/0xFF) | El chip no contesta por SPI | Medir 3,3 V, revisar soldadura, revisar cada cable (MISO y MOSI sin cruzar) |
| `[1] ... CAMBIA entre lecturas` | Contacto intermitente | Cambiar los cables Dupont, acortarlos, revisar GND |
| `[2] ... FALLA` | MOSI no llega | Cable de GPIO23 |
| `[1]`, `[2]` y `[3]` OK, pero ninguna tarjeta | El chip funciona; falla el campo o la tarjeta | Ver paso 3 |
| `[4] TARJETA LEIDA` | **El hardware funciona** | Ver paso 4 |
| `[4] Algo respondio, pero el UID llego incompleto` | Campo débil | Acercar la tarjeta, probar `g`, medir el voltaje |
| `No es MIFARE Classic` | El lector funciona, pero la tarjeta no sirve como cédula | Usar MIFARE Classic 1K |
| `[!] El RC522 dejo de responder` | Se cortó la alimentación o un cable | Si ocurre ~10 s después de arrancar con Q1 en el circuito, es el PIC |
| `[!] El RC522 se reinicio solo` | La alimentación cae por momentos (típico al apoyar la tarjeta con voltaje justo) | Medir el 3,3 V; sacar a Q1 del camino |

### Paso 3: si el chip contesta pero no aparece ninguna tarjeta

1. Usar la **tarjeta blanca o el llavero azul del kit del RC522** (MIFARE Classic 1K, 13,56 MHz). Las tarjetas de acceso de 125 kHz (muchas tarjetas grises de edificio o de oficina) **el RC522 no las ve**.
2. Apoyar la tarjeta **plana sobre la antena** (la zona con la bobina impresa), a 0–2 cm, lejos de metal (mesa metálica, protoboard con riel, la carcasa).
3. Escribir `g` en el monitor para subir la ganancia a 48 dB y repetir.
4. Medir el pin 3.3V del RC522 contra GND mientras se apoya la tarjeta: debe dar 3,0–3,4 V.
5. Si con el pin directo al 3V3, la tarjeta del kit, ganancia máxima y 3,3 V medidos no aparece ninguna tarjeta, **el módulo está malo**: cambiarlo por el de repuesto.

### Paso 4: el hardware lee, ahora volver a poner a Q1 y al PIC

1. Volver a conectar el 3.3V del RC522 al emisor de Q1. GPIO26→`HB_IN` (RA4) y GPIO27→`BLOQ_OUT` (RB7).
2. Resetear el PIC o pulsar REARME. El sketch envía el latido por GPIO26 e informa el estado de BLOQ.
3. Debe aparecer `[PIC] BLOQ en BAJO` y el LED de la Curiosity Nano debe estar **apagado**.
4. **Medir el pin 3.3V del RC522 con una tarjeta apoyada.**
   - ≥ 3,0 V y lee: Q1 está bien.
   - Entre 2,5 y 3,0 V: funciona pero al límite; el alcance baja.
   - < 2,5 V o deja de leer: **Q1 como seguidor de emisor no alcanza.** Opciones, de la más rápida a la más correcta:
     1. Para la demo, RC522 directo al 3V3 y el guardián actuando solo sobre el ESP32: con BLOQ en ALTO el firmware ya deja de leer y suelta los pines del lector.
     2. Rearmar el corte por el lado de GND (Q1 como interruptor: emisor a GND, colector al GND del RC522), como prevé el paso 9 del roadmap de P3. El RC522 recibe así el 3,3 V completo. Ojo: con la GND cortada, los pines SPI siguen pudiendo alimentarlo; el ESP32 ya los suelta al bloquear.
     3. Un P-MOSFET de nivel lógico en el lado de 3,3 V (el diseño original).

### Paso 5: volver al firmware principal

```bash
pio run -d firmware/esp32 -t upload
```

Después de grabar: **REARME en el PIC**, abrir el gateway (o el monitor) y:

1. Escribir `scan` y apoyar la tarjeta. Debe imprimir UID, tipo y el mapa de sectores.
2. Para una verificación real: el gateway manda la hora (o pegar `{"t":"time","ts":<hora Unix>}` en el monitor), luego `a` y apoyar la cédula antes de 10 s.

Si `scan` lee pero el flujo normal no, el problema está en la hora o en el consentimiento, no en el lector.

## Tarjetas: qué esperar en `scan`

- **Tarjeta virgen** (todo `F`): el lector funciona. En modo verificar responde "no tiene índice de cédula" porque está vacía; para tener datos firmados hay que grabarla en **modo emitir**.
- **Cédula emitida** (sectores `R`): lista para verificar con el rol correcto.
- **Sectores `x`**: clave desconocida. Es una tarjeta de otro sistema, o un trailer mal grabado (ese sector queda perdido para siempre).

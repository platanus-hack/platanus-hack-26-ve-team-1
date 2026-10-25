# HMI del ciudadano (Nextion)

Las pantallas las dibuja el ESP32 (`firmware/esp32/src/hmi.cpp`) con comandos de la Nextion: `cls`, `fill`, `xstr`, `line` y `cirs`. Los botones también son dibujados: la Nextion informa cada toque como coordenadas (`sendxy=1`, eventos `0x67`) y el ESP32 decide qué botón se tocó.

A la Nextion solo hay que cargarle **una vez** un proyecto base con una página vacía y cuatro fuentes. Ese archivo `.tft` lo genera únicamente **Nextion Editor** (gratuito, solo para Windows); no hay otra forma oficial de hacerlo. Cambiar las pantallas no requiere volver a tocar el editor: se cambia `hmi.cpp` y se carga el firmware del ESP32.

## Proyecto base en Nextion Editor (una sola vez)

1. **Modelo:** está en la etiqueta trasera, por ejemplo `NX4832T035_011`. Crea el proyecto con ese modelo exacto y orientación horizontal; si no coincide, la pantalla rechaza el `.tft`.
2. **Codificación:** en la configuración del proyecto, `iso-8859-1`. El ESP32 convierte sus textos UTF-8 a esa codificación.
3. **Fuentes:** en *Tools → Font Generator*, en `iso-8859-1`, agrégalas al proyecto **en este orden**, porque el firmware las usa por número:

   | Número | Alto | Para qué |
   |-|-|-|
   | 0 | 16 px | Notas y aclaraciones |
   | 1 | 24 px | Cabecera y textos secundarios |
   | 2 | 32 px | Títulos y botones |
   | 3 | 56 px | El dato pedido, el código de vinculación, la sangre |

4. **Página `page0`:** vacía, sin componentes. En su evento *Preinitialize* escribe `sendxy=1`, así los toques funcionan aunque la pantalla se reinicie sola.
5. **Compilar y cargar:**
   - Compila y copia el `.tft` generado, solo, en la raíz de una microSD en FAT32.
   - Insértala con la Nextion apagada, enciéndela y espera a que termine la actualización.
   - Apágala, retira la tarjeta y vuelve a encenderla.
6. **Guardar en el repo:** el proyecto y el `.tft` van en `firmware/nextion/` (`sisa.HMI` y `sisa.tft`).

Para otro tamaño de pantalla, cambia `HMI_ANCHO` y `HMI_ALTO` en `firmware/esp32/include/config.h`. Las pantallas se diseñaron sobre 480 × 320 y se escalan a esa medida. En una de 320 × 240, usa fuentes más chicas (12, 16, 24 y 40 px) manteniendo el orden.

## Probar la pantalla

Desde el monitor serie o la consola del gateway, envía `{"t":"hmi_test"}`. Aparece una pantalla de prueba con las cuatro fuentes, tildes (`áéíóú ñ ¿¡`) y una muestra de colores.

- **Falta una fuente:** la Nextion responde `0x05` y el lector lo informa como `Nextion devolvió 0x05 (fuente inexistente...)`.
- **Tildes como signos raros:** el proyecto no quedó en `iso-8859-1`.

Conexión: TX de la Nextion → GPIO16 y RX → GPIO17, a 9600 baudios, alimentada desde la fuente externa de 5 V con GND común. Antes de conectar el TX, mide su voltaje: el ESP32 no tolera 5 V.

## Pantallas y flujo

| Modo | Reposo | Flujo |
|-|-|-|
| `verificar` | "Este terminal le pide: {dato}" con **Rechazar** y **Aceptar** | Aceptar → "Acerque su cédula" (10 s, o Cancelar) → Verificando → resultado 3 s → reposo. Rechazar → "No se compartió" (`06`) |
| `emergencia` | "Acerque la cédula del paciente" | Sin consentimiento → sangre, alergias (A4) y contacto (A5) durante 15 s |
| `vincular` | "Acerque su cédula" | Lee el índice → "Pidiendo el código..." → código de 6 dígitos durante 2 min |
| `emitir` | "Acerque una tarjeta virgen" | "Grabando la cédula..." → "Cédula emitida" o "No se pudo emitir" |
| Cualquiera | "Conectando con el sistema..." | Mientras no llegue la hora del gateway, no se verifican cédulas |
| Cualquiera | "Terminal bloqueado", en rojo | Mientras el PIC mantenga el bloqueo |

El reposo del modo verificar es la propia solicitud: nada se lee antes de que el ciudadano acepte. Como en ese momento todavía no hay cédula, un rechazo (`06`) se muestra en la Nextion y en la pantalla del comercio, pero no llega al backend, porque no hay handle al que atribuirlo.

## Menú oculto

En cualquier pantalla de reposo, una pulsación de **2 s en la esquina superior izquierda** abre el menú: Bar, Clínica, Ambulancia, Tránsito, Registro y Volver. Como el rol de cada terminal está atado a su token, el lector no cambia de rol por su cuenta: envía `{"t":"rol","rol":"salud"}` al gateway. El gateway activa el terminal con ese rol y le responde con su `cfg`. Si en 8 s no llega respuesta, el lector vuelve al reposo.

## Pendiente

- Armar el proyecto base y probarlo en la pantalla real. Las posiciones y los tamaños de letra se ajustaron sin verlos, así que lo normal es que haga falta una pasada de ajustes.
- Medir el TX de la Nextion antes de conectarla al ESP32.

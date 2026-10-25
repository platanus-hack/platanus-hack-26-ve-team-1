# 01 · Brief del proyecto

## Resumen

**SoloLoJusto** es el prototipo de una cédula de identidad electrónica emitida por el Estado que *sabe más y muestra menos*: el chip guarda más datos que la cédula actual (tipo de sangre, contacto de emergencia, licencia), pero cada terminal recibe únicamente el dato que necesita y que el ciudadano autoriza. Track: **Trust** · Platanus Hack 26 Caracas.

## Problema

- Validar identidad en Venezuela implica fotocopiar la cédula, comprobar a ojo un documento laminado sin chip y dejar datos completos en bases que nadie controla.
- El SAIME anunció una cédula con chip (sangre, salud, licencia, contacto de emergencia) que no se ha concretado.
- Hay desconfianza legítima sobre qué haría un chip estatal con esos datos y quién puede leerlos.

## Propuesta de valor

| Para… | Valor |
|---|---|
| Ciudadano | Comparte solo lo necesario ("mayor de 18", no su fecha de nacimiento), con aviso inmediato por WhatsApp de **cada** uso, aprobado o denegado, y bloqueo de la cédula con un toque. |
| Comercio | Respuesta binaria verificable sin internet, sin almacenar datos personales. |
| Salud / emergencias | Datos vitales disponibles bajo consentimiento, o sin él en un terminal de emergencia que siempre notifica. |
| Emisor (Estado) | Control de emisión, revocación y carnets de otras instituciones, sin ver el uso. |

## Solución en cinco piezas

1. **La cédula** (tarjeta NFC MIFARE 1K): atributos firmados con ECDSA P-256; cada grupo de sectores se abre con una clave distinta según el rol del terminal.
2. **El terminal** (ESP32 + lector RC522, pantalla Nextion, pantalla del comercio con UNO): verifica la firma sin internet, pide consentimiento y responde con voz y pantalla.
3. **El guardián** (PIC16F13145): bloquea el lector por hardware si abren la carcasa o el firmware deja de latir.
4. **La billetera y el verificador** (PWA): credenciales derivadas con QR dinámico firmado por el celular; verificación sin red.
5. **La nube** (API en Go + Postgres en Render + Kapso): emisión, vinculación, historial en vivo, avisos por WhatsApp y bloqueo respondiendo BLOQUEAR.

## Flujo estrella

Una persona acerca su cédula al terminal → el firmware lee el índice (handle) y **solo el atributo que el terminal pide** → verifica firma, vigencia, revocación y permiso por rol → el resultado va a la API → la API avisa por WhatsApp (Kapso) al ciudadano **tanto si se concedió como si se denegó el acceso**. Detalle en [04-flujos.md](04-flujos.md).

## Alcance

**Incluido (prototipo):** emisión simulada, terminal con 4 modos (verificar, emergencia, emitir, vincular), billetera y verificador PWA, portal ciudadano, paneles de emisor y comercio, avisos y webhook de Kapso, guardián PIC, carnet universitario, datos de demo.

**Fuera de alcance (hoja de ruta):** chip seguro real con autenticación activa, biometría y foto, historial cifrado de extremo a extremo, firmas no vinculables, estándares internacionales, terminales certificados.

## Retos de CédulaTouch cubiertos

1. Verificación de edad sin revelar datos (núcleo).
2. Check-in rápido en salud con acceso de emergencia.
3. Billetera de credenciales offline.

## Usuarios

Ciudadano (Ana, Luis, María en la demo), comercio (bar, hotel), personal de salud y paramédico, emisor/autoridad, universidad.

## Métricas de éxito

| Métrica | Objetivo |
|---|---|
| Verificación de punta a punta (tarjeta → resultado) | < 5 s |
| Aviso de WhatsApp tras la lectura | < 10 s con red |
| Datos personales visibles al comercio | 0 |
| Terminal y verificador funcionando sin red | Sí |
| Intentos fraudulentos denegados (copia, revocada, vencida, sin permiso) | 100 % |

## Equipo

Luis Arturo Becerra Dalponte, Isabella Noy, Manuela Panzarelli, Samuel David Sánchez Chacón y Matias Silveira (ver [README](../README.md)).

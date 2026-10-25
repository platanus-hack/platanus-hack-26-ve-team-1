# SoloLoJusto · Cédula electrónica — Roadmap del equipo (36 h)

Oct 24, 2026 · @Lartrax

## Resumen

Construimos el prototipo de una cédula de identidad electrónica emitida
por el Estado que **sabe más y muestra menos**: el chip guarda más datos
que la cédula actual (tipo de sangre, contacto de emergencia, licencia),
pero cada terminal solo recibe el dato que necesita y que el ciudadano
autoriza.

**El problema que resolvemos.** Validar identidad en Venezuela hoy
significa fotocopiar la cédula, verificar a ojo un documento laminado
sin chip y dejar datos completos en bases que nadie controla. El SAIME
anunció una cédula con chip que guardaría tipo de sangre, salud,
licencia y contacto de emergencia, pero el proyecto no se ha concretado.
Y existe desconfianza sobre qué haría un chip estatal con esos datos.

**Nuestra respuesta, en cinco piezas:**

1.  **La cédula** (tarjeta NFC): atributos firmados digitalmente por el
    emisor, cada grupo protegido con claves distintas según quién puede
    leerlo.

2.  **El terminal** (Arduino Mega + PN532): verifica la firma sin
    internet, pide consentimiento en la Nextion, responde con voz y
    muestra al comercio solo el resultado. Un PIC lo bloquea por
    hardware si lo abren.

3.  **La billetera** (PWA en cualquier celular): se vincula una vez a la
    cédula y presenta credenciales derivadas con un QR dinámico firmado.

4.  **El verificador** (PWA en el teléfono del comercio): lee ese QR y
    lo valida sin internet.

5.  **La nube**: portal ciudadano con historial, avisos y bloqueo por
    WhatsApp (Kapso), y paneles del emisor.

**Retos de CédulaTouch que cubrimos:** verificación de edad sin revelar
datos (reto 1, el núcleo), check-in rápido en salud con acceso de
emergencia (reto 2, escena de la demo) y billetera de credenciales
offline (reto 3).

En el prototipo, "la cédula" es una tarjeta MIFARE del kit y el emisor
es una autoridad simulada, sin logos oficiales. En el pitch se explica
que la versión real usaría un chip seguro como el de los pasaportes
electrónicos.

## Arquitectura

<img src="media/image1.png" style="width:6in;height:5.49107in"
alt="arquitectura · cédula, billetera y nube" />

arquitectura · cédula, billetera y nube

Hay dos caminos de verificación que no necesitan internet: la cédula en
el terminal y la billetera ante el verificador. La nube solo emite
credenciales, vincula la billetera, envía avisos y guarda el historial
del ciudadano.

## Infraestructura recomendada

Todo se despliega en Render: un Static Site con la app React aloja el
portal y las dos PWAs, y un Web Service en Go expone la API; una base
Postgres de Render guarda los datos, y la misma API en Go maneja
usuarios e historial en vivo. Un solo render.yaml despliega ambos
servicios desde el monorepo en cada push, hay un único login y nadie
pierde horas configurando servidores.

| Componente | Tecnología | Por qué |
|----|----|----|
| Portal, PWAs y API | Frontend: React + TypeScript + Vite + Tailwind, como Static Site en Render. Backend: Go (net/http con chi), como Web Service en Render | Un repositorio, despliegue automático en cada push, URL pública para el webhook de Kapso |
| Base de datos y usuarios | Postgres de Render (pgx y sqlc desde Go); login propio con correo y contraseña (bcrypt + JWT en cookie HttpOnly); historial en vivo con Server-Sent Events y LISTEN/NOTIFY | Todo queda en Render; el historial aparece en vivo en la web y cada consulta filtra por el usuario de la sesión, así nadie ve datos de otro |
| Firmas del emisor | ECDSA P-256 con el módulo crypto/ecdsa de Go; claves privadas como variables de entorno de Render | Mismo algoritmo en servidor, navegador (WebCrypto) y Mega (micro-ecc) |
| Billetera y verificador | PWA dentro de la app React (vite-plugin-pwa); WebCrypto con claves no extraíbles en IndexedDB; librería qrcode para generar y @zxing/browser para leer | Funciona en iPhone y Android sin tiendas ni cuenta de Apple; verifica sin internet |
| Terminal | Arduino Mega con PlatformIO o Arduino IDE; librerías Adafruit PN532, micro-ecc y Nextion | Lectura de MIFARE y verificación de firmas en 8 bits |
| Pantalla del comercio | Arduino UNO + shield ILI9488 (MCUFRIEND_kbv), recibe el resultado del Mega por serial | Separa físicamente lo que ve el comercio de lo que ve el ciudadano |
| Guardián | PIC16F13145 con MPLAB X, MCC y el sintetizador de CLB | Bloqueo por lógica de hardware, independiente del firmware del Mega |
| Gateway | Node.js 20 en la laptop: serialport, cola local en SQLite, reproducción de MP3 | Conecta el Mega con la nube y da voz al terminal aunque no haya internet |
| Voz | ElevenLabs (modelo multilingüe, voz en español), audios generados una vez | Respuesta inmediata y ningún dato personal sale hacia ElevenLabs |
| Avisos | Kapso: número sandbox de WhatsApp, API de envío y webhook de entrada | Aviso tras cada uso y bloqueo respondiendo BLOQUEAR |
| Secretos | Gestor de contraseñas compartido (Bitwarden o 1Password) + .env.local fuera de git | Nadie sube claves al repositorio |

**Rutas de la aplicación React (React Router) y de la API en Go:**

- / portal ciudadano: mi cédula, mis datos opcionales, historial,
  bloquear, WhatsApp.

- /emisor panel de la autoridad simulada: emitir cédula, revocar, emitir
  carnet universitario.

- /comercio configuración de terminales: rol, dato que piden, nombre del
  local.

- /billetera PWA del ciudadano.

- /verificar PWA del comercio.

- /api/... endpoints del contrato técnico, servidos por el Web Service
  en Go; el Static Site reescribe /api/\* hacia él, así el frontend usa
  las mismas rutas.

**Por qué no otras opciones:** Supabase o Firebase darían login y tiempo
real hechos, pero con Postgres de Render todo vive en una sola
plataforma y el backend en Go controla el acceso a los datos. Una app
nativa da más control de NFC, pero exige compilar por plataforma y
cuenta de Apple; con la cédula como portadora, el celular ya no necesita
NFC.

## La cédula y los datos (contrato técnico)

Este contrato se cierra entre H1 y H4 y vive en docs/contrato.md.
Cualquier cambio se anuncia en el canal del equipo antes de hacerlo.

### Formato de cada atributo en el chip

Cada atributo es una credencial independiente firmada por el emisor:

| Campo | Tamaño | Contenido |
|----|----|----|
| versión | 1 B | 0x01 |
| emisor | 1 B | 0x01 autoridad nacional, 0x02 universidad |
| atributo | 1 B | Código de la tabla siguiente |
| largo | 1 B | Bytes de valor |
| vence | 4 B | Fecha Unix |
| handle | 8 B | Identificador seudónimo de la cédula, aleatorio |
| valor | hasta 16 B (64 B en identidad) | El dato |
| firma | 64 B | ECDSA P-256 sobre UID de la tarjeta ‖ todos los campos anteriores |

Incluir el UID en la firma hace que copiar un atributo a otra tarjeta
invalide la firma.

### Mapa de la tarjeta MIFARE 1K y acceso por rol

Cada sector tiene 3 bloques de 16 B (48 B). La clave A de cada sector
decide qué rol de terminal puede leerlo; la clave B, que solo tiene el
emisor, permite escribir.

| Sectores | Atributo | Quién puede leer (clave A) |
|----|----|----|
| 1 | Índice: versión y handle | Todos los terminales |
| 2–3 | A1 · Mayor de 18 | Comercio |
| 4–5 | A2 · Mayor de 21 | Comercio |
| 6–8 | A3 · Identidad: nombre, número V, vigencia | Registro (hotel, banco, hospital) |
| 9–10 | A4 · Tipo de sangre y alergias | Salud |
| 11–12 | A5 · Contacto de emergencia | Salud |
| 13–14 | A6 · Donante y licencia de conducir | Salud y tránsito |
| 15 | Reserva | — |

Este es el mismo principio que usan los pasaportes electrónicos para
proteger la huella: un terminal de bar no puede leer datos de salud
aunque lo intente. Advertencia para P2: escribir mal los bits de acceso
de un sector lo bloquea para siempre. Se prueba primero en una tarjeta
de sacrificio y con una calculadora de bits de acceso.

### Flujo de verificación en el terminal

1.  La Nextion muestra lo que pide el terminal: "Bar La Esquina pide:
    mayor de 18". El ciudadano toca **Aceptar** y acerca la cédula.

2.  El Mega lee el UID y se autentica en los sectores del atributo con
    la clave de su rol.

3.  Verifica la firma del emisor, la vigencia (con la hora que envía el
    gateway) y que el handle no esté revocado.

4.  Responde con un código: 01 aprobado, 02 firma inválida, 03 revocada,
    04 vencida, 05 sin permiso para ese dato, 06 rechazado por el
    ciudadano.

5.  Voz, pantalla del comercio y evento al gateway.

El rol **emergencia** es la única excepción al consentimiento: lee tipo
de sangre y contacto sin que el ciudadano toque Aceptar, y el uso se
notifica igual por WhatsApp.

### Flujo de emisión (autoridad simulada)

1.  En /emisor, se elige un ciudadano y se pulsa "Emitir cédula".

2.  El terminal en modo emisión lee el UID de una tarjeta virgen y el
    gateway lo envía al backend.

3.  El backend firma cada atributo con ese UID y devuelve los bloques a
    escribir.

4.  El Mega escribe los sectores, configura claves y bits de acceso, y
    confirma.

### Flujo de vinculación con la billetera

1.  El ciudadano toca su cédula en el terminal en modo vinculación.

2.  El backend genera un código de 6 dígitos que la Nextion muestra
    durante 2 minutos.

3.  En /billetera, el ciudadano inicia sesión, escribe el código y la
    PWA envía su clave pública.

4.  El backend asocia cuenta, cédula y dispositivo, y emite las
    credenciales derivadas.

### Credencial de la billetera y QR dinámico

La credencial derivada tiene los mismos campos que la del chip, más la
**clave pública del celular** (33 B), y la firma del emisor cubre esa
clave. El QR contiene:

- SLJ1: + base64url de: credencial ‖ hora actual (4 B) ‖ firma del
  celular sobre hora ‖ atributo.

- Se regenera cada 30 segundos. El verificador acepta si la firma del
  emisor y la del celular son válidas, la hora está dentro de ±30 s y el
  handle no está en la lista de revocación guardada.

Una captura de pantalla del QR deja de servir en medio minuto.

### Serial entre el Mega y el gateway

Una línea JSON por mensaje a 115200 baudios. El Mega lee en flujo con
ArduinoJson porque su búfer serial es de 64 B.

- Mega → gateway: verif (handle, atributo, resultado, hora), tamper, hb
  (latido cada 5 s), uid (modo emisión), link (handle en modo
  vinculación), written (emisión terminada).

- Gateway → Mega: time, rev (lista de handles revocados), cfg (rol,
  atributo, nombre del local, modo), write (sector y bloques en
  hexadecimal), code (código de vinculación).

### API

| Ruta | Quién la llama | Qué hace |
|----|----|----|
| POST /api/emision/start | Gateway | Recibe UID, devuelve bloques firmados |
| POST /api/link/start | Gateway | Recibe handle, devuelve código de 6 dígitos |
| POST /api/link/confirm | Billetera | Código + clave pública; vincula y emite credenciales |
| GET /api/wallet | Billetera | Credenciales vigentes del ciudadano |
| POST /api/events | Gateway, verificador | Registra el uso y dispara el WhatsApp |
| GET /api/revocations | Gateway, verificador | Handles revocados |
| GET /api/issuers | Verificador, firmware | Claves públicas de los emisores |
| POST /api/revoke | Portal, emisor, webhook | Revoca una cédula y sus credenciales |
| POST /api/webhooks/kapso | Kapso | Respuesta BLOQUEAR |
| POST /api/auth/register | Portal, billetera | Crea la cuenta del ciudadano |
| POST /api/auth/login | Portal, billetera, paneles | Inicia sesión y deja el JWT en cookie |
| GET /api/events/stream | Portal | Historial en vivo (Server-Sent Events) |

Los terminales y el gateway se autentican con un token por terminal.

### Tablas en Postgres (Render)

- citizens: cuenta (correo y hash bcrypt de la contraseña), rol
  (ciudadano, emisor o comercio), nombre, teléfono de WhatsApp, avisos
  activados.

- cards: handle, UID, ciudadano, estado (activa o revocada), fecha de
  emisión.

- attributes: cédula, código, valor, vencimiento. Los datos opcionales
  que el ciudadano declara (sangre, alergias, contacto) se escriben en
  el chip al emitir.

- devices y wallet_credentials: clave pública del celular y credenciales
  derivadas.

- terminals: nombre del local, rol, atributo que pide, token.

- events: handle, terminal, atributo, resultado, hora. La API filtra:
  cada ciudadano ve solo los suyos; el comercio ve totales, nunca
  handles. Retención de 30 días.

- link_codes: código, handle, vencimiento.

### Mensajes de WhatsApp (Kapso)

- Uso: "Tu cédula compartió 'Mayor de 18' con Bar La Esquina hoy a las
  21:40. ¿No fuiste tú? Responde BLOQUEAR."

- Emergencia: "Un terminal de emergencia leyó tu tipo de sangre y
  contacto hoy a las 21:40."

- Bloqueo: "Tu cédula fue bloqueada. Puedes reactivarla desde el
  portal."

- Manipulación: "El terminal 01 de Bar La Esquina reportó apertura de la
  carcasa."

### Audios (ElevenLabs)

bienvenida, acerque_cedula, acceso_concedido, acceso_denegado,
cedula_revocada, cedula_vencida, sin_permiso, acceso_emergencia,
vinculacion_lista, terminal_bloqueado.

## Cómo trabajamos

Cinco personas en 36 horas solo funcionan con un contrato fijo, ramas
separadas y reuniones cortas a hora fija.

**Repositorio (monorepo en GitHub):**

- web/ React: portal, emisor, comercio, billetera y verificador.

- api/ Go: endpoints del contrato técnico, firmas e integración con
  Kapso. En la raíz, render.yaml define los dos servicios.

- firmware/mega/, firmware/uno/, firmware/pic/

- gateway/ Node.js

- db/ migraciones SQL (golang-migrate) y datos de prueba

- docs/contrato.md

- keys/ solo claves **públicas**; las privadas van en el gestor de
  contraseñas.

**Ramas:** cada persona trabaja en p1-backend, p2-firmware, etc. y hace
merge a main al menos en cada punto de control. Dentro de web/, P5 es
dueño del diseño general y P4 de /billetera y /verificar; nadie edita la
carpeta de otro sin avisar.

**Reuniones de 10 minutos de pie:** H4, H8, H12, H18, H24, H30. Cada uno
responde tres cosas: qué terminó, qué sigue, qué lo bloquea. P5 lleva la
hora y decide recortes de alcance.

**Simuladores para no esperar a nadie:**

- P3 escribe un mega-sim.js que imita al Mega por consola, para probar
  el gateway sin hardware.

- P1 publica tarjetas de prueba (bloques firmados en hexadecimal) para
  que P2 verifique firmas antes de que exista la API.

- P1 publica credenciales de billetera de prueba para que P4 genere y
  verifique QR desde H4.

**Datos de demo** (los crea P1): Ana, 24 años, cédula activa con todos
los datos; Luis, 16 años; María, cédula revocada; una tarjeta alterada a
mano.

## P1 · Backend y criptografía

P1 es el "Estado" del sistema: emite y firma las cédulas, guarda el
historial y conecta WhatsApp. Su entregable final es la API en Go
desplegada en Render con avisos llegando en vivo.

**H0–H4 · Base y pruebas de riesgo**

- [ ] Crear en Render la base Postgres y los dos servicios, y el
  proyecto en Kapso (número sandbox); compartir claves en el gestor.

- [ ] Escribir la migración SQL con las 7 tablas del contrato y sus
  índices.

- [ ] Generar dos pares de claves P-256 (autoridad nacional y
  universidad). Exportar las públicas como JSON para las PWAs y como
  arreglo C para el firmware.

- [ ] Escribir sign_attribute.go: arma los bytes del atributo, antepone
  el UID y firma. Validarlo contra un verificador independiente en Node.

- [ ] Enviar un mensaje de prueba por Kapso a los teléfonos del equipo.

- [ ] Publicar en docs/ tarjetas de prueba en hexadecimal (válida,
  alterada, vencida) y credenciales de billetera de prueba.

**H4–H12 · API principal**

- [ ] POST /api/emision/start: recibe UID, firma los atributos del
  ciudadano elegido y devuelve los bloques por sector.

- [ ] POST /api/link/start y /api/link/confirm: código de 6 dígitos con
  2 minutos de vida y vinculación de la clave pública del celular.

- [ ] GET /api/wallet: emite credenciales derivadas (mayor de 18,
  licencia) atadas a la clave del celular.

- [ ] GET /api/issuers y GET /api/revocations.

- [ ] Tokens por terminal y verificación en cada endpoint del gateway;
  registro e inicio de sesión (bcrypt, JWT en cookie HttpOnly) y
  middleware de sesión y rol en los endpoints del portal, la billetera y
  los paneles.

**H12–H18 · Eventos y avisos**

- [ ] POST /api/events: guarda el evento, envía el WhatsApp
  correspondiente (uso normal o emergencia) y lo publica con NOTIFY para
  GET /api/events/stream.

- [ ] POST /api/webhooks/kapso: si la respuesta es BLOQUEAR, revoca la
  cédula, responde la confirmación y la lista de revocación cambia.

- [ ] POST /api/revoke para el portal y el emisor.

- [ ] Cargar los datos de demo: Ana, Luis, María y la tarjeta alterada.

**H18–H24 · Integración**

- [ ] Acompañar a P2 y P3 en la primera emisión real de una tarjeta y la
  primera verificación de punta a punta.

- [ ] Alerta de manipulación al comercio por WhatsApp.

- [ ] Revisar que ningún endpoint devuelva datos de un ciudadano a otro
  (probar con dos cuentas).

**H24–H30 · Diferenciadores**

- [ ] Emisor universidad: carnet universitario como credencial de
  billetera.

- [ ] Tarea programada (Cron Job de Render) que borra eventos de más de
  30 días.

- [ ] Corregir los errores que reporte P5.

**H30–H36 · Demo**

- [ ] Restaurar los datos de demo a su estado inicial con un solo
  comando antes de cada ensayo.

- [ ] Apoyar la grabación del video de respaldo.

## P2 · Firmware del terminal

P2 construye el corazón de la demo: el Mega que lee la cédula, verifica
la firma sin internet y decide. Su entregable final es un terminal con
cuatro modos: verificar, emergencia, emitir y vincular.

**H0–H4 · Pruebas de riesgo**

- [ ] Conectar el PN532 al Mega por SPI (pines 50–53 y un pin de
  selección) y leer el UID de una tarjeta del kit.

- [ ] Autenticarse en un bloque con la clave de fábrica y leerlo.

- [ ] Compilar micro-ecc y verificar la firma de la tarjeta de prueba de
  P1. **Medir el tiempo**: si pasa de 5 s, avisar en la reunión de H4.

- [ ] Hola mundo en la Nextion por Serial1.

**H4–H12 · Lectura y pantallas**

- [ ] Módulo de lectura: autenticar los sectores del atributo con la
  clave del rol, leer y armar el mensaje UID ‖ campos.

- [ ] Protocolo serial con el gateway usando ArduinoJson en flujo,
  probado contra el mega-sim de P3.

- [ ] Pantallas Nextion: espera, solicitud con botón Aceptar,
  verificando, resultado, código de vinculación, bloqueado.

- [ ] Recibir cfg del gateway: rol, atributo pedido, nombre del local y
  modo.

**H12–H18 · Criptografía y emisión**

- [ ] Verificación completa: firma del emisor, vigencia con la hora del
  gateway, revocación y códigos de resultado.

- [ ] Lista de revocación en la EEPROM del Mega (4 KB, unos 500 handles
  de 8 B).

- [ ] Modo emisión: leer UID de tarjeta virgen, recibir bloques,
  escribir sectores y, al final, los bits de acceso y claves por rol.
  **Probar en una tarjeta de sacrificio primero.**

- [ ] Enviar el resultado al UNO por Serial2.

**H18–H24 · Integración**

- [ ] Primera emisión real y primera verificación de punta a punta con
  P1 y P3.

- [ ] Modo vinculación: leer el handle del índice, enviarlo y mostrar el
  código recibido.

- [ ] Latido cada 5 s hacia el PIC por un pin digital y entrada de
  "bloqueado" desde el PIC.

**H24–H30 · Diferenciadores**

- [ ] Modo emergencia: lee sectores de salud sin consentimiento y lo
  marca así en el evento.

- [ ] Menú oculto en la Nextion para cambiar de rol durante la demo
  (bar, hotel, hospital, paramédico).

- [ ] Probar los casos de la matriz de P5 y corregir.

**H30–H36 · Demo**

- [ ] Dejar 4 tarjetas emitidas y etiquetadas (Ana, Luis, María,
  alterada) más 2 vírgenes de repuesto.

## P3 · Hardware, guardián y voz

P3 convierte las placas sueltas en un terminal creíble: alimentado,
cableado, en una carcasa, con voz y con un guardián que nadie puede
saltarse por software. Su entregable final es el terminal físico armado.

**H0–H4 · Inventario y pruebas de riesgo**

- [ ] Inventario de componentes y voltajes; plan de alimentación con una
  fuente externa de 5 V para la Nextion y el PN532 (el USB de la laptop
  se queda corto).

- [ ] Confirmar programador del PIC (Curiosity Nano o PICkit) e instalar
  MPLAB X con MCC.

- [ ] Primer CLB: un latch que se activa con una entrada y enciende un
  LED hasta pulsar un botón de rearme.

- [ ] Generar los 10 audios con ElevenLabs en español y guardarlos en
  gateway/audio/.

**H4–H12 · Gateway y guardián**

- [ ] Gateway en Node.js: abrir el puerto serial, leer líneas JSON,
  reproducir el audio según el resultado.

- [ ] mega-sim.js para que el gateway y P1 prueben sin hardware.

- [ ] Conectar sensor de proximidad o IR (tapa abierta) al PIC.

- [ ] Salida del PIC que corta el PN532 con un transistor o mantiene su
  pin de reset en bajo.

**H12–H18 · Gateway conectado a la nube**

- [ ] Reenviar eventos a POST /api/events con cola en SQLite y
  reintentos si no hay red.

- [ ] Cada 60 s: descargar revocaciones y enviarlas al Mega; enviar la
  hora.

- [ ] Enviar cfg al Mega según la configuración del terminal en
  /comercio.

- [ ] Puente para emisión y vinculación: UID y handle hacia la API,
  bloques y código hacia el Mega.

**H18–H24 · Pantalla del comercio y carcasa**

- [ ] UNO + ILI9488: recibe el resultado por serial y muestra un visto
  bueno o una cruz grande, el atributo y "0 datos personales guardados".

- [ ] Carcasa (MDF, cartón grueso o acrílico): Nextion y lector hacia el
  ciudadano, pantalla del UNO hacia el comercio, sensor de tapa dentro.

**H24–H30 · Guardián completo**

- [ ] El PIC también vigila el latido del Mega: si falta más de 10 s,
  bloquea.

- [ ] Al bloquear, avisa al Mega por un pin; el Mega envía tamper, suena
  "terminal bloqueado" y P1 manda la alerta.

- [ ] Cableado ordenado y etiquetado.

**H30–H36 · Demo**

- [ ] Lista de verificación de hardware, cables y fuente de repuesto,
  celular con datos para compartir internet.

## P4 · Billetera y verificador (PWA)

P4 resuelve el reto 3: la cédula se vincula una vez al celular y desde
ahí el ciudadano presenta credenciales con un QR que el comercio
verifica sin internet. Su entregable final son dos PWAs instalables en
iPhone y Android.

**H0–H4 · Pruebas de riesgo**

- [ ] Crear /billetera y /verificar dentro de la app de P5, con
  manifiesto y service worker para instalarlas en la pantalla de inicio.

- [ ] Generar una clave P-256 no extraíble con WebCrypto y guardarla en
  IndexedDB.

- [ ] Mostrar un QR en un celular y leerlo con la cámara de otro, en
  iPhone **y** Android.

**H4–H12 · Las dos PWAs con datos de prueba**

- [ ] Billetera: inicio de sesión con /api/auth, lista de credenciales,
  QR que se regenera cada 30 s con barra de tiempo.

- [ ] Verificador: escanear, decodificar SLJ1:, verificar firma del
  emisor, firma del celular y hora (±30 s).

- [ ] Pantalla de resultado grande: "Mayor de 18: SÍ", nombre del
  emisor, y nada más.

- [ ] Claves públicas de los emisores incluidas en la PWA para verificar
  sin red.

**H12–H18 · Conexión real**

- [ ] Pantalla de vinculación: escribir el código de 6 dígitos y enviar
  la clave pública a /api/link/confirm.

- [ ] Descargar credenciales reales desde /api/wallet.

- [ ] Verificador: guardar la lista de revocación y actualizarla cuando
  haya red.

- [ ] Prueba en modo avión: el verificador debe seguir funcionando.

**H18–H24 · Integración**

- [ ] Vinculación de punta a punta con el terminal de P2.

- [ ] El verificador envía el evento a /api/events cuando hay red, para
  que aparezca en el historial y llegue el WhatsApp.

- [ ] Elegir qué credencial presentar: el ciudadano toca la que quiere
  mostrar.

**H24–H30 · Diferenciadores**

- [ ] Carnet universitario del emisor universidad en la billetera.

- [ ] Historial local en la billetera.

- [ ] Pruebas cruzadas en al menos dos iPhone y dos Android, y corregir
  lo que falle.

**H30–H36 · Demo**

- [ ] Dejar instaladas las PWAs en los celulares de la demo, con sesión
  iniciada y permisos de cámara dados.

## P5 · Portal web, pitch y calidad

P5 es la cara del proyecto: construye lo que el jurado ve en pantalla,
cuida el reloj del equipo y, desde H24, se convierte en responsable de
pruebas. Su entregable final es el portal desplegado y el pitch
ensayado.

**H0–H4 · Base**

- [ ] Crear la app React con Vite y Tailwind, desplegarla como Static
  Site en Render (reescritura de rutas a index.html) y conectar el login
  con /api/auth.

- [ ] Definir la estructura de carpetas y el diseño base (colores,
  tipografía, componentes) que también usará P4.

- [ ] Conseguir lo que falte de hardware o cuentas detectado en el
  inventario.

**H4–H12 · Portal ciudadano**

- [ ] Página "Mi cédula": qué datos tiene el chip y quién puede leer
  cada uno (comercio, registro, salud).

- [ ] Datos opcionales: tipo de sangre, alergias, contacto de
  emergencia, donante. Se guardan y se escriben en el chip en la próxima
  emisión.

- [ ] Historial en tiempo real con Server-Sent Events de la API: lugar,
  dato compartido, hora y resultado.

- [ ] Botón "Bloquear mi cédula" y configuración del número de WhatsApp.

**H12–H18 · Paneles**

- [ ] /emisor: lista de ciudadanos, emitir cédula (envía la orden al
  terminal), revocar.

- [ ] /comercio: crear terminales, elegir rol y dato que piden, ver
  totales de uso sin datos personales.

- [ ] Primer borrador de slides centrado en el problema.

**H18–H24 · Guion y pulido**

- [ ] Guion de la demo con tiempos y quién hace qué.

- [ ] Pulido visual del portal para que se lea bien proyectado.

- [ ] Matriz de pruebas: Ana en bar, Luis en bar, María revocada,
  tarjeta alterada, bar intentando leer salud, hotel, paramédico,
  billetera sin red, BLOQUEAR, carcasa abierta.

**H24–H30 · Calidad**

- [ ] Ejecutar la matriz completa cada 2 horas y reportar fallas al
  dueño de cada pieza.

- [ ] Decidir recortes de alcance si un hito se atrasa.

- [ ] Slides finales.

**H30–H36 · Pitch**

- [ ] Grabar el video de respaldo con P1.

- [ ] Dirigir al menos 5 ensayos con cronómetro.

- [ ] Preparar las respuestas a preguntas del jurado.

## Cronograma

El flujo completo (cédula → terminal → voz → WhatsApp → historial) debe
funcionar en H24; en H30 se congela el código. Si un hito se atrasa más
de 2 horas, P5 recorta alcance.

<img src="media/image2.png" style="width:6in;height:3.125in"
alt="bloques de trabajo · 36 horas" />

bloques de trabajo · 36 horas

La tabla siguiente detalla qué hace cada persona en cada bloque; el paso
a paso completo está en su sección.

| Bloque | P1 · Backend | P2 · Firmware | P3 · Hardware y voz | P4 · PWAs | P5 · Portal y pitch | Punto de control |
|----|----|----|----|----|----|----|
| H0–H4 | Cuentas, tablas, claves, firma de prueba, Kapso | PN532 lee UID, micro-ecc medido, Nextion | Inventario, CLB básico, audios | QR entre dos celulares, clave WebCrypto | App en Render con login | **H4:** contrato cerrado; riesgos probados |
| H4–H12 | Emisión, vinculación, billetera, revocaciones | Lectura por rol, serial, pantallas | Gateway, simulador, sensores al PIC | Billetera y verificador con datos de prueba | Portal: mi cédula, datos, historial | **H12:** tarjeta de prueba verificada en el Mega; QR verificado sin red |
| H12–H18 | Eventos, WhatsApp, BLOQUEAR, datos demo | Verificación completa, EEPROM, modo emisión | Gateway en la nube, cola, revocaciones | Vinculación y credenciales reales | Paneles emisor y comercio, slides | **H18:** una cédula emitida de verdad y su uso visible en la web |
| H18–H24 | Integración, alerta de manipulación | Vinculación, latido, integración | Pantalla del comercio, carcasa | Vinculación con el terminal, eventos | Guion, matriz de pruebas | **H24: flujo completo** |
| H24–H30 | Universidad, retención, correcciones | Emergencia, cambio de rol, correcciones | Guardián completo y alerta | Carnet, pruebas cruzadas | Ejecutar pruebas, recortes, slides | **H30:** código congelado |
| H30–H36 | Datos demo reiniciables, video | Tarjetas finales | Lista de hardware | PWAs instaladas | Video, ensayos, preguntas | **H36:** pitch |

## Turnos de sueño

Cada persona duerme un bloque de 4 horas, nunca más de dos a la vez, y
P1 y P2 no duermen juntos antes de H18 porque la verificación depende de
ambos.

| Bloque | Duermen | Quién sigue y en qué |
|----|----|----|
| H10–H14 | P4, P5 | P1 cierra la API; P2 lectura; P3 gateway |
| H14–H18 | P1, P3 | P2 cripto; P4 y P5 conexión real y paneles |
| H18–H22 | P2 | P1, P3, P4, P5 integran |
| H22–H30 | Siestas de 1 h, una persona a la vez | Diferenciadores y pruebas |

Después de H30 nadie duerme más de 1 hora.

## Alcance priorizado

Si el tiempo aprieta, se recorta de abajo hacia arriba; nunca se
sacrifica el flujo completo por un extra.

**Imprescindible (sin esto no hay demo)**

- Cédula emitida y firmada; terminal que verifica sin red y respeta el
  acceso por rol.

- Voz de resultado y pantalla del comercio.

- Aviso por WhatsApp y historial en el portal.

**Importante (lo que nos diferencia)**

- Billetera con QR dinámico y verificador sin red.

- BLOQUEAR por WhatsApp.

- PIC guardián.

- Escena de hospital y emergencia.

**Si sobra tiempo**

- Carnet universitario del segundo emisor.

- Panel del comercio con totales.

- Borrado automático de eventos a los 30 días.

**Fuera del hackathon (diapositiva de hoja de ruta)**

- Chip seguro real con autenticación activa.

- Biometría en la cédula y foto del titular.

- Historial cifrado de extremo a extremo.

- Firmas no vinculables y compatibilidad con estándares internacionales.

## Demo y speech

El speech se centra en el problema y en cómo lo resolvemos, no en a
quién venderlo. Dura unos 4 minutos; cada escena prueba una afirmación.

**Apertura · el problema (40 s).** Una fotocopia de cédula sobre la
mesa: "Para entrar a un edificio, registrarse en un hotel o abrir una
cuenta, entregamos esto: nombre, número, fecha de nacimiento, todo. La
cédula actual no tiene chip y se verifica a ojo. El Estado anunció una
cédula electrónica con chip que guardaría hasta tipo de sangre y
contacto de emergencia, y todavía no existe. Además, hay una pregunta
justa: ¿quién lee ese chip y qué hace con los datos?"

**La respuesta (10 s).** "Diseñamos cómo debería ser: una cédula que
sabe más y muestra menos."

**Escenas** (las marcadas con \* son las primeras que se cortan si el
tiempo no alcanza):

1.  **Bar (30 s).** Ana acepta en la Nextion y acerca su cédula: "Acceso
    concedido". El comercio solo ve el visto bueno; a Ana le llega el
    WhatsApp y el uso aparece en el portal.

2.  **Menor (10 s).** Luis: "Acceso denegado". El bar nunca supo su edad
    exacta ni su nombre.

3.  **El bar intenta leer datos de salud (15 s).** "Sin permiso": su
    terminal no tiene la clave de esos sectores.

4.  **Hospital (30 s).** Cambio de rol: con consentimiento, el
    formulario de ingreso se autocompleta en menos de 2 segundos.

5.  *Emergencia (20 s).*\* Terminal de paramédico: tipo de sangre y
    contacto sin consentimiento, y el aviso llega igual al ciudadano.

6.  *Billetera (30 s).*\* Ana muestra un QR desde su celular; el
    verificador, en modo avión, responde "Mayor de 18: SÍ".

7.  **Cédula alterada (10 s).** "Acceso denegado": la firma no coincide.

8.  **Robo (30 s).** "No fui yo": María responde BLOQUEAR y su cédula
    queda revocada en el siguiente intento.

9.  *Manipulación (15 s).*\* Abren la carcasa: el PIC bloquea el
    terminal y suena "terminal bloqueado".

**Cierre (20 s).** "Esto no es solo un prototipo de hackathon: es una
propuesta de estándar para que la cédula electrónica venezolana proteja
al ciudadano por diseño."

### Preguntas probables

- **"¿El Estado no podría rastrear a todos con esto?"** El terminal
  verifica sin consultar a nadie. El historial existe para el ciudadano,
  usa un identificador seudónimo y se borra a los 30 días; la siguiente
  versión lo cifra de extremo a extremo para que ni el servidor lo lea.

- **"¿Una MIFARE no se puede clonar?"** Sí, y lo decimos: es el material
  del prototipo. Una cédula real usaría un chip seguro con autenticación
  activa, como los pasaportes electrónicos. La billetera ya resuelve
  esto hoy con un desafío firmado cada 30 segundos.

- **"¿Qué pasa sin internet?"** El terminal y el verificador funcionan
  sin red; avisos e historial se sincronizan al volver.

- **"¿Y si le presto mi cédula a un menor?"** Igual que hoy; la hoja de
  ruta agrega foto del titular y biometría en la tarjeta.

- **"¿Por qué un emisor estatal y no privado?"** Porque la cédula es el
  documento que todos ya aceptan; nuestro aporte es cómo debe
  comportarse su chip.

## Riesgos y planes B

Al usar la cédula como portadora desaparece el riesgo mayor del plan
anterior (emulación NFC en iPhone); los que quedan tienen salida
conocida.

| Riesgo | Señal temprana | Plan B | Dueño |
|----|----|----|----|
| No conseguimos PN532 | Falta en el inventario de H1 | Usar el RC522 del kit: lee MIFARE igual; solo cambia la librería | P2 |
| Un sector queda bloqueado por bits de acceso mal escritos | Una tarjeta deja de autenticar | Probar siempre en tarjeta de sacrificio; tener 2 vírgenes de repuesto | P2 |
| micro-ecc tarda demasiado en el Mega | Más de 5 s en la prueba de H4 | Animación de "verificando"; en el peor caso verificar en el gateway y presentarlo como modo conectado | P2 |
| La cámara de la PWA falla en algún iPhone | Prueba de H4 | Abrir en Safari en vez de la PWA instalada; tener un Android como verificador de la demo | P4 |
| Kapso o WhatsApp no llega a tiempo | El aviso tarda en las pruebas | El historial en tiempo real del portal muestra lo mismo; video de respaldo | P1 |
| Cae el internet del evento | El gateway acumula cola | Compartir datos desde un celular; terminal, voz y verificador funcionan sin red | P3 |
| La alimentación reinicia el Mega | Reinicios al encender la Nextion | Fuente externa de 5 V con tierra común | P3 |
| Integración tardía | H12 o H18 se atrasan más de 2 h | Recortar "Si sobra tiempo" y luego "Importante" | P5 |

## Después del hackathon

La última diapositiva muestra qué haría falta para que esto sea un
estándar nacional y no solo un prototipo.

| Etapa | Qué agrega | Por qué importa |
|----|----|----|
| Chip real | Tarjeta de policarbonato con chip seguro y autenticación activa; claves del emisor en un módulo de seguridad de hardware | Elimina la clonación y protege la clave más valiosa del sistema |
| Contra el préstamo | Foto del titular en la pantalla del comercio y biometría verificada dentro de la tarjeta | La persona que presenta la cédula es su dueña |
| Privacidad del historial | Cifrado de extremo a extremo y presentaciones no vinculables | Ni el servidor ni los comercios pueden reconstruir dónde estuvo alguien |
| Terminales certificados | Registro público de terminales y sus roles, auditable por cualquiera | Se sabe qué terminal puede leer qué dato |
| Interoperabilidad | Formatos internacionales de credenciales digitales | La billetera funciona fuera del país |
| Piloto | Un hospital, una universidad y algunos comercios en una ciudad | Medir tiempos reales de verificación y adopción |

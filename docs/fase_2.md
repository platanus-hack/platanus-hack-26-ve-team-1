# SoloLoJusto · Fase 2: Evolución a Estándar Nacional

Tras validar el flujo principal y la viabilidad técnica durante el hackathon (Fase 1), la Fase 2 tiene como objetivo transformar el prototipo de **CédulaTouch** en una solución de identidad digital robusta, segura y lista para implementarse a nivel nacional.

Esta fase deja atrás los componentes de prueba (como las tarjetas MIFARE y la autoridad simulada) para adoptar estándares internacionales de criptografía e infraestructura de clave pública (PKI).

## 1. Evolución del Hardware (La Cédula Física)
El prototipo validó la lectura descentralizada, pero el hardware debe ser impenetrable.

*   **De MIFARE a Chip Seguro:** Migración a tarjetas de policarbonato con un módulo de seguridad de hardware (Secure Element) incorporado, similar al estándar de los pasaportes electrónicos (eMRTD).
*   **Autenticación Activa:** El chip ya no solo almacenará datos firmados pasivamente, sino que tendrá su propia clave privada inextraíble. El chip firmará un desafío criptográfico enviado por el terminal en cada lectura para garantizar que no es un clon.
*   **Control de Acceso Básico/Extendido (BAC/EAC):** Implementación de canales seguros entre el lector y el chip para evitar la interceptación de datos por aire.

## 2. Prevención de Préstamo y Suplantación
Actualmente, el sistema confía en que quien porta la tarjeta es el dueño. La Fase 2 cierra esta brecha de seguridad.

*   **Match Biométrico on-Card:** Integración de la huella dactilar del ciudadano directamente en un sector protegido del chip. Los terminales de alta seguridad (bancos, fronteras) enviarán la lectura de la huella al chip, y este responderá únicamente "Sí" o "No", sin revelar el mapa biométrico.
*   **Fotografía Descentralizada:** El atributo de "Identidad" incluirá un hash de la fotografía del titular. Al verificar, la pantalla del comercio mostrará la foto oficial para que el vendedor haga una verificación visual rápida (útil para el caso de compra de alcohol o entrada a locales).

## 3. Privacidad y Criptografía Avanzada
El historial del ciudadano debe ser privado por diseño, protegiéndolo incluso de los administradores del servidor.

*   **Historial Cifrado de Extremo a Extremo (E2EE):** Los eventos generados por el terminal se cifrarán con la clave pública del celular del ciudadano antes de enviarse a la nube. Ni Supabase, ni el Estado, ni el equipo de desarrollo podrán leer dónde estuvo la persona.
*   **Presentaciones No Vinculables:** Uso de firmas criptográficas avanzadas (como *Zero-Knowledge Proofs* o credenciales BBS+) que permiten al ciudadano demostrar que es mayor de edad sin revelar su número de cédula exacto, evitando que los comercios crucen bases de datos para rastrearlo.

## 4. Ecosistema de Terminales Certificados
El hardware de los comercios (los ESP32 y PIC) pasará de ser un desarrollo libre a un estándar auditable.

*   **Registro Público de Terminales:** Los comercios deberán registrar sus terminales en una blockchain o registro inmutable del Estado. Cualquiera podrá auditar qué comercio tiene permiso para leer qué atributo.
*   **Rotación de Claves:** Los terminales se conectarán periódicamente para rotar sus claves de lectura y descargar listas de revocación de forma segura.

## 5. Interoperabilidad Internacional (Billetera)
La billetera PWA se actualizará para hablar el idioma del resto del mundo.

*   **Estándar ISO 18013-5 (mDL):** Adaptación de las credenciales derivadas para que funcionen como Licencias de Conducir Móviles (Mobile Driving Licenses), permitiendo que la billetera sea leída por autoridades de tránsito en otros países.
*   **W3C Verifiable Credentials:** Transición del formato cerrado (SLJ1) a credenciales verificables estándar, permitiendo guardar la cédula en Apple Wallet o Google Wallet de forma nativa.

## 6. Programa Piloto Controlado
Antes de un despliegue nacional, el sistema se probará en entornos cerrados de alto impacto.

*   **Sector Salud:** Implementación de terminales de emergencia en un hospital público principal y dos clínicas privadas para validar la velocidad de acceso al tipo de sangre y contacto de emergencia.
*   **Sector Universitario:** Uso de la infraestructura para emitir carnets estudiantiles en una universidad, controlando el acceso a bibliotecas y comedores.
*   **Comercios Seleccionados:** Despliegue de terminales de verificación de edad en 50 locales nocturnos y licorerías de la ciudad capital (Caracas).
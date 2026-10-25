# 02 · Especificación de requisitos de software (SRS)

Basada en IEEE 830, adaptada al alcance del prototipo.

## 1. Introducción

### 1.1 Propósito
Definir los requisitos del sistema SoloLoJusto para desarrolladores, probadores y jurado.

### 1.2 Alcance
Sistema de cédula electrónica con terminal NFC, billetera/verificador PWA y nube (API, base de datos, WhatsApp). No incluye el chip seguro real ni integración con sistemas oficiales.

### 1.3 Definiciones

| Término | Significado |
|---|---|
| Handle | Identificador seudónimo aleatorio de 8 B de una cédula |
| Atributo | Dato firmado: A1 mayor de 18, A2 mayor de 21, A3 identidad, A4 sangre y alergias, A5 contacto de emergencia, A6 donante y licencia |
| Rol | Tipo de terminal: comercio, registro, salud, tránsito, emergencia |
| Emisor | Autoridad que firma: 0x01 nacional, 0x02 universidad |
| Gateway | Programa Node.js en la laptop que conecta el lector con la API |
| Kapso | Plataforma de WhatsApp Business usada para avisos y webhook |
| Códigos `res` | 01 aprobado · 02 firma inválida · 03 revocada · 04 vencida · 05 sin permiso · 06 rechazada por el ciudadano |

### 1.4 Referencias
[contrato.md](contrato.md), [contrato-terminal.md](contrato-terminal.md), [roadmap.md](roadmap.md).

## 2. Descripción general

- **Perspectiva:** sistema distribuido (firmware + gateway + PWA + API). Los caminos de verificación (cédula en terminal y billetera ante verificador) funcionan sin internet.
- **Actores:** ciudadano, comercio, personal de salud, paramédico, emisor, universidad, Kapso, guardián PIC (ver [03](03-casos-de-uso.md)).
- **Restricciones:** MIFARE Classic 1K (clonable; se declara como limitación del prototipo), micro-ecc en 8/32 bits, plan gratuito de Render (arranque en frío), 36 h de desarrollo.
- **Supuestos:** el ciudadano tiene WhatsApp; el gateway tiene internet intermitente; la hora llega del gateway.

## 3. Requisitos funcionales

### Emisión y vinculación
| ID | Requisito |
|---|---|
| RF-01 | El emisor podrá ordenar la emisión de una cédula a un ciudadano con perfil completo. |
| RF-02 | El backend firmará cada atributo con `UID ‖ cabecera ‖ valor` y devolverá los bloques por sector. |
| RF-03 | El terminal en modo emitir escribirá sectores, claves por rol y bits de acceso. |
| RF-04 | El terminal en modo vincular enviará el handle y mostrará un código de 6 dígitos válido 2 minutos. |
| RF-05 | La billetera canjeará el código con su clave pública y recibirá credenciales derivadas (A1, A2, A6, carnet). |

### Verificación en terminal
| ID | Requisito |
|---|---|
| RF-10 | El terminal mostrará lo que pide ("{local} pide: mayor de 18") y esperará consentimiento, salvo en modo emergencia. |
| RF-11 | Al acercar la tarjeta, el firmware leerá el UID, el índice (handle) y **solo** el atributo pedido, autenticando con la clave A del rol. |
| RF-12 | El firmware verificará la firma ECDSA del emisor sin internet, la vigencia (hora del gateway) y que el handle no esté revocado. |
| RF-13 | El firmware producirá un código `res` (01–06) y emitirá un evento `verif` con handle, atributo, resultado, hora y marca de emergencia. |
| RF-14 | El terminal responderá con voz y en la pantalla del comercio, que solo recibe códigos. |
| RF-15 | Un rol sin la clave de un sector obtendrá `05` y no podrá leer el dato. |
| RF-16 | El rol emergencia leerá sangre y contacto sin consentimiento y marcará el evento `emg = true`. |
| RF-17 | El rechazo del ciudadano en la pantalla producirá `06`. Se registra como evento y se avisa solo si ya hay una cédula identificada (en el flujo normal el rechazo ocurre antes de acercarla y no hay handle). |

### Registro y avisos
| ID | Requisito |
|---|---|
| RF-20 | `POST /api/events` guardará cada `verif` y `tamper`, autenticado por token de terminal. |
| RF-21 | **Toda lectura (`res` 01–06) avisará por WhatsApp (Kapso) al dueño de la cédula** si tiene teléfono y avisos activos, con el motivo del resultado. |
| RF-22 | Los avisos de acceso incluirán el botón "Bloquear cédula", salvo cuando la cédula ya esté revocada (`03`). |
| RF-23 | Si el envío con botones falla, se enviará el mismo texto con "Responde BLOQUEAR". |
| RF-24 | Un `tamper` avisará al dueño del terminal. |
| RF-25 | El gateway guardará los eventos en cola local y reintentará si la API falla. |

### Bloqueo y revocación
| ID | Requisito |
|---|---|
| RF-30 | El webhook de Kapso validará la firma HMAC y, ante BLOQUEAR (texto o botón), revocará las cédulas de ese teléfono y confirmará por WhatsApp. |
| RF-31 | El webhook será idempotente ante reintentos (`X-Idempotency-Key`). |
| RF-32 | El ciudadano y el emisor podrán revocar y el ciudadano reactivar desde el portal. |
| RF-33 | `GET /api/revocations` publicará los handles revocados; terminales y verificadores los sincronizarán. |

### Billetera y verificador
| ID | Requisito |
|---|---|
| RF-40 | La billetera generará una clave P-256 no extraíble y un QR `SLJ1:` que se regenera cada 20–30 s. |
| RF-41 | El verificador validará firma del emisor, firma del celular, ventana de ±30 s y revocación, sin red. |
| RF-42 | El verificador mostrará solo el resultado y el emisor, y enviará el evento cuando haya red. |

### Portal y paneles
| ID | Requisito |
|---|---|
| RF-50 | El ciudadano verá su cédula, datos opcionales (sangre, alergias, contacto, donante), WhatsApp y historial en vivo (SSE). |
| RF-51 | El emisor gestionará ciudadanos, emisión, revocación y carnets. |
| RF-52 | El comercio gestionará terminales (rol, atributo) y verá totales sin handles. |
| RF-53 | Cada consulta filtrará por la cuenta de la sesión; nadie verá datos de otro. |

### Guardián
| ID | Requisito |
|---|---|
| RF-60 | El firmware enviará un latido al PIC; sin latido 10 s o con la tapa abierta, el PIC cortará el lector. |
| RF-61 | Con el bloqueo activo, el firmware emitirá `tamper` y mostrará "terminal bloqueado". |

## 4. Requisitos no funcionales

| ID | Categoría | Requisito |
|---|---|---|
| RNF-01 | Disponibilidad offline | Terminal y verificador funcionarán sin internet; avisos e historial se sincronizan al volver. |
| RNF-02 | Rendimiento | Verificación completa < 5 s; respuesta de `POST /api/events` sin esperar a Kapso. |
| RNF-03 | Privacidad | El comercio y la pantalla del comercio nunca reciben datos personales; los eventos usan handle seudónimo. |
| RNF-04 | Retención | Los eventos se borran a los 30 días. |
| RNF-05 | Seguridad criptográfica | ECDSA P-256 / SHA-256, firma cruda `r‖s`; el UID entra en la firma para impedir copias entre tarjetas. |
| RNF-06 | Seguridad de acceso | JWT HS256 en cookie HttpOnly; tokens de terminal guardados como SHA-256; CORS con credenciales solo para orígenes configurados. |
| RNF-07 | Resiliencia | Un fallo de Kapso no debe afectar la respuesta al terminal ni perder el evento. |
| RNF-08 | Interoperabilidad | Mismo formato de bytes en Go, WebCrypto y micro-ecc; vectores de prueba compartidos. |
| RNF-09 | Usabilidad | PWA instalable en iPhone y Android; mensajes en español; hora de Venezuela (UTC-4). |
| RNF-10 | Mantenibilidad | Pruebas automatizadas de cripto, API y front; CI por rutas. |

## 5. Interfaces externas

| Interfaz | Detalle |
|---|---|
| Lector ↔ gateway | Serial USB 115200, una línea JSON ([contrato-terminal.md](contrato-terminal.md) §1) |
| Lector ↔ PIC | Señales HB y BLOQ |
| Lector → pantalla comercio | Serial 9600, solo códigos |
| Gateway ↔ API | HTTPS con `Bearer <token>` |
| Front ↔ API | HTTPS, cookie de sesión, SSE |
| API ↔ Kapso | `POST https://api.kapso.ai/meta/whatsapp/v24.0/{id}/messages` (clave `X-API-Key`); webhook entrante firmado con HMAC |

## 6. Trazabilidad

| Requisito | Implementación | Prueba |
|---|---|---|
| RF-02, RNF-05 | `api/internal/sign` | `sign_attribute_test.go`, `tools/verify.mjs` |
| RF-20, RF-21, RF-22, RF-23 | `api/internal/api/events.go`, `notify.go` | `notify_test.go`, `kapso_integration_test.go` |
| RF-30, RF-31 | `api/internal/kapso`, `notify.go` | `kapso_test.go`, `kapso_integration_test.go` |
| RF-32, RF-33 | `events.go` (`revoke`), `cards.go` | `integration_test.go` |
| RF-50, RF-53 | `history.go`, `citizen.go`, `profile.go` | `history_integration_test.go`, `front_integration_test.go` |
| RF-11–RF-17, RF-60 | `firmware/esp32`, `firmware/pic` | Checklist de hardware ([09](09-pruebas.md)) |
| RF-40, RF-41 | `web/src/lib/slj` | Pruebas de `web/tests/slj`, conformidad Go↔WebCrypto |

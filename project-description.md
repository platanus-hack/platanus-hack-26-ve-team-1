# SoloLoJusto · Cédula electrónica que sabe más y muestra menos

**Track Trust · Platanus Hack 26 Caracas · team-1**

## El problema

Validar identidad en Venezuela hoy significa fotocopiar la cédula, comprobar a ojo un documento laminado sin chip y dejar datos completos en bases que nadie controla. El SAIME anunció una cédula con chip (tipo de sangre, salud, licencia, contacto de emergencia) que no se ha concretado, y existe una pregunta justa: ¿quién lee ese chip y qué hace con los datos?

## Nuestra respuesta

Un prototipo de cédula electrónica que **guarda más datos pero muestra menos**. Cada atributo está firmado digitalmente por el emisor y protegido con claves distintas según el rol del terminal. Un bar solo puede comprobar "mayor de 18"; un hospital, con el consentimiento del ciudadano, puede leer tipo de sangre y contacto; un paramédico, en emergencia, puede leerlos sin consentimiento, y el ciudadano **siempre** recibe el aviso.

## Cinco piezas

1. **La cédula:** tarjeta NFC MIFARE 1K con atributos firmados (ECDSA P-256) y sectores protegidos por rol.
2. **El terminal:** ESP32 + lector RC522, pantalla del comercio (UNO) que solo recibe códigos y voz en español. Verifica la firma **sin internet**.
3. **El guardián:** un PIC bloquea el lector por hardware si abren la carcasa o el firmware deja de latir.
4. **La billetera y el verificador:** PWA instalables; QR dinámico firmado por el celular, verificable sin red.
5. **La nube:** API en Go y Postgres en Render, historial en vivo, paneles del emisor y el comercio, y avisos por WhatsApp con Kapso.

## Flujo principal

1. El terminal muestra qué dato pide y el ciudadano acepta.
2. Acerca la cédula: el firmware lee el índice (handle) y **solo el atributo pedido**.
3. Verifica firma del emisor, vigencia, revocación y permiso del rol; resultado 01 aprobado, 02 firma inválida, 03 revocada, 04 vencida, 05 sin permiso, 06 rechazada.
4. Responde con voz y pantallas, y envía el evento a la nube.
5. La API avisa por **WhatsApp (Kapso) tanto si el acceso se autorizó como si se denegó**, con el motivo y un botón **Bloquear cédula**. Responder BLOQUEAR revoca la cédula al instante.

## Privacidad por diseño

- El comercio nunca recibe datos personales, solo el resultado.
- Los eventos usan un handle seudónimo; el historial es visible solo para su dueño y se borra a los 30 días.
- Las claves privadas del emisor nunca están en el repositorio.
- Limitaciones declaradas: MIFARE es clonable (una cédula real usaría un chip seguro con autenticación activa); la billetera ya mitiga con un desafío firmado cada pocos segundos.

## Stack

| Capa | Tecnología |
|---|---|
| Firmware | ESP32, PlatformIO, MFRC522, micro-ecc, ArduinoJson |
| Guardián | PIC16F13145, MPLAB X, CLB |
| Gateway | Node.js (serial, cola SQLite, audio) |
| API | Go, chi, pgx, JWT, bcrypt, Server-Sent Events |
| Base de datos | Postgres con migraciones SQL |
| Front y PWAs | React, TypeScript, Vite, Tailwind, WebCrypto, IndexedDB |
| Avisos | Kapso (WhatsApp) con webhook firmado |
| Despliegue | Render (`render.yaml`) |

## Demo

Ana (adulta) entra al bar y recibe su WhatsApp; Luis (menor) es denegado y también se entera; el bar que intenta leer salud recibe "sin permiso"; una tarjeta alterada falla por firma; María responde BLOQUEAR y su cédula queda revocada; el paramédico lee datos vitales y Ana es notificada; el verificador valida el QR en modo avión; abrir la carcasa bloquea el terminal.

## Cómo correrlo

```bash
cp .env.example .env.local        # completar DATABASE_URL y claves
go -C api test ./...              # pruebas
go -C api run ./cmd/admin demo -historial
go -C api run ./cmd/api           # API en :8080
npm --prefix web ci && npm --prefix web run dev
```

## Documentación

Índice completo en [`docs/README.md`](docs/README.md): [brief](docs/01-brief.md), [SRS](docs/02-srs.md), [casos de uso](docs/03-casos-de-uso.md), [flujos](docs/04-flujos.md), [arquitectura](docs/05-arquitectura.md), [modelo de datos](docs/06-modelo-datos.md), [seguridad y privacidad](docs/07-seguridad-privacidad.md), [despliegue y operación](docs/08-despliegue-operacion.md), [pruebas](docs/09-pruebas.md), y los contratos técnicos [`contrato.md`](docs/contrato.md) y [`contrato-terminal.md`](docs/contrato-terminal.md).

## Equipo

Luis Arturo Becerra Dalponte, Isabella Noy, Manuela Panzarelli, Samuel David Sánchez Chacón, Matias Silveira.

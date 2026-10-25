# P4 · Integración con la app web de P5

La billetera (`/billetera`) y el verificador (`/verificar`) reemplazan los placeholders de `web/src/pages/WalletPage.tsx` y `VerifyPage.tsx`. Usan el cliente de la API (`api/client.ts`), la sesión (`useAuth`), los componentes `ui/` y el tema de P5. El nombre en pantalla es **sIsA**.

## Dónde está cada cosa

| Pieza | Ruta |
| :---- | :---- |
| Núcleo criptográfico (sin dependencias) | `web/src/lib/slj/` |
| API de la billetera y del verificador | `web/src/api/wallet.ts` (+ `walletMock.ts`, solo con `VITE_USE_MOCK=true`) |
| Estado | `web/src/hooks/useWallet.ts`, `useVerifier.ts`, `useRotatingQr.ts`, `useOnline.ts` |
| Pantallas | `web/src/components/wallet/`, `web/src/components/verify/` |
| Instalable y sin red | `web/src/lib/pwa.ts`, `web/public/sw-p4.js`, `*.webmanifest`, íconos |
| Pruebas | `web/tests/slj/` (`node --test "web/tests/slj/**/*.test.ts"`) |

## Endpoints (contrato de P1)

| Quién | Ruta | Uso |
| :---- | :---- | :---- |
| Billetera | `POST /api/link/confirm` `{code, public_key}` | Vincula este celular; entrega los blobs |
| Billetera | `GET /api/wallet?public_key=` | Renueva las credenciales; vacío = cédula bloqueada |
| Verificador | `GET /api/issuers` | Claves de los emisores (JWK); se guardan para verificar sin red |
| Verificador | `GET /api/revocations` | `{hs:[…]}`; se guarda como blob binario |
| Verificador | `POST /api/events` | `{t:"verif",h,a,res,ts,emg}` con `Authorization: Bearer <token del terminal>` |

Los endpoints de P5 (`/api/me`, `/api/auth/*`, etc.) no se tocan.

## Decisiones y cambios en archivos de P5

- `api/client.ts`: `request` acepta `headers` (para el token del terminal). Cambio aditivo.
- `App.tsx` y `lib/roles.ts`: `/verificar` pasa al rol **comercio** y se añade el enlace "Verificar" a su navegación. **A confirmar con P5.**
- `package.json`: dependencias `uqr` (dibuja el QR, sin dependencias) y `jsqr` (lector de QR de respaldo para iPhone; se descarga solo si el navegador no trae `BarcodeDetector`).
- No se usó `vite-plugin-pwa` ni se tocó `vite.config.ts`: el service worker es un archivo estático propio.

## Privacidad (B.6 de P5)

- La clave del celular es WebCrypto **no extraíble**, en IndexedDB. Nunca en `localStorage`.
- El resultado del verificador muestra solo "Mayor de 18: SÍ" y el emisor. Nunca nombre ni handle, tampoco al rechazar.
- No hay `console.log`. La cadena `SLJ1:` no se pinta como texto (solo el QR).
- El token del terminal se guarda en IndexedDB del dispositivo del comercio (hace falta para enviar eventos al volver la red). Es opcional y se puede quitar.

## Sin red

Tras la primera carga con red, `/verificar` y `/billetera` abren y funcionan en modo avión: el service worker guarda los archivos de la app, y los emisores, las revocaciones y la cola de eventos están en IndexedDB. **El service worker nunca intercepta `/api/*`** (probado en `web/tests/slj/sw.test.ts`). Los eventos que no se pueden enviar se reintentan al volver la red; el verificador solo los encola si hay token.

La lista de revocación solo se actualiza con red: una cédula bloqueada mientras el verificador está en modo avión se acepta hasta que éste se reconecte.

## Probar sin backend

```bash
cd web && VITE_USE_MOCK=true npm run dev
```

Cuentas demo de P5 (`ana@sisa.demo` ciudadano, `comercio@sisa.demo` comercio, contraseña `demo1234`). Con el mock, cualquier código de 6 dígitos vincula y el "emisor" vive en memoria de la pestaña: la billetera y el verificador deben usarse sin recargar la página entre una y otra. En desarrollo, `/verificar` tiene un cuadro para pegar una cadena `SLJ1:` (no existe en producción).

## Pendiente de confirmar

- **P1:** bytes de "hora ‖ atributo" del QR (hora de 4 B big-endian + atributo de 1 B) y que `POST /api/events` acepte el token del terminal en `Authorization: Bearer` desde el verificador.
- **P5:** que `/verificar` sea del rol comercio, el enlace en la navegación y el manifiesto único "sIsA".
- **Render:** el Static Site necesita el rewrite `/* → /index.html` ya acordado; el service worker (`/sw-p4.js`) debe servirse con `Cache-Control: no-cache` para que las actualizaciones lleguen.
- **Celulares reales:** cámara, instalación y modo avión en iPhone y Android (no se pudo probar el service worker en un navegador real desde esta sesión).

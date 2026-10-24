# P4 · Pruebas

## Cómo correrlas

```bash
node --test "web/tests/slj/**/*.test.ts"          # 96 pruebas del núcleo (Node 26, sin dependencias)
cd tools/slj && go vet ./... && go test ./...   # CLI Go: códec, vectores, CLI como subproceso
cd tools/slj && go test -run xxx -fuzz=FuzzVerifyPresentation -fuzztime=30s
cd tools/slj && go run . vectors -out ../../web/tests/slj/testdata/vectors.json   # regenera vectores y corre ambos lados de nuevo
```

CI: `.github/workflows/p4.yml` (solo se dispara si cambian rutas de P4). El job `conformance` regenera los vectores y la presentación de WebCrypto y las verifica en ambos sentidos.

## Mapa de cobertura

Salió del grafo de graphify (comunidades por módulo); cada comunidad de código tiene su archivo de pruebas.

| Comunidad del grafo | Archivos | Pruebas |
| :---- | :---- | :---- |
| Utilidades binarias | `bytes.ts` | `bytes.test.ts`: base64url por la ruta nativa y por la de respaldo, hex, `subarray` con `byteOffset ≠ 0` |
| Códec | `codec.ts` | `codec.test.ts`: blob de billetera (el valor va antes de la clave del celular), largos 0/1/64/255, ±1 byte, versión, presentación, revocaciones (blob y JSON de P1) |
| P-256 | `p256.ts` | `p256.test.ts`: compresión con ambas paridades, puntos fuera de la curva, llave no extraíble |
| Verificación | `verify.ts`, `present.ts` | `verify.test.ts`: todos los códigos, bordes de hora y vencimiento, precedencias, anti-reuso |
| Contrato de P1 | `codec.ts`, `p256.ts`, `events.ts` | `p1.test.ts` y `p1_test.go`: credenciales y claves reales de P1; `events.test.ts`: eventos que acepta la API |
| Cola de eventos y billetera | `outbox.ts`, `wallet.ts`, `qrPath.ts` | `outbox.test.ts` (envío en orden, reintento, descarte, tope de 500), `wallet.test.ts` (respuesta de P1, credenciales rotas, vencidas, trazo del QR ida y vuelta) |
| Service worker | `web/public/sw-p4.js` | `sw.test.ts`: simulador de la Cache API; nunca toca `/api/*`, navegación sin red, caché de `/assets/` |
| Almacén IndexedDB | `store.ts` | `store.test.ts` sobre `test/fake-idb.ts`: llave única, blob de revocaciones, outbox concurrente y atómico, una sola conexión |
| Flujo completo | todo el núcleo | `flow.test.ts`: escenarios de la demo y modo avión |
| Robustez | `verify.ts` | `fuzz.test.ts` (3.000 mutaciones) y `FuzzVerifyPresentation` (Go) |
| Go: tipos y códec | `slj.go` | `codec_test.go` |
| Go: CLI y vectores | `main.go`, `vectors.go` | `cli_test.go`, `vectors_test.go` |
| Despliegue | `render.yaml`, laboratorio | `repo.test.ts` |
| Interoperabilidad | TS ↔ Go | `conformance.test.ts`, `TestVectorsRoundTrip`, `TestVerifyTSPresentation` |

## Escenarios de la demo y código esperado

| Escenario | Esperado |
| :---- | :---- |
| Ana (mayor de 18) en el bar | 01 |
| Luis solo tiene carnet; el bar no lo acepta | 05 |
| Carnet universitario (A3 del emisor 2) en un verificador que acepta A3 | 01 (emisor 2) |
| María responde BLOQUEAR y llega la nueva lista | 03 |
| Credencial alterada a mano | 02 |
| Captura de pantalla a los 35 s | 07 |
| Mismo QR escaneado dos veces | 07 |
| QR rotando cada 20 s durante 2 minutos | 01 en todos |
| Reloj del verificador +10 s / +31 s | 01 / 07 |
| Modo avión | 01 sin llamar a `fetch`; el evento queda en el outbox |

## Precedencia de códigos

La verificación corta en el primer fallo, en este orden: formato (08) → hora (07) → emisor conocido (08) → **firmas (02)** → vencimiento (04) → permiso (05) → revocación (03) → reuso (07). Las firmas van antes que cualquier resultado que nombre la cédula: un QR falsificado con un handle revocado da 02 y no 03, y no se envía a la API.

## Defectos que encontró el análisis (corregidos en esta rama)

1. **Outbox:** `pushOutbox` leía y escribía en transacciones distintas y `takeOutbox`+`clearOutbox` borraba lo llegado entre ambas. Ahora cada operación es una sola transacción (`drainOutbox` es atómico).
2. **Conexiones:** `store.ts` abría IndexedDB en cada operación; ahora reutiliza una conexión.
3. **Anti-reuso:** la clave de la caché era `handle:t`, así que dos atributos del mismo ciudadano en el mismo segundo daban un 07 falso. Ahora es `handle:attr:t`.
4. **Formato propio frente al contrato:** el verificador usaba un formato de credencial y un atributo (A7) que no eran los de P1, y enviaba resultados 07/08 que la API rechaza con 400. Ahora sigue el contrato de P1 (cabecera `0x01`, valor antes de la clave del celular, carnet = A3 del emisor 2, códigos 01–06 hacia la API) y se prueba con sus credenciales reales.
5. **Atribución de resultados sin autenticar:** 03, 04 y 05 se decidían antes de verificar las firmas, así que un QR falso podía quedar atribuido a una cédula real. Ahora las firmas van primero.

## Lo que estas pruebas no cubren

- IndexedDB, la cámara y WebCrypto en iPhone y Android reales: se validan a mano con `app/public/p4-lab` sobre HTTPS.
- Las pantallas `/billetera` y `/verificar` no tienen pruebas automáticas de interfaz: se recorrieron a mano en el navegador (modo demostración) y se comprueban con `npm run lint` y `npm run build` (CI, job `web`).
- El service worker se prueba en un simulador, no en un navegador real: falta confirmar el registro, la instalación y el modo avión en iPhone y Android.

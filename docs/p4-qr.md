# P4 · QR SLJ1 y verificación

La fuente de verdad es el **contrato de P1** (`docs/contrato.md` en la rama `p1-backend`). P4 no define formatos propios: este documento solo explica cómo lo implementa `app/lib/slj` y qué decisiones quedan de nuestro lado. Todo es big-endian y las firmas son ECDSA P-256 / SHA-256 crudas `r‖s` de 64 B.

## Credencial de billetera (la emite P1)

```
blob = cabecera(16) ‖ valor(largo) ‖ clave pública del celular(33, comprimida) ‖ firma(64)
cabecera = versión(1)=0x01 ‖ emisor(1) ‖ atributo(1) ‖ largo(1) ‖ vence(4) ‖ handle(8)
```

El emisor firma `cabecera ‖ valor ‖ clave del celular` (sin UID). Atributos: A1 Mayor de 18, A2 Mayor de 21, A3 Identidad, A4 Sangre y alergias, A5 Contacto de emergencia, A6 Donante y licencia.

**El carnet universitario no es un atributo nuevo:** es un **A3 firmado por el emisor `0x02`**. El verificador lo distingue por el byte de emisor y valida con la clave de ese emisor (`/api/issuers`).

## QR (lo implementa P4)

```
"SLJ1:" + base64url( blob ‖ hora(4) ‖ firma del celular(64) ‖ atributo(1) )
firma del celular = ECDSA(llave del celular, hora(4 B) ‖ atributo(1 B))
```

El contrato dice "sobre (hora ‖ atributo)" sin dar los bytes exactos; P4 usa la hora como 4 B big-endian seguida del atributo en 1 B. **A confirmar con P1.** Con un valor de 1 B el QR mide 249 caracteres.

La clave del celular dentro del blob está firmada por el emisor, así que la firma del celular no necesita cubrir el blob: quien no tenga la llave no puede producirla.

## Cómo verifica `verifyPresentation`

Corta en el primer fallo, en este orden:

1. Prefijo, largos exactos, versión `0x01` y atributo final igual al del blob (08).
2. `|ahora − hora| ≤ 30 s` (07).
3. El emisor es conocido (08).
4. Las dos firmas, en paralelo: la del emisor sobre el blob y la del celular sobre `hora ‖ atributo` (02).
5. `vence ≥ ahora` (04).
6. El atributo está entre los que acepta ese local (05).
7. El handle no está en la lista de revocación (03).
8. La terna `(handle, atributo, hora)` no se había visto ya (07). Se registra solo después de validar las firmas.

Las firmas van **antes** de cualquier resultado que nombre la cédula (03, 04, 05, 01). Así un QR falsificado nunca puede quedar atribuido a una cédula real en el historial: da 02 y no se envía.

El QR se renueva cada **20 s** y se acepta con ±30 s de ventana: quedan 10 s de margen para el reloj y el escaneo, y una captura deja de servir antes de 30 s.

## Códigos de resultado

Del contrato (los únicos que conocen la API y el UNO): 01 aprobado · 02 firma inválida · 03 revocada · 04 vencida · 05 sin permiso · 06 rechazado por el ciudadano (solo terminal).

**Internos de la pantalla del verificador** (07 y 08): nunca se envían a la API ni llegan al UNO. 07 = QR caducado o ya usado; 08 = no es un QR SLJ1 válido o el emisor no se conoce.

## Qué se envía a `POST /api/events`

`{t:"verif", h, a, res, ts, emg}` con `h` de 16 caracteres hex, `a` de `A1` a `A6` y `res` de `01` a `06`. `toVerifEvent` solo genera evento para 01, 03, 04 y 05, es decir, resultados ya autenticados que la API acepta. **02, 07 y 08 no se envían.** La API rechaza con 400 cualquier otro resultado o atributo.

## Datos que consume el verificador

| Dato | Endpoint | Formato |
| :---- | :---- | :---- |
| Claves de los emisores | `GET /api/issuers` | `{"0x01":JWK,"0x02":JWK}` → `importIssuersJwk` |
| Revocaciones | `GET /api/revocations` | `{hs:[handle hex,...]}` → `revocationsFromHex`, se guarda como blob de n × 8 B |
| Credenciales | `GET /api/wallet`, `POST /api/link/confirm` | `blob_hex` por atributo; la clave del celular va en hex de 66 caracteres |

## Pruebas contra P1

`app/lib/slj/testdata/p1-*.json` son copias de `docs/credenciales-billetera-prueba.json` y `keys/issuers.json` de P1 (claves de prueba públicas). `p1.test.ts` (TS) y `p1_test.go` (Go) verifican QR hechos con **esas** credenciales y con el celular de prueba de P1.

## Herramientas

```bash
node --test "app/lib/slj/**/*.test.ts"
cd tools/slj && go test ./...
cd tools/slj && go run . present                       # QR SLJ1 fresco del dispositivo de prueba
cd tools/slj && go run . verify -keys ../../app/lib/slj/testdata/p1-issuers.json -rev revocations.json 'SLJ1:...'
```

`verify -keys` acepta el `keys/issuers.json` de P1 y `-rev` acepta el JSON de `/api/revocations`. Las claves de `tools/slj` y de los vectores son **solo de prueba**.

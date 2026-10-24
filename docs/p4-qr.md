# P4 · Credencial de billetera y QR SLJ1

Propuesta de P4 para cerrar con P1 en `docs/contrato.md`. Todo en big-endian; firmas ECDSA P-256 / SHA-256 **raw r‖s de 64 B** (IEEE P1363, el formato nativo de WebCrypto).

> P1: en Node firmar con `crypto.sign('sha256', data, { key, dsaEncoding: 'ieee-p1363' })`.

## Credencial de billetera (ver = 0x02)

| Campo | Tamaño | Notas |
| :---- | :---- | :---- |
| ver | 1 | `0x02` (el chip usa `0x01`) |
| iss | 1 | 0x01 nacional, 0x02 universidad |
| attr | 1 | 1 mayor18, 2 mayor21, 3 identidad, 4 sangre, 5 emergencia, 6 licencia, 7 carnet |
| len | 1 | bytes de `value` |
| exp | 4 | Unix, segundos |
| handle | 8 | seudónimo de la cédula |
| devpub | 33 | clave pública P-256 **comprimida** del celular |
| value | len | el dato |
| sig | 64 | del emisor sobre todos los campos anteriores (sin UID) |

## Presentación QR

```
"SLJ1:" + base64url( cred ‖ t(4) ‖ devSig(64) ‖ attr(1) )
devSig = ECDSA(devKey, "SLJ1" ‖ t ‖ cred)
```

Son 249 caracteres con `len = 1`: un QR de versión ~9–10 con corrección M.

El verificador comprueba, en este orden y cortando en el primer fallo:

1. Prefijo, largos exactos y `attr` final igual al de la credencial (08).
2. `|ahora − t| ≤ 30 s` (07).
3. `exp ≥ ahora` (04).
4. El atributo está entre los que acepta ese verificador (05).
5. El handle no está en la lista de revocación (03).
6. El emisor es conocido (08).
7. Las dos firmas, en paralelo: emisor sobre la credencial y celular sobre `t ‖ cred` (02).
8. La pareja `(handle, t)` no se había visto ya (07). Se registra solo después de validar las firmas.

El QR se renueva cada **20 s**, así que una captura deja de servir antes de 30 s y quedan 10 s de margen para el desfase de reloj y el escaneo.

## Códigos de resultado

01 aprobado · 02 firma inválida · 03 revocada · 04 vencida · 05 sin permiso · 06 rechazado por el ciudadano (solo terminal) · **07 presentación caducada o repetida** · **08 formato inválido o emisor desconocido**. Los códigos 07 y 08 son propuesta de P4; P1 debe admitirlos en `events`.

## Herramientas

```bash
node --test app/lib/slj/slj.test.ts        # unitarios + conformidad + benchmark
cd tools/slj && go test ./...             # Go verifica los vectores y una presentación de WebCrypto
cd tools/slj && go run . vectors -out ../../app/lib/slj/testdata/vectors.json
cd tools/slj && go run . present          # QR SLJ1 fresco del dispositivo de prueba
cd tools/slj && go run . verify -keys ../../keys/issuers.json 'SLJ1:...'   # verificador independiente
```

Las claves de `tools/slj` y de los vectores son **solo de prueba** y están derivadas de semillas públicas; nunca son las del emisor real.

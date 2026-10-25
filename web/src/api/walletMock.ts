import { concat, fromHex, toHex } from '../lib/slj/bytes.ts'
import { encodeUnsigned } from '../lib/slj/codec.ts'
import { sign } from '../lib/slj/p256.ts'
import { ATTR, ISSUER } from '../lib/slj/tables.ts'
import { ApiError } from './client'

// Emisor de demostración que vive solo en memoria de la pestaña, para trabajar sin backend
// (VITE_USE_MOCK=true). En producción esta rama no se ejecuta y el compilador la descarta.
type Issuer = { priv: CryptoKey; jwk: JsonWebKey }

let issuersPromise: Promise<Record<number, Issuer>> | null = null

function issuers(): Promise<Record<number, Issuer>> {
  issuersPromise ??= (async () => {
    const make = async (): Promise<Issuer> => {
      const pair = (await crypto.subtle.generateKey({ name: 'ECDSA', namedCurve: 'P-256' }, true, [
        'sign',
        'verify',
      ])) as CryptoKeyPair
      const jwk = await crypto.subtle.exportKey('jwk', pair.publicKey)
      return { priv: pair.privateKey, jwk: { kty: jwk.kty, crv: jwk.crv, x: jwk.x, y: jwk.y } }
    }
    return { [ISSUER.NACIONAL]: await make(), [ISSUER.UNIVERSIDAD]: await make() }
  })()
  return issuersPromise
}

const issued = new Map<string, string[]>() // clave pública del celular -> blobs en hex

async function issueFor(publicKeyHex: string): Promise<string[]> {
  const keys = await issuers()
  const devpub = fromHex(publicKeyHex)
  const handle = crypto.getRandomValues(new Uint8Array(8))
  const handleBig = new DataView(handle.buffer).getBigUint64(0)
  const exp = Math.floor(Date.now() / 1000) + 365 * 86400
  const specs = [
    { issuer: ISSUER.NACIONAL, attr: ATTR.MAYOR18, value: new Uint8Array([1]) },
    { issuer: ISSUER.NACIONAL, attr: ATTR.MAYOR21, value: new Uint8Array([1]) },
    { issuer: ISSUER.NACIONAL, attr: ATTR.DONANTE, value: new TextEncoder().encode('D;LIC-B2') },
    { issuer: ISSUER.UNIVERSIDAD, attr: ATTR.IDENTIDAD, value: new TextEncoder().encode('ANA PEREZ|20-12345|2030-01-01') },
  ]
  const blobs: string[] = []
  for (const spec of specs) {
    const unsigned = encodeUnsigned({ ...spec, exp, handle: handleBig, devpub })
    const sig = await sign(keys[spec.issuer].priv, unsigned)
    blobs.push(toHex(concat(unsigned, sig)))
  }
  return blobs
}

const asCredentials = (blobs: string[]) => ({ credentials: blobs.map((blob_hex) => ({ blob_hex })) })

export async function mockConfirmLink(code: string, publicKeyHex: string) {
  if (!/^\d{6}$/.test(code)) throw new ApiError(400, 'El código no es válido o ya venció.')
  if (!/^0[23][0-9a-f]{64}$/i.test(publicKeyHex)) throw new ApiError(400, 'La clave del celular no es válida.')
  const blobs = await issueFor(publicKeyHex)
  issued.set(publicKeyHex, blobs)
  return asCredentials(blobs)
}

export async function mockGetWallet(publicKeyHex: string) {
  return asCredentials(issued.get(publicKeyHex) ?? [])
}

export async function mockGetIssuers(): Promise<Record<string, JsonWebKey>> {
  const keys = await issuers()
  return { '0x01': keys[ISSUER.NACIONAL].jwk, '0x02': keys[ISSUER.UNIVERSIDAD].jwk }
}

export async function mockGetRevocations() {
  return { hs: [] as string[] }
}

export async function mockPostEvent(): Promise<void> {
  // Sin backend no hay historial al que enviar: el evento se da por entregado.
}

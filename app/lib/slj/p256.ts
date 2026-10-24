// P-256: compresión de puntos y utilidades de claves sobre WebCrypto.

const P = 0xffffffff00000001000000000000000000000000ffffffffffffffffffffffffn;
const B = 0x5ac635d8aa3a93e7b3ebbd55769886bc651d06b0cc53b0f63bce3c3e27d2604bn;

function modpow(b: bigint, e: bigint, m: bigint): bigint {
  let r = 1n;
  b %= m;
  while (e > 0n) {
    if (e & 1n) r = (r * b) % m;
    b = (b * b) % m;
    e >>= 1n;
  }
  return r;
}

function bytesToBig(b: Uint8Array): bigint {
  let n = 0n;
  for (const x of b) n = (n << 8n) | BigInt(x);
  return n;
}

function bigToBytes(n: bigint, len: number): Uint8Array {
  const out = new Uint8Array(len);
  for (let i = len - 1; i >= 0; i--) {
    out[i] = Number(n & 0xffn);
    n >>= 8n;
  }
  return out;
}

/** 65 B (0x04||X||Y) -> 33 B (0x02/0x03||X). */
export function compressPoint(raw: Uint8Array): Uint8Array {
  if (raw.length !== 65 || raw[0] !== 4) throw new Error("punto no comprimible");
  const out = new Uint8Array(33);
  out[0] = 2 + (raw[64] & 1);
  out.set(raw.subarray(1, 33), 1);
  return out;
}

/** 33 B -> 65 B, o null si el punto no está en la curva. */
export function decompressPoint(c: Uint8Array): Uint8Array | null {
  if (c.length !== 33 || (c[0] !== 2 && c[0] !== 3)) return null;
  const x = bytesToBig(c.subarray(1));
  if (x >= P) return null;
  const rhs = (((((x * x) % P) * x - 3n * x + B) % P) + P) % P;
  let y = modpow(rhs, (P + 1n) >> 2n, P); // P ≡ 3 (mod 4)
  if ((y * y) % P !== rhs) return null;
  if (Number(y & 1n) !== (c[0] & 1)) y = P - y;
  const out = new Uint8Array(65);
  out[0] = 4;
  out.set(bigToBytes(x, 32), 1);
  out.set(bigToBytes(y, 32), 33);
  return out;
}

const ALG = { name: "ECDSA", namedCurve: "P-256" } as const;
const SIGALG = { name: "ECDSA", hash: "SHA-256" } as const;

export function importPublic(raw65: Uint8Array): Promise<CryptoKey> {
  return crypto.subtle.importKey("raw", raw65 as BufferSource, ALG, false, ["verify"]);
}

/** Acepta 33 B o 65 B. Devuelve null si la clave no es válida. */
export async function importPublicAny(pub: Uint8Array): Promise<CryptoKey | null> {
  const raw = pub.length === 33 ? decompressPoint(pub) : pub.length === 65 ? pub : null;
  if (!raw) return null;
  try {
    return await importPublic(raw);
  } catch {
    return null;
  }
}

/** Importa una sola vez las claves públicas de los emisores (id -> bytes de 33 o 65 B). */
export async function importIssuers(keys: Record<number, Uint8Array>): Promise<Map<number, CryptoKey>> {
  const m = new Map<number, CryptoKey>();
  for (const [id, pub] of Object.entries(keys)) {
    const k = await importPublicAny(pub);
    if (k) m.set(Number(id), k);
  }
  return m;
}

/** `GET /api/issuers` entrega `{"0x01":JWK,"0x02":JWK}`; se importan una sola vez. Las claves inválidas se descartan. */
export async function importIssuersJwk(jwks: Record<string, JsonWebKey>): Promise<Map<number, CryptoKey>> {
  const m = new Map<number, CryptoKey>();
  for (const [id, jwk] of Object.entries(jwks)) {
    const n = Number(id);
    if (!Number.isInteger(n) || n < 0 || n > 255) continue;
    try {
      m.set(n, await crypto.subtle.importKey("jwk", { kty: jwk.kty, crv: jwk.crv, x: jwk.x, y: jwk.y }, ALG, false, ["verify"]));
    } catch {
      /* clave inválida: ese emisor queda desconocido */
    }
  }
  return m;
}

/** Llave del dispositivo: no extraíble (la privada nunca sale de WebCrypto). */
export async function generateDeviceKey(): Promise<CryptoKeyPair> {
  return (await crypto.subtle.generateKey(ALG, false, ["sign", "verify"])) as CryptoKeyPair;
}

export async function exportCompressed(pub: CryptoKey): Promise<Uint8Array> {
  return compressPoint(new Uint8Array(await crypto.subtle.exportKey("raw", pub)));
}

export async function sign(key: CryptoKey, msg: Uint8Array): Promise<Uint8Array> {
  return new Uint8Array(await crypto.subtle.sign(SIGALG, key, msg as BufferSource));
}

export function verify(key: CryptoKey, sig: Uint8Array, msg: Uint8Array): Promise<boolean> {
  return crypto.subtle.verify(SIGALG, key, sig as BufferSource, msg as BufferSource);
}

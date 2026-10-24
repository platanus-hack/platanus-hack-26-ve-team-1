import {
  ATTR, ISSUER, concat, createPresentation, decompressPoint, encodeUnsigned, exportCompressed,
  generateDeviceKey, importIssuers, sign, type VerifyContext,
} from "../index.ts";

export const NOW = 1_800_000_000;
export const HANDLE = 0x1122334455667788n;

export async function issuerPair() {
  const kp = (await crypto.subtle.generateKey({ name: "ECDSA", namedCurve: "P-256" }, true, ["sign", "verify"])) as CryptoKeyPair;
  const pub = new Uint8Array(await crypto.subtle.exportKey("raw", kp.publicKey));
  return { kp, pub };
}

type Issue = Partial<Parameters<typeof encodeUnsigned>[0]>;

/** Un emisor nacional (1), una universidad (2) y un celular, con todo lo necesario para emitir y verificar. */
export async function makeWorld() {
  const nacional = await issuerPair();
  const universidad = await issuerPair();
  const dev = await generateDeviceKey();
  const devpub = await exportCompressed(dev.publicKey);
  const issuers = await importIssuers({ [ISSUER.NACIONAL]: nacional.pub, [ISSUER.UNIVERSIDAD]: universidad.pub });
  const pairs: Record<number, { kp: CryptoKeyPair }> = { 1: nacional, 2: universidad };

  const issue = async (over: Issue = {}) => {
    const issuer = over.issuer ?? ISSUER.NACIONAL;
    const unsigned = encodeUnsigned({
      issuer, attr: ATTR.MAYOR18, exp: NOW + 86400, handle: HANDLE, devpub, value: new Uint8Array([1]), ...over,
    });
    const signer = (pairs[issuer] ?? nacional).kp.privateKey;
    return concat(unsigned, await sign(signer, unsigned));
  };
  const present = (cred: Uint8Array, t = NOW) => createPresentation(cred, dev.privateKey, t);
  const ctx = (extra: Partial<VerifyContext> = {}): VerifyContext => ({ issuers, revoked: new Set(), now: NOW, ...extra });
  return { nacional, universidad, dev, devpub, issuers, issue, present, ctx };
}

/** Busca un x pequeño que no pertenezca a la curva P-256 y devuelve una clave comprimida inválida. */
export function offCurveKey(): Uint8Array {
  for (let x = 1; x < 100; x++) {
    const c = new Uint8Array(33);
    c[0] = 2;
    c[32] = x;
    if (decompressPoint(c) === null) return c;
  }
  throw new Error("no se encontró un x fuera de la curva");
}

/** PRNG determinista (mulberry32) para que los fuzz sean reproducibles. */
export function rng(seed: number) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/** Sustituye fetch por una función que falla y cuenta las llamadas (prueba de modo avión). */
export function noNetwork() {
  const original = globalThis.fetch;
  const state = { calls: 0 };
  globalThis.fetch = (() => {
    state.calls++;
    throw new Error("sin red: no se debe llamar a fetch");
  }) as typeof fetch;
  return { state, restore: () => { globalThis.fetch = original; } };
}

/** Quita las APIs nativas de base64 de Uint8Array para forzar la ruta de respaldo con atob/btoa. */
export function withoutNativeBase64<T>(f: () => T): T {
  const U8 = Uint8Array as any;
  const fromBase64 = U8.fromBase64;
  const toBase64 = U8.prototype.toBase64;
  delete U8.fromBase64;
  delete U8.prototype.toBase64;
  try {
    return f();
  } finally {
    if (fromBase64) U8.fromBase64 = fromBase64;
    if (toBase64) U8.prototype.toBase64 = toBase64;
  }
}

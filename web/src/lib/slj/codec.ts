import { readU32, readU64, u32, concat, fromB64u, toB64u, fromHex } from "./bytes.ts";
import { PREFIX, VER } from "./tables.ts";

/**
 * Credencial de billetera (contrato de P1):
 *   blob = cabecera(16) ‖ valor(largo) ‖ clave pública del celular(33) ‖ firma(64)
 *   cabecera = versión(1)=0x01 ‖ emisor(1) ‖ atributo(1) ‖ largo(1) ‖ vence(4) ‖ handle(8)
 *   el emisor firma cabecera ‖ valor ‖ clave del celular (sin UID).
 */
export interface Credential {
  ver: number;
  issuer: number;
  attr: number;
  exp: number;
  handle: bigint;
  value: Uint8Array;
  devpub: Uint8Array; // 33 B comprimida
  sig: Uint8Array; // 64 B raw r||s
  signed: Uint8Array; // bytes que cubre la firma del emisor
  raw: Uint8Array; // blob completo
}

export interface Presentation {
  cred: Credential;
  t: number;
  devSig: Uint8Array;
  attr: number;
}

const HEADER = 16;
const SIG = 64;
const DEVPUB = 33;

export const blobLength = (valueLen: number): number => HEADER + valueLen + DEVPUB + SIG;

/** Parsea el blob sin copiar (subarray). null si el formato no cuadra exactamente. */
export function parseCredential(b: Uint8Array): Credential | null {
  if (b.length < HEADER + DEVPUB + SIG || b[0] !== VER) return null;
  const len = b[3];
  if (b.length !== blobLength(len)) return null;
  const vEnd = HEADER + len;
  const pEnd = vEnd + DEVPUB;
  return {
    ver: b[0],
    issuer: b[1],
    attr: b[2],
    exp: readU32(b, 4),
    handle: readU64(b, 8),
    value: b.subarray(HEADER, vEnd),
    devpub: b.subarray(vEnd, pEnd),
    sig: b.subarray(pEnd),
    signed: b.subarray(0, pEnd),
    raw: b,
  };
}

export interface CredentialFields {
  issuer: number;
  attr: number;
  exp: number;
  handle: bigint;
  devpub: Uint8Array;
  value: Uint8Array;
}

/** Mensaje que firma el emisor: cabecera ‖ valor ‖ clave del celular. */
export function encodeUnsigned(f: CredentialFields): Uint8Array {
  const h = new Uint8Array(8);
  new DataView(h.buffer).setBigUint64(0, f.handle);
  return concat(new Uint8Array([VER, f.issuer, f.attr, f.value.length]), u32(f.exp), h, f.value, f.devpub);
}

/**
 * QR: "SLJ1:" + base64url( blob ‖ hora(4) ‖ firma del celular(64) sobre (hora ‖ atributo) ‖ atributo(1) )
 */
export function parsePresentation(qr: string): Presentation | null {
  if (!qr.startsWith(PREFIX)) return null;
  const b = fromB64u(qr.slice(PREFIX.length));
  if (!b || b.length < blobLength(0) + 4 + SIG + 1) return null;
  const credLen = blobLength(b[3]);
  if (b.length !== credLen + 4 + SIG + 1) return null;
  const cred = parseCredential(b.subarray(0, credLen));
  if (!cred) return null;
  return {
    cred,
    t: readU32(b, credLen),
    devSig: b.subarray(credLen + 4, credLen + 4 + SIG),
    attr: b[credLen + 4 + SIG],
  };
}

export function encodePresentation(blob: Uint8Array, t: number, devSig: Uint8Array, attr: number): string {
  return PREFIX + toB64u(concat(blob, u32(t), devSig, new Uint8Array([attr])));
}

/** Lista de revocación guardada en el celular: n × 8 B big-endian. */
export function parseRevocations(b: Uint8Array): Set<bigint> {
  const s = new Set<bigint>();
  const dv = new DataView(b.buffer, b.byteOffset, b.byteLength);
  for (let o = 0; o + 8 <= b.length; o += 8) s.add(dv.getBigUint64(o));
  return s;
}

/** `GET /api/revocations` entrega `{hs:[handle hex de 16 caracteres,...]}`; se guarda como blob binario. */
export function revocationsFromHex(hs: string[]): Uint8Array {
  const out = new Uint8Array(hs.length * 8);
  hs.forEach((h, i) => {
    if (!/^[0-9a-fA-F]{16}$/.test(h)) throw new Error("handle inválido en la lista de revocación");
    out.set(fromHex(h), i * 8);
  });
  return out;
}

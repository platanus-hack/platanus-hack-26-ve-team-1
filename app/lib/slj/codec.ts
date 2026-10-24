import { readU32, readU64, u32, concat, fromB64u, toB64u } from "./bytes.ts";
import { PREFIX, VER_CHIP, VER_WALLET } from "./tables.ts";

export interface Credential {
  ver: number;
  issuer: number;
  attr: number;
  exp: number;
  handle: bigint;
  devpub: Uint8Array | null; // 33 B comprimida; solo ver=0x02
  value: Uint8Array;
  sig: Uint8Array; // 64 B raw r||s
  signed: Uint8Array; // bytes cubiertos por la firma del emisor (sin UID)
  raw: Uint8Array; // credencial completa
}

export interface Presentation {
  cred: Credential;
  t: number;
  devSig: Uint8Array;
  attr: number;
}

const HEADER = 16; // ver iss attr len exp(4) handle(8)
const SIG = 64;
const DEVPUB = 33;

export function credLength(len: number, ver: number): number {
  return HEADER + (ver === VER_WALLET ? DEVPUB : 0) + len + SIG;
}

/** Parsea una credencial sin copiar (subarray). Devuelve null si el formato no cuadra exactamente. */
export function parseCredential(b: Uint8Array): Credential | null {
  if (b.length < HEADER + SIG) return null;
  const ver = b[0];
  if (ver !== VER_CHIP && ver !== VER_WALLET) return null;
  const len = b[3];
  if (b.length !== credLength(len, ver)) return null;
  const wallet = ver === VER_WALLET;
  const vo = HEADER + (wallet ? DEVPUB : 0);
  const end = vo + len;
  return {
    ver,
    issuer: b[1],
    attr: b[2],
    exp: readU32(b, 4),
    handle: readU64(b, 8),
    devpub: wallet ? b.subarray(HEADER, HEADER + DEVPUB) : null,
    value: b.subarray(vo, end),
    sig: b.subarray(end, end + SIG),
    signed: b.subarray(0, end),
    raw: b,
  };
}

export interface CredentialFields {
  ver?: number;
  issuer: number;
  attr: number;
  exp: number;
  handle: bigint;
  devpub?: Uint8Array;
  value: Uint8Array;
}

/** Bytes que firma el emisor (todo menos la firma). */
export function encodeUnsigned(f: CredentialFields): Uint8Array {
  const ver = f.ver ?? (f.devpub ? VER_WALLET : VER_CHIP);
  const h = new Uint8Array(8);
  new DataView(h.buffer).setBigUint64(0, f.handle);
  return concat(
    new Uint8Array([ver, f.issuer, f.attr, f.value.length]),
    u32(f.exp),
    h,
    f.devpub ?? new Uint8Array(0),
    f.value,
  );
}

export function parsePresentation(qr: string): Presentation | null {
  if (!qr.startsWith(PREFIX)) return null;
  const b = fromB64u(qr.slice(PREFIX.length));
  if (!b || b.length < HEADER + DEVPUB + SIG + 4 + SIG + 1) return null;
  if (b[0] !== VER_WALLET) return null;
  const credLen = credLength(b[3], VER_WALLET);
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

export function encodePresentation(cred: Uint8Array, t: number, devSig: Uint8Array, attr: number): string {
  return PREFIX + toB64u(concat(cred, u32(t), devSig, new Uint8Array([attr])));
}

/** Lista de revocación: n × 8 B big-endian. */
export function parseRevocations(b: Uint8Array): Set<bigint> {
  const s = new Set<bigint>();
  const dv = new DataView(b.buffer, b.byteOffset, b.byteLength);
  for (let o = 0; o + 8 <= b.length; o += 8) s.add(dv.getBigUint64(o));
  return s;
}

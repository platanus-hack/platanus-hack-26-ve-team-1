// Utilidades binarias sin dependencias. Todo es big-endian.

const B64U = /^[A-Za-z0-9_-]*$/;

export function toB64u(b: Uint8Array): string {
  const n = b as any;
  if (typeof n.toBase64 === "function") return n.toBase64({ alphabet: "base64url", omitPadding: true });
  let s = "";
  for (let i = 0; i < b.length; i++) s += String.fromCharCode(b[i]);
  return btoa(s).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

export function fromB64u(s: string): Uint8Array | null {
  if (!B64U.test(s) || s.length % 4 === 1) return null;
  const U8 = Uint8Array as any;
  if (typeof U8.fromBase64 === "function") {
    try {
      return U8.fromBase64(s, { alphabet: "base64url", lastChunkHandling: "loose" });
    } catch {
      return null;
    }
  }
  try {
    const t = s.replace(/-/g, "+").replace(/_/g, "/");
    const bin = atob(t + "=".repeat((4 - (t.length % 4)) % 4));
    const out = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
    return out;
  } catch {
    return null;
  }
}

export function toHex(b: Uint8Array): string {
  let s = "";
  for (let i = 0; i < b.length; i++) s += b[i].toString(16).padStart(2, "0");
  return s;
}

export function fromHex(h: string): Uint8Array {
  if (h.length % 2 || /[^0-9a-fA-F]/.test(h)) throw new Error("hex inválido");
  const out = new Uint8Array(h.length / 2);
  for (let i = 0; i < out.length; i++) out[i] = parseInt(h.substr(i * 2, 2), 16);
  return out;
}

export function concat(...parts: Uint8Array[]): Uint8Array {
  let n = 0;
  for (const p of parts) n += p.length;
  const out = new Uint8Array(n);
  let o = 0;
  for (const p of parts) {
    out.set(p, o);
    o += p.length;
  }
  return out;
}

export function u32(n: number): Uint8Array {
  const b = new Uint8Array(4);
  new DataView(b.buffer).setUint32(0, n >>> 0);
  return b;
}

export function readU32(b: Uint8Array, o: number): number {
  return new DataView(b.buffer, b.byteOffset, b.byteLength).getUint32(o);
}

export function readU64(b: Uint8Array, o: number): bigint {
  return new DataView(b.buffer, b.byteOffset, b.byteLength).getBigUint64(o);
}

import { fromHex } from "./bytes.ts";
import { parseCredential } from "./codec.ts";
import { attrLabel } from "./tables.ts";

/** Credencial de billetera lista para mostrar. El blob es la fuente de verdad: emisor y atributo salen de él. */
export interface WalletCredential {
  blob: Uint8Array;
  attr: number;
  issuer: number;
  exp: number;
  handle: bigint;
  label: string;
}

export function toWalletCredential(blob: Uint8Array): WalletCredential | null {
  const c = parseCredential(blob);
  if (!c) return null;
  return { blob, attr: c.attr, issuer: c.issuer, exp: c.exp, handle: c.handle, label: attrLabel(c.attr, c.issuer) };
}

/**
 * `POST /api/link/confirm` y `GET /api/wallet` devuelven `{credentials:[{a, blob_hex, expires}]}`.
 * Se ignoran las entradas que no sean un blob válido en vez de romper toda la lista.
 */
export function credentialsFromResponse(credentials: unknown): WalletCredential[] {
  if (!Array.isArray(credentials)) return [];
  const out: WalletCredential[] = [];
  for (const item of credentials) {
    const hex = (item as { blob_hex?: unknown } | null)?.blob_hex;
    if (typeof hex !== "string") continue;
    let blob: Uint8Array;
    try {
      blob = fromHex(hex);
    } catch {
      continue;
    }
    const cred = toWalletCredential(blob);
    if (cred) out.push(cred);
  }
  return out.sort((a, b) => a.attr - b.attr || a.issuer - b.issuer);
}

/** Vigentes a `nowSec`: una credencial vencida ya no se puede presentar. */
export function usable(creds: WalletCredential[], nowSec: number): WalletCredential[] {
  return creds.filter((c) => c.exp >= nowSec);
}

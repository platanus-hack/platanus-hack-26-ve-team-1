import { concat, u32 } from "./bytes.ts";
import { encodePresentation, parseCredential } from "./codec.ts";
import { sign } from "./p256.ts";

/** Lo que firma el celular según el contrato: hora(4 B, big-endian) ‖ atributo(1 B). */
export function deviceMessage(t: number, attr: number): Uint8Array {
  return concat(u32(t), new Uint8Array([attr]));
}

/** Arma el QR `SLJ1:` para un blob de billetera. `nowSec` en segundos Unix. */
export async function createPresentation(blob: Uint8Array, devPriv: CryptoKey, nowSec: number): Promise<string> {
  const c = parseCredential(blob);
  if (!c) throw new Error("credencial de billetera inválida");
  const devSig = await sign(devPriv, deviceMessage(nowSec, c.attr));
  return encodePresentation(blob, nowSec, devSig, c.attr);
}

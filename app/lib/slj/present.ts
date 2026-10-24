import { concat, u32 } from "./bytes.ts";
import { encodePresentation, parseCredential } from "./codec.ts";
import { sign } from "./p256.ts";
import { TAG } from "./tables.ts";

/** Mensaje que firma el celular: "SLJ1" || t || credencial completa. */
export function deviceMessage(t: number, cred: Uint8Array): Uint8Array {
  return concat(TAG, u32(t), cred);
}

/** Arma el QR `SLJ1:` para una credencial de billetera. `nowSec` en segundos Unix. */
export async function createPresentation(cred: Uint8Array, devPriv: CryptoKey, nowSec: number): Promise<string> {
  const c = parseCredential(cred);
  if (!c || !c.devpub) throw new Error("credencial de billetera inválida");
  const devSig = await sign(devPriv, deviceMessage(nowSec, cred));
  return encodePresentation(cred, nowSec, devSig, c.attr);
}

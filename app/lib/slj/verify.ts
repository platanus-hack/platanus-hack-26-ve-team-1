import { parsePresentation } from "./codec.ts";
import { importPublicAny, verify } from "./p256.ts";
import { deviceMessage } from "./present.ts";
import { RESULT, WINDOW_SECONDS, type ResultCode } from "./tables.ts";

export interface VerifyContext {
  issuers: Map<number, CryptoKey>; // claves de emisor ya importadas
  revoked: Set<bigint>;
  now: number; // segundos Unix
  accept?: Set<number>; // atributos que este verificador acepta (vacío/undefined = todos)
  seen?: Map<string, number>; // caché anti-reuso: handle:t -> t
}

export interface VerifyResult {
  code: ResultCode;
  attr?: number;
  issuer?: number;
  handle?: bigint;
  t?: number;
}

/**
 * Verifica un QR SLJ1 sin red. Orden: lo barato primero, firmas en paralelo al final.
 * El reuso se registra solo tras validar las firmas, para que un atacante no envenene la caché.
 */
export async function verifyPresentation(qr: string, ctx: VerifyContext): Promise<VerifyResult> {
  const p = parsePresentation(qr);
  if (!p) return { code: RESULT.BAD_FORMAT };
  const { cred, t } = p;
  const meta = { attr: cred.attr, issuer: cred.issuer, handle: cred.handle, t };
  const res = (code: ResultCode): VerifyResult => ({ code, ...meta });

  if (p.attr !== cred.attr) return res(RESULT.BAD_FORMAT);
  if (Math.abs(ctx.now - t) > WINDOW_SECONDS) return res(RESULT.STALE);
  if (cred.exp < ctx.now) return res(RESULT.EXPIRED);
  if (ctx.accept && ctx.accept.size && !ctx.accept.has(cred.attr)) return res(RESULT.NO_PERMISSION);
  if (ctx.revoked.has(cred.handle)) return res(RESULT.REVOKED);
  const issuerKey = ctx.issuers.get(cred.issuer);
  if (!issuerKey) return res(RESULT.BAD_FORMAT);
  const devKey = await importPublicAny(cred.devpub!);
  if (!devKey) return res(RESULT.BAD_SIG);

  const [okIssuer, okDevice] = await Promise.all([
    verify(issuerKey, cred.sig, cred.signed),
    verify(devKey, p.devSig, deviceMessage(t, cred.raw)),
  ]);
  if (!okIssuer || !okDevice) return res(RESULT.BAD_SIG);

  if (ctx.seen) {
    const key = `${cred.handle}:${cred.attr}:${t}`;
    if (ctx.seen.has(key)) return res(RESULT.STALE);
    ctx.seen.set(key, t);
    for (const [k, ts] of ctx.seen) if (ctx.now - ts > WINDOW_SECONDS * 2) ctx.seen.delete(k);
  }
  return res(RESULT.OK);
}

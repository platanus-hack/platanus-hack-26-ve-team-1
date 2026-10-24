import { test } from "node:test";
import assert from "node:assert/strict";
import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import {
  ATTR, ISSUER, RESULT, concat, createPresentation, compressPoint, decompressPoint, encodeUnsigned,
  fromB64u, fromHex, importIssuers, importPublicAny, parseCredential, parsePresentation,
  parseRevocations, sign, toB64u, toHex, verifyPresentation, generateDeviceKey, exportCompressed,
  type VerifyContext,
} from "./index.ts";

const dir = dirname(fileURLToPath(import.meta.url));
const NOW = 1_800_000_000;

async function issuerPair() {
  const kp = (await crypto.subtle.generateKey({ name: "ECDSA", namedCurve: "P-256" }, true, ["sign", "verify"])) as CryptoKeyPair;
  const pub = new Uint8Array(await crypto.subtle.exportKey("raw", kp.publicKey));
  return { kp, pub };
}

async function makeWorld() {
  const iss = await issuerPair();
  const dev = await generateDeviceKey();
  const devpub = await exportCompressed(dev.publicKey);
  const issuers = await importIssuers({ [ISSUER.NACIONAL]: iss.pub });
  const issue = async (over: Partial<Parameters<typeof encodeUnsigned>[0]> = {}) => {
    const unsigned = encodeUnsigned({
      issuer: ISSUER.NACIONAL, attr: ATTR.MAYOR18, exp: NOW + 86400, handle: 0x1122334455667788n,
      devpub, value: new Uint8Array([1]), ...over,
    });
    return concat(unsigned, await sign(iss.kp.privateKey, unsigned));
  };
  const ctx = (extra: Partial<VerifyContext> = {}): VerifyContext => ({ issuers, revoked: new Set(), now: NOW, ...extra });
  return { iss, dev, devpub, issuers, issue, ctx };
}

test("base64url ida y vuelta, y rechazo de basura", () => {
  const b = new Uint8Array([0xfb, 0xff, 0x00, 0x01, 0x7f]);
  assert.deepEqual(fromB64u(toB64u(b)), b);
  assert.equal(fromB64u("a+b"), null);
  assert.equal(fromB64u("abcde"), null);
});

test("compresión de punto coincide con el export de WebCrypto", async () => {
  for (let i = 0; i < 20; i++) {
    const { pub } = await issuerPair();
    const c = compressPoint(pub);
    assert.deepEqual(decompressPoint(c), pub);
  }
  // aproximadamente la mitad de las x no tienen y válida
  let bad: Uint8Array | null = null;
  for (let x = 1; x < 50 && !bad; x++) {
    const c = new Uint8Array(33);
    c[0] = 2;
    c[32] = x;
    if (decompressPoint(c) === null) bad = c;
  }
  assert.ok(bad, "debe existir un x fuera de la curva");
  assert.equal(await importPublicAny(bad), null);
});

test("codec: credencial de billetera ida y vuelta", async () => {
  const w = await makeWorld();
  const cred = await w.issue();
  const c = parseCredential(cred)!;
  assert.equal(c.attr, ATTR.MAYOR18);
  assert.equal(c.handle, 0x1122334455667788n);
  assert.deepEqual(c.devpub, w.devpub);
  assert.equal(parseCredential(cred.subarray(0, cred.length - 1)), null);
  const qr = await createPresentation(cred, w.dev.privateKey, NOW);
  const p = parsePresentation(qr)!;
  assert.equal(p.t, NOW);
  assert.equal(p.attr, ATTR.MAYOR18);
  assert.equal(parsePresentation(qr.slice(0, -4)), null);
  assert.equal(parsePresentation("XXX:" + qr.slice(5)), null);
});

test("verificación: todos los códigos de resultado", async () => {
  const w = await makeWorld();
  const cred = await w.issue();
  const qr = await createPresentation(cred, w.dev.privateKey, NOW);
  const code = async (q: string, ctx = w.ctx()) => (await verifyPresentation(q, ctx)).code;

  assert.equal(await code(qr), RESULT.OK);
  assert.equal(await code(qr, w.ctx({ now: NOW + 30 })), RESULT.OK);
  assert.equal(await code(qr, w.ctx({ now: NOW + 31 })), RESULT.STALE);
  assert.equal(await code(qr, w.ctx({ now: NOW - 31 })), RESULT.STALE);
  assert.equal(await code(qr, w.ctx({ revoked: new Set([0x1122334455667788n]) })), RESULT.REVOKED);
  assert.equal(await code(qr, w.ctx({ accept: new Set([ATTR.MAYOR21]) })), RESULT.NO_PERMISSION);
  assert.equal(await code(qr, w.ctx({ accept: new Set([ATTR.MAYOR18]) })), RESULT.OK);

  const expired = await w.issue({ exp: NOW - 1 });
  assert.equal(await code(await createPresentation(expired, w.dev.privateKey, NOW)), RESULT.EXPIRED);

  const otherIssuer = await w.issue({ issuer: 9 });
  assert.equal(await code(await createPresentation(otherIssuer, w.dev.privateKey, NOW)), RESULT.BAD_FORMAT);

  const other = await makeWorld(); // emisor y celular ajenos
  assert.equal(await code(await createPresentation(await other.issue(), other.dev.privateKey, NOW)), RESULT.BAD_SIG);

  // cédula de otro celular: la credencial es válida pero no la firma el dueño de devpub
  assert.equal(await code(await createPresentation(cred, other.dev.privateKey, NOW)), RESULT.BAD_SIG);

  const raw = fromB64u(qr.slice(5))!;
  const flip = (i: number) => { const c = raw.slice(); c[i] ^= 1; return "SLJ1:" + toB64u(c); };
  assert.equal(await code(flip(20)), RESULT.BAD_SIG); // devpub alterada
  assert.equal(await code(flip(raw.length - 10)), RESULT.BAD_SIG); // firma del celular
  assert.equal(await code(flip(raw.length - 1)), RESULT.BAD_FORMAT); // attr final distinto
  assert.equal(await code("SLJ1:%%%"), RESULT.BAD_FORMAT);
  assert.equal(await code("hola"), RESULT.BAD_FORMAT);
});

test("anti-reuso: la misma presentación no pasa dos veces, y una inválida no envenena la caché", async () => {
  const w = await makeWorld();
  const qr = await createPresentation(await w.issue(), w.dev.privateKey, NOW);
  const seen = new Map<string, number>();
  assert.equal((await verifyPresentation(qr, w.ctx({ seen }))).code, RESULT.OK);
  assert.equal((await verifyPresentation(qr, w.ctx({ seen }))).code, RESULT.STALE);
  const raw = fromB64u(qr.slice(5))!;
  raw[raw.length - 10] ^= 1;
  const seen2 = new Map<string, number>();
  assert.equal((await verifyPresentation("SLJ1:" + toB64u(raw), w.ctx({ seen: seen2 }))).code, RESULT.BAD_SIG);
  assert.equal(seen2.size, 0);
});

test("revocaciones: n×8 B big-endian", () => {
  const b = new Uint8Array(16);
  new DataView(b.buffer).setBigUint64(0, 7n);
  new DataView(b.buffer).setBigUint64(8, 0xffffffffffffffffn);
  const s = parseRevocations(b);
  assert.ok(s.has(7n) && s.has(0xffffffffffffffffn) && s.size === 2);
});

test("benchmark: verifyPresentation", async () => {
  const w = await makeWorld();
  const qr = await createPresentation(await w.issue(), w.dev.privateKey, NOW);
  const ctx = w.ctx();
  for (let i = 0; i < 20; i++) await verifyPresentation(qr, ctx);
  const N = 200;
  const t0 = performance.now();
  for (let i = 0; i < N; i++) await verifyPresentation(qr, ctx);
  const ms = (performance.now() - t0) / N;
  console.log(`  verifyPresentation: ${ms.toFixed(2)} ms/op, QR de ${qr.length} caracteres`);
  assert.ok(ms < 5, `demasiado lento: ${ms} ms`);
});

test("conformidad: vectores firmados por Go (tools/slj)", { skip: !existsSync(join(dir, "testdata/vectors.json")) }, async () => {
  const v = JSON.parse(readFileSync(join(dir, "testdata/vectors.json"), "utf8"));
  const issuers = await importIssuers(Object.fromEntries(Object.entries(v.issuers).map(([k, h]) => [k, fromHex(h as string)])));
  for (const c of v.cases) {
    const ctx: VerifyContext = {
      issuers, now: c.now,
      revoked: new Set((c.revoked ?? []).map((h: string) => BigInt("0x" + h))),
      accept: c.accept ? new Set<number>(c.accept) : undefined,
    };
    assert.equal((await verifyPresentation(c.qr, ctx)).code, c.expect, `vector ${c.name}`);
  }
});

test("conformidad: presentación TS para que la verifique Go", async () => {
  const w = await makeWorld();
  const cred = await w.issue();
  const qr = await createPresentation(cred, w.dev.privateKey, NOW);
  writeFileSync(join(dir, "testdata/ts-presentation.json"), JSON.stringify({
    now: NOW, qr, issuers: { [ISSUER.NACIONAL]: toHex(w.iss.pub) }, expect: RESULT.OK,
  }, null, 2) + "\n");
});

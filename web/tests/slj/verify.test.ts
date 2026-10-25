import { test } from "node:test";
import assert from "node:assert/strict";
import {
  ATTR, ISSUER, RESULT, WINDOW_SECONDS, fromB64u, toB64u, verifyPresentation, type VerifyContext,
} from "../../src/lib/slj/index.ts";
import { HANDLE, makeWorld, NOW, offCurveKey } from "./helpers/helpers.ts";

const code = async (qr: string, ctx: VerifyContext) => (await verifyPresentation(qr, ctx)).code;

test("aprobado devuelve los metadatos de la credencial", async () => {
  const w = await makeWorld();
  const r = await verifyPresentation(await w.present(await w.issue()), w.ctx());
  assert.deepEqual(r, { code: RESULT.OK, attr: ATTR.MAYOR18, issuer: ISSUER.NACIONAL, handle: HANDLE, t: NOW });
});

test("ventana de hora: ±30 s aprueba, ±31 s caduca", async () => {
  const w = await makeWorld();
  const qr = await w.present(await w.issue(), NOW);
  for (const d of [-WINDOW_SECONDS, 0, WINDOW_SECONDS]) assert.equal(await code(qr, w.ctx({ now: NOW + d })), RESULT.OK, `Δ${d}`);
  for (const d of [-WINDOW_SECONDS - 1, WINDOW_SECONDS + 1]) assert.equal(await code(qr, w.ctx({ now: NOW + d })), RESULT.STALE, `Δ${d}`);
});

test("vencimiento: exp == ahora aprueba, exp == ahora - 1 vence", async () => {
  const w = await makeWorld();
  assert.equal(await code(await w.present(await w.issue({ exp: NOW })), w.ctx()), RESULT.OK);
  assert.equal(await code(await w.present(await w.issue({ exp: NOW - 1 })), w.ctx()), RESULT.EXPIRED);
});

test("permiso: accept vacío equivale a aceptar todo; con lista, solo esos atributos", async () => {
  const w = await makeWorld();
  const qr = await w.present(await w.issue());
  assert.equal(await code(qr, w.ctx({ accept: new Set() })), RESULT.OK);
  assert.equal(await code(qr, w.ctx({ accept: new Set([ATTR.MAYOR18, ATTR.MAYOR21]) })), RESULT.OK);
  assert.equal(await code(qr, w.ctx({ accept: new Set([ATTR.MAYOR21]) })), RESULT.NO_PERMISSION);
});

test("revocado y emisor desconocido", async () => {
  const w = await makeWorld();
  assert.equal(await code(await w.present(await w.issue()), w.ctx({ revoked: new Set([HANDLE]) })), RESULT.REVOKED);
  assert.equal(await code(await w.present(await w.issue({ issuer: 9 })), w.ctx()), RESULT.BAD_FORMAT);
});

test("firmas: emisor ajeno, celular ajeno, devpub alterada y firma del celular alterada → 02", async () => {
  const w = await makeWorld();
  const other = await makeWorld();
  assert.equal(await code(await other.present(await other.issue()), w.ctx()), RESULT.BAD_SIG);
  assert.equal(await code(await other.present(await w.issue()), w.ctx()), RESULT.BAD_SIG, "credencial de otro celular");
  const raw = fromB64u((await w.present(await w.issue())).slice(5))!;
  for (const i of [20, raw.length - 10]) {
    const c = raw.slice();
    c[i] ^= 1;
    assert.equal(await code("SLJ1:" + toB64u(c), w.ctx()), RESULT.BAD_SIG, `byte ${i}`);
  }
});

test("devpub fuera de la curva → 02", async () => {
  const w = await makeWorld();
  const cred = await w.issue({ devpub: offCurveKey() });
  assert.equal(await code(await w.present(cred), w.ctx()), RESULT.BAD_SIG);
});

test("formato: attr final distinto, prefijo, basura y vacío → 08", async () => {
  const w = await makeWorld();
  const raw = fromB64u((await w.present(await w.issue())).slice(5))!;
  raw[raw.length - 1] ^= 1;
  assert.equal(await code("SLJ1:" + toB64u(raw), w.ctx()), RESULT.BAD_FORMAT);
  for (const s of ["", "hola", "SLJ1:", "SLJ1:%%%", "slj1:AAAA"]) assert.equal(await code(s, w.ctx()), RESULT.BAD_FORMAT, s);
});

test("precedencias: formato > hora > emisor conocido > firmas > vencimiento > permiso > revocación", async () => {
  const w = await makeWorld();
  const revoked = new Set([HANDLE]);
  const other = await makeWorld(); // emisor y celular ajenos: sus QR llevan firmas que no valen aquí
  // desfasado + revocado → 07
  assert.equal(await code(await w.present(await w.issue(), NOW - 100), w.ctx({ revoked })), RESULT.STALE);
  // emisor desconocido + revocado → 08 (ni se miran las firmas ni la lista)
  assert.equal(await code(await w.present(await w.issue({ issuer: 9 })), w.ctx({ revoked })), RESULT.BAD_FORMAT);
  // falsificado + revocado → 02: la revocación NO se atribuye a un QR que no es auténtico
  assert.equal(await code(await other.present(await other.issue()), w.ctx({ revoked })), RESULT.BAD_SIG);
  const raw = fromB64u((await w.present(await w.issue())).slice(5))!;
  raw[raw.length - 10] ^= 1;
  assert.equal(await code("SLJ1:" + toB64u(raw), w.ctx({ revoked })), RESULT.BAD_SIG);
  // falsificado + vencido → 02
  assert.equal(await code(await other.present(await other.issue({ exp: NOW - 1 })), w.ctx()), RESULT.BAD_SIG);
  // auténtico: vencido > sin permiso > revocado
  const accept = new Set([ATTR.MAYOR21]);
  assert.equal(await code(await w.present(await w.issue({ exp: NOW - 1 })), w.ctx({ revoked, accept })), RESULT.EXPIRED);
  assert.equal(await code(await w.present(await w.issue()), w.ctx({ revoked, accept })), RESULT.NO_PERMISSION);
  assert.equal(await code(await w.present(await w.issue()), w.ctx({ revoked })), RESULT.REVOKED);
});

test("anti-reuso: la misma presentación repetida → 07, y una inválida no ensucia la caché", async () => {
  const w = await makeWorld();
  const qr = await w.present(await w.issue());
  const seen = new Map<string, number>();
  assert.equal(await code(qr, w.ctx({ seen })), RESULT.OK);
  assert.equal(await code(qr, w.ctx({ seen })), RESULT.STALE);
  const raw = fromB64u(qr.slice(5))!;
  raw[raw.length - 10] ^= 1;
  const seen2 = new Map<string, number>();
  assert.equal(await code("SLJ1:" + toB64u(raw), w.ctx({ seen: seen2 })), RESULT.BAD_SIG);
  assert.equal(seen2.size, 0);
});

test("anti-reuso: dos atributos distintos del mismo ciudadano en el mismo segundo no chocan", async () => {
  const w = await makeWorld();
  const seen = new Map<string, number>();
  const mayor18 = await w.present(await w.issue({ attr: ATTR.MAYOR18 }));
  const licencia = await w.present(await w.issue({ attr: ATTR.DONANTE }));
  assert.equal(await code(mayor18, w.ctx({ seen })), RESULT.OK);
  assert.equal(await code(licencia, w.ctx({ seen })), RESULT.OK);
});

test("anti-reuso: se podan las entradas de más de 60 s", async () => {
  const w = await makeWorld();
  const seen = new Map<string, number>([["viejo", NOW - 100], ["reciente", NOW - 10]]);
  assert.equal(await code(await w.present(await w.issue()), w.ctx({ seen })), RESULT.OK);
  assert.ok(!seen.has("viejo"));
  assert.ok(seen.has("reciente"));
});

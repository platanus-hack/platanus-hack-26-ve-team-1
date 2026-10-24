import { test } from "node:test";
import assert from "node:assert/strict";
import { ATTR, ISSUER, RESULT, attrLabel, toVerifEvent, verifyPresentation } from "../../src/lib/slj/index.ts";
import { HANDLE, makeWorld, NOW } from "./helpers/helpers.ts";

const run = async (w: Awaited<ReturnType<typeof makeWorld>>, over = {}, ctx = {}) => {
  const r = await verifyPresentation(await w.present(await w.issue(over)), w.ctx(ctx));
  return { r, ev: toVerifEvent(r, NOW) };
};

test("aprobado → evento del contrato de P1: {t,h,a,res,ts,emg}", async () => {
  const w = await makeWorld();
  const { ev } = await run(w);
  assert.deepEqual(ev, { t: "verif", h: "1122334455667788", a: "A1", res: "01", ts: NOW, emg: false });
});

test("el handle va con 16 caracteres aunque empiece en ceros", async () => {
  const w = await makeWorld();
  const { ev } = await run(w, { handle: 0xabcn });
  assert.equal(ev!.h, "0000000000000abc");
});

test("revocada, vencida y sin permiso (01–05 ya autenticados) sí se envían", async () => {
  const w = await makeWorld();
  assert.equal((await run(w, {}, { revoked: new Set([HANDLE]) })).ev!.res, "03");
  assert.equal((await run(w, { exp: NOW - 1 })).ev!.res, "04");
  assert.equal((await run(w, {}, { accept: new Set([ATTR.MAYOR21]) })).ev!.res, "05");
});

test("02, 07 y 08 NO se envían: la API no los conoce o el handle no es de fiar", async () => {
  const w = await makeWorld();
  const other = await makeWorld();
  const forged = await verifyPresentation(await other.present(await other.issue()), w.ctx());
  assert.equal(forged.code, RESULT.BAD_SIG);
  assert.equal(toVerifEvent(forged, NOW), null);
  const stale = await verifyPresentation(await w.present(await w.issue(), NOW - 100), w.ctx());
  assert.equal(stale.code, RESULT.STALE);
  assert.equal(toVerifEvent(stale, NOW), null);
  const unknown = await verifyPresentation(await w.present(await w.issue({ issuer: 9 })), w.ctx());
  assert.equal(unknown.code, RESULT.BAD_FORMAT);
  assert.equal(toVerifEvent(unknown, NOW), null);
  assert.equal(toVerifEvent({ code: RESULT.BAD_FORMAT }, NOW), null);
});

test("el carnet universitario es A3 del emisor 2: el evento lleva A3, nunca un atributo nuevo", async () => {
  const w = await makeWorld();
  const { r, ev } = await run(w, { issuer: ISSUER.UNIVERSIDAD, attr: ATTR.IDENTIDAD, value: new TextEncoder().encode("ANA|123|2030-01-01") });
  assert.equal(r.code, RESULT.OK);
  assert.equal(ev!.a, "A3");
  assert.equal(attrLabel(r.attr!, r.issuer!), "Carnet universitario");
  assert.equal(attrLabel(ATTR.IDENTIDAD, ISSUER.NACIONAL), "Identidad");
});

test("todos los eventos usan solo códigos y atributos que la API acepta (01–06, A1–A6)", async () => {
  const w = await makeWorld();
  for (const attr of [1, 2, 3, 4, 5, 6]) {
    const { ev } = await run(w, { attr });
    assert.match(ev!.a, /^A[1-6]$/);
    assert.match(ev!.res, /^0[1-6]$/);
  }
  assert.equal((await run(w, { attr: 7 })).ev, null, "un atributo fuera del contrato no se envía");
});

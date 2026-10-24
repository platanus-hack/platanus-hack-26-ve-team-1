// Escenarios de la demo (roadmap de P5) de punta a punta: emisión → QR → verificación sin red → outbox.
import { test } from "node:test";
import assert from "node:assert/strict";
import { ATTR, ISSUER, RESULT, ROTATE_SECONDS, attrLabel, fromB64u, parseRevocations, toB64u, verifyPresentation } from "../../src/lib/slj/index.ts";
import { HANDLE, makeWorld, noNetwork, NOW } from "./helpers/helpers.ts";
import { createFakeIDB } from "./helpers/fake-idb.ts";
import * as store from "../../src/lib/slj/store.ts";

const bar = new Set<number>([ATTR.MAYOR18, ATTR.MAYOR21]);
const code = async (qr: string, ctx: Parameters<typeof verifyPresentation>[1]) => (await verifyPresentation(qr, ctx)).code;

test("Ana (mayor de 18) en el bar → aprobado", async () => {
  const w = await makeWorld();
  const qr = await w.present(await w.issue({ attr: ATTR.MAYOR18 }));
  assert.equal(await code(qr, w.ctx({ accept: bar })), RESULT.OK);
});

test("Luis (16) solo tiene carnet (A3 del emisor 2): el bar no lo acepta → sin permiso", async () => {
  const w = await makeWorld();
  const carnet = await w.issue({ issuer: ISSUER.UNIVERSIDAD, attr: ATTR.IDENTIDAD, value: new TextEncoder().encode("LUIS|99|2030-01-01") });
  assert.equal(await code(await w.present(carnet), w.ctx({ accept: bar })), RESULT.NO_PERMISSION);
});

test("carnet universitario (A3, emisor 2) en un verificador que acepta A3 → aprobado, con emisor universidad", async () => {
  const w = await makeWorld();
  const carnet = await w.issue({ issuer: ISSUER.UNIVERSIDAD, attr: ATTR.IDENTIDAD });
  const r = await verifyPresentation(await w.present(carnet), w.ctx({ accept: new Set([ATTR.IDENTIDAD]) }));
  assert.equal(r.code, RESULT.OK);
  assert.equal(r.issuer, ISSUER.UNIVERSIDAD);
  assert.equal(attrLabel(r.attr!, r.issuer!), "Carnet universitario", "A3 del emisor 2 es el carnet");
});

test("María responde BLOQUEAR: antes aprueba, y al llegar la nueva lista de revocación → revocada", async () => {
  const w = await makeWorld();
  const cred = await w.issue();
  assert.equal(await code(await w.present(cred), w.ctx()), RESULT.OK);
  const lista = new Uint8Array(8);
  new DataView(lista.buffer).setBigUint64(0, HANDLE);
  assert.equal(await code(await w.present(cred), w.ctx({ revoked: parseRevocations(lista) })), RESULT.REVOKED);
});

test("credencial alterada a mano → firma inválida", async () => {
  const w = await makeWorld();
  const raw = fromB64u((await w.present(await w.issue())).slice(5))!;
  raw[16 + 33] ^= 1; // el valor del atributo
  assert.equal(await code("SLJ1:" + toB64u(raw), w.ctx()), RESULT.BAD_SIG);
});

test("captura de pantalla: a los 35 s ya no sirve", async () => {
  const w = await makeWorld();
  const qr = await w.present(await w.issue(), NOW);
  assert.equal(await code(qr, w.ctx({ now: NOW + 5 })), RESULT.OK);
  assert.equal(await code(qr, w.ctx({ now: NOW + 35 })), RESULT.STALE);
});

test("el mismo QR escaneado dos veces en la ventana → la segunda es rechazada", async () => {
  const w = await makeWorld();
  const qr = await w.present(await w.issue());
  const seen = new Map<string, number>();
  assert.equal(await code(qr, w.ctx({ seen })), RESULT.OK);
  assert.equal(await code(qr, w.ctx({ seen })), RESULT.STALE);
});

test("rotación cada 20 s durante 2 minutos: cada QR fresco es válido", async () => {
  const w = await makeWorld();
  const cred = await w.issue({ exp: NOW + 3600 });
  const seen = new Map<string, number>();
  for (let s = 0; s <= 120; s += ROTATE_SECONDS) {
    const qr = await w.present(cred, NOW + s);
    assert.equal(await code(qr, w.ctx({ now: NOW + s + 3, seen })), RESULT.OK, `t=+${s}s`);
  }
});

test("reloj del verificador adelantado: +10 s aprueba, +31 s caduca", async () => {
  const w = await makeWorld();
  const qr = await w.present(await w.issue(), NOW);
  assert.equal(await code(qr, w.ctx({ now: NOW + 10 })), RESULT.OK);
  assert.equal(await code(qr, w.ctx({ now: NOW + 31 })), RESULT.STALE);
});

test("modo avión: verifica sin llamar a fetch y el evento queda en el outbox hasta que hay red", async () => {
  const fake = createFakeIDB();
  (globalThis as any).indexedDB = fake.indexedDB;
  const w = await makeWorld();
  const net = noNetwork();
  try {
    const r = await verifyPresentation(await w.present(await w.issue()), w.ctx({ accept: bar }));
    assert.equal(r.code, RESULT.OK);
    await store.pushOutbox({ handle: String(r.handle), attr: r.attr, code: r.code, t: r.t });
    assert.equal(net.state.calls, 0, "la verificación no debe usar la red");
  } finally {
    net.restore();
  }
  // vuelve la red: se vacía el outbox una sola vez
  const sent = await store.drainOutbox();
  assert.equal(sent.length, 1);
  assert.deepEqual(await store.drainOutbox(), []);
});

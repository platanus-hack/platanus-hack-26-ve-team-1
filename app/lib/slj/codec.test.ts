import { test } from "node:test";
import assert from "node:assert/strict";
import {
  ATTR, concat, encodePresentation, encodeUnsigned, parseCredential, parsePresentation, parseRevocations, toB64u,
} from "./index.ts";
import { HANDLE, makeWorld, NOW } from "./test/helpers.ts";

const sig64 = new Uint8Array(64).fill(7);
const devpub = Uint8Array.from({ length: 33 }, (_, i) => (i === 0 ? 2 : i));

test("credencial de billetera: ida y vuelta de todos los campos", async () => {
  const w = await makeWorld();
  const raw = await w.issue({ value: new Uint8Array([1]) });
  const c = parseCredential(raw)!;
  assert.equal(c.ver, 2);
  assert.equal(c.issuer, 1);
  assert.equal(c.attr, ATTR.MAYOR18);
  assert.equal(c.exp, NOW + 86400);
  assert.equal(c.handle, HANDLE);
  assert.deepEqual(c.devpub, w.devpub);
  assert.deepEqual(c.value, new Uint8Array([1]));
  assert.equal(c.sig.length, 64);
  assert.deepEqual(concat(c.signed, c.sig), raw);
});

test("credencial de chip (ver 0x01): sin devpub y con layout propio", () => {
  const unsigned = encodeUnsigned({ issuer: 1, attr: ATTR.MAYOR21, exp: 5, handle: 9n, value: new Uint8Array([1]) });
  assert.equal(unsigned[0], 1, "sin devpub se infiere ver=0x01");
  const c = parseCredential(concat(unsigned, sig64))!;
  assert.equal(c.ver, 1);
  assert.equal(c.devpub, null);
  assert.deepEqual(c.value, new Uint8Array([1]));
  assert.equal(c.handle, 9n);
});

test("largos de valor 0, 1, 64 y 255 se parsean; un byte de más o de menos no", () => {
  for (const len of [0, 1, 64, 255]) {
    const u = encodeUnsigned({ issuer: 1, attr: 3, exp: 1, handle: 1n, devpub, value: new Uint8Array(len).fill(5) });
    const raw = concat(u, sig64);
    assert.equal(parseCredential(raw)!.value.length, len, `len ${len}`);
    assert.equal(parseCredential(concat(raw, new Uint8Array(1))), null, `len ${len} +1`);
    assert.equal(parseCredential(raw.subarray(0, raw.length - 1)), null, `len ${len} -1`);
  }
});

test("versión desconocida o credencial diminuta → null", () => {
  const u = encodeUnsigned({ issuer: 1, attr: 1, exp: 1, handle: 1n, devpub, value: new Uint8Array([1]) });
  const raw = concat(u, sig64).slice();
  raw[0] = 3;
  assert.equal(parseCredential(raw), null);
  raw[0] = 0;
  assert.equal(parseCredential(raw), null);
  assert.equal(parseCredential(new Uint8Array(0)), null);
  assert.equal(parseCredential(new Uint8Array(79)), null);
});

test("presentación: ida y vuelta y rechazo de formatos rotos", async () => {
  const w = await makeWorld();
  const qr = await w.present(await w.issue(), NOW);
  const p = parsePresentation(qr)!;
  assert.equal(p.t, NOW);
  assert.equal(p.attr, ATTR.MAYOR18);
  assert.equal(p.devSig.length, 64);
  assert.equal(p.cred.handle, HANDLE);
  assert.equal(encodePresentation(p.cred.raw, p.t, p.devSig, p.attr), qr);

  assert.equal(parsePresentation(""), null);
  assert.equal(parsePresentation("SLJ1:"), null);
  assert.equal(parsePresentation("SLJ2:" + qr.slice(5)), null);
  assert.equal(parsePresentation(qr.slice(0, -4)), null);
  assert.equal(parsePresentation(qr + "AAAA"), null);
  assert.equal(parsePresentation("SLJ1:" + qr.slice(5) + "!"), null);
});

test("un QR no puede llevar una credencial de chip (ver 0x01)", async () => {
  const u = encodeUnsigned({ issuer: 1, attr: 1, exp: NOW + 1, handle: 1n, value: new Uint8Array([1]) });
  const body = concat(u, sig64, new Uint8Array(4), sig64, new Uint8Array([1]));
  assert.equal(parsePresentation("SLJ1:" + toB64u(body)), null);
});

test("revocaciones: vacía, con elementos y con bytes sobrantes", () => {
  assert.equal(parseRevocations(new Uint8Array(0)).size, 0);
  const b = new Uint8Array(20);
  new DataView(b.buffer).setBigUint64(0, 1n);
  new DataView(b.buffer).setBigUint64(8, 0xffffffffffffffffn);
  const s = parseRevocations(b); // los 4 bytes finales se ignoran
  assert.deepEqual([...s].sort(), [1n, 0xffffffffffffffffn].sort());
  const sub = b.subarray(8, 16); // con byteOffset ≠ 0
  assert.ok(parseRevocations(sub).has(0xffffffffffffffffn));
});

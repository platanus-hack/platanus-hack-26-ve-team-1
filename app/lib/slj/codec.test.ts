import { test } from "node:test";
import assert from "node:assert/strict";
import {
  ATTR, concat, encodePresentation, encodeUnsigned, parseCredential, parsePresentation, parseRevocations,
  revocationsFromHex, toB64u,
} from "./index.ts";
import { HANDLE, makeWorld, NOW } from "./test/helpers.ts";

const sig64 = new Uint8Array(64).fill(7);
const devpub = Uint8Array.from({ length: 33 }, (_, i) => (i === 0 ? 2 : i));
const fields = (len: number) => ({ issuer: 1, attr: 3, exp: 1, handle: 1n, devpub, value: new Uint8Array(len).fill(5) });

test("blob de billetera: cabecera ‖ valor ‖ clave del celular ‖ firma, y la firma del emisor cubre todo menos ella", async () => {
  const w = await makeWorld();
  const raw = await w.issue({ value: new Uint8Array([1]) });
  const c = parseCredential(raw)!;
  assert.equal(c.ver, 1);
  assert.equal(c.issuer, 1);
  assert.equal(c.attr, ATTR.MAYOR18);
  assert.equal(c.exp, NOW + 86400);
  assert.equal(c.handle, HANDLE);
  assert.deepEqual(c.value, new Uint8Array([1]));
  assert.deepEqual(c.devpub, w.devpub);
  assert.equal(c.sig.length, 64);
  assert.deepEqual(concat(c.signed, c.sig), raw);
  // el valor va ANTES de la clave del celular (contrato de P1)
  assert.deepEqual(raw.subarray(16, 17), new Uint8Array([1]));
  assert.deepEqual(raw.subarray(17, 50), w.devpub);
});

test("largos de valor 0, 1, 64 y 255 se parsean; un byte de más o de menos no", () => {
  for (const len of [0, 1, 64, 255]) {
    const raw = concat(encodeUnsigned(fields(len)), sig64);
    assert.equal(parseCredential(raw)!.value.length, len, `len ${len}`);
    assert.equal(parseCredential(concat(raw, new Uint8Array(1))), null, `len ${len} +1`);
    assert.equal(parseCredential(raw.subarray(0, raw.length - 1)), null, `len ${len} -1`);
  }
});

test("versión distinta de 0x01 o credencial diminuta → null", () => {
  const raw = concat(encodeUnsigned(fields(1)), sig64).slice();
  for (const v of [0, 2, 3]) {
    raw[0] = v;
    assert.equal(parseCredential(raw), null, `ver ${v}`);
  }
  assert.equal(parseCredential(new Uint8Array(0)), null);
  assert.equal(parseCredential(new Uint8Array(112)), null);
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

test("un QR con versión de cabecera distinta de 0x01 → null", async () => {
  const body = concat(encodeUnsigned(fields(1)), sig64, new Uint8Array(4), sig64, new Uint8Array([3]));
  assert.ok(parsePresentation("SLJ1:" + toB64u(body)));
  body[0] = 2;
  assert.equal(parsePresentation("SLJ1:" + toB64u(body)), null);
});

test("revocaciones: blob vacío, con elementos y con bytes sobrantes", () => {
  assert.equal(parseRevocations(new Uint8Array(0)).size, 0);
  const b = new Uint8Array(20);
  new DataView(b.buffer).setBigUint64(0, 1n);
  new DataView(b.buffer).setBigUint64(8, 0xffffffffffffffffn);
  const s = parseRevocations(b); // los 4 bytes finales se ignoran
  assert.deepEqual([...s].sort(), [1n, 0xffffffffffffffffn].sort());
  assert.ok(parseRevocations(b.subarray(8, 16)).has(0xffffffffffffffffn), "con byteOffset ≠ 0");
});

test("revocaciones: la lista JSON de P1 ({hs:[hex16]}) se guarda como blob y vuelve igual", () => {
  const hs = ["9e429475799fc8ed", "0000000000000001", "ffffffffffffffff"];
  const blob = revocationsFromHex(hs);
  assert.equal(blob.length, 24);
  assert.deepEqual([...parseRevocations(blob)].sort(), [0x9e429475799fc8edn, 1n, 0xffffffffffffffffn].sort());
  assert.equal(revocationsFromHex([]).length, 0);
  for (const bad of ["zz", "abc", "9e429475799fc8e", "9e429475799fc8edaa"]) {
    assert.throws(() => revocationsFromHex([bad]), undefined, bad);
  }
});

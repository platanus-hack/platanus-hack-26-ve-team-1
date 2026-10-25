import { test } from "node:test";
import assert from "node:assert/strict";
import {
  compressPoint, decompressPoint, exportCompressed, generateDeviceKey, importIssuers, importPublicAny,
} from "../../src/lib/slj/index.ts";
import { issuerPair, offCurveKey } from "./helpers/helpers.ts";

test("compress/decompress: ida y vuelta con ambas paridades de y", async () => {
  const seen = new Set<number>();
  for (let i = 0; i < 40; i++) {
    const { pub } = await issuerPair();
    const c = compressPoint(pub);
    seen.add(c[0]);
    assert.deepEqual(decompressPoint(c), pub);
  }
  assert.deepEqual([...seen].sort(), [2, 3], "debieron aparecer los dos prefijos");
});

test("compressPoint: rechaza prefijo o largo erróneo", () => {
  assert.throws(() => compressPoint(new Uint8Array(64)));
  assert.throws(() => compressPoint(new Uint8Array(65))); // prefijo 0x00
});

test("decompressPoint: rechaza prefijo, largo, x ≥ p y x fuera de la curva", () => {
  assert.equal(decompressPoint(new Uint8Array(32)), null);
  assert.equal(decompressPoint(new Uint8Array(34)), null);
  const badPrefix = new Uint8Array(33);
  badPrefix[0] = 4;
  assert.equal(decompressPoint(badPrefix), null);
  const big = new Uint8Array(33).fill(0xff);
  big[0] = 2; // x = 2^256 - 1 ≥ p
  assert.equal(decompressPoint(big), null);
  assert.equal(decompressPoint(offCurveKey()), null);
});

test("importPublicAny: acepta 33 y 65 B válidos; null para basura", async () => {
  const { pub } = await issuerPair();
  assert.ok(await importPublicAny(pub));
  assert.ok(await importPublicAny(compressPoint(pub)));
  assert.equal(await importPublicAny(offCurveKey()), null);
  assert.equal(await importPublicAny(new Uint8Array(65)), null); // 65 B que no son un punto
  assert.equal(await importPublicAny(new Uint8Array(10)), null);
});

test("importIssuers: descarta claves inválidas sin romper las buenas", async () => {
  const a = await issuerPair();
  const m = await importIssuers({ 1: a.pub, 2: offCurveKey(), 3: new Uint8Array(5) });
  assert.deepEqual([...m.keys()], [1]);
});

test("la llave del dispositivo no deja exportar la privada", async () => {
  const kp = await generateDeviceKey();
  assert.equal(kp.privateKey.extractable, false);
  await assert.rejects(crypto.subtle.exportKey("pkcs8", kp.privateKey));
  assert.equal((await exportCompressed(kp.publicKey)).length, 33);
});

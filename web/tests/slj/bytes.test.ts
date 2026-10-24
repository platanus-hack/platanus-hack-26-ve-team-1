import { test } from "node:test";
import assert from "node:assert/strict";
import { concat, fromB64u, fromHex, readU32, readU64, toB64u, toHex, u32 } from "../../src/lib/slj/bytes.ts";
import { withoutNativeBase64 } from "./helpers/helpers.ts";

const samples = [
  new Uint8Array(0),
  new Uint8Array([0]),
  new Uint8Array([0xfb, 0xff]),
  new Uint8Array([0xfb, 0xff, 0x00, 0x01, 0x7f]),
  Uint8Array.from({ length: 186 }, (_, i) => (i * 37) & 0xff),
];

test("base64url: ida y vuelta, sin relleno y con alfabeto url", () => {
  for (const b of samples) {
    const s = toB64u(b);
    assert.ok(!/[+/=]/.test(s), "solo alfabeto base64url");
    assert.deepEqual(fromB64u(s), b);
  }
});

test("base64url: la ruta de respaldo (atob) da los mismos bytes que la nativa", () => {
  for (const b of samples) {
    const native = toB64u(b);
    const fallback = withoutNativeBase64(() => toB64u(b));
    assert.equal(fallback, native);
    assert.deepEqual(withoutNativeBase64(() => fromB64u(native)), b);
  }
});

test("base64url: rechaza entradas inválidas por ambas rutas", () => {
  const bad = ["a+b", "a/b", "ab=", "abcde", "a", "ñ", "ab cd", "ab\ncd"];
  for (const s of bad) {
    assert.equal(fromB64u(s), null, `nativa: ${JSON.stringify(s)}`);
    assert.equal(withoutNativeBase64(() => fromB64u(s)), null, `respaldo: ${JSON.stringify(s)}`);
  }
});

test("hex: ida y vuelta y rechazo de basura", () => {
  const b = new Uint8Array([0, 1, 0xab, 0xff]);
  assert.equal(toHex(b), "0001abff");
  assert.deepEqual(fromHex("0001ABFF"), b);
  assert.throws(() => fromHex("abc"));
  assert.throws(() => fromHex("zz"));
});

test("concat une en orden y no comparte memoria con las partes", () => {
  const a = new Uint8Array([1, 2]);
  const c = concat(a, new Uint8Array(0), new Uint8Array([3]));
  assert.deepEqual(c, new Uint8Array([1, 2, 3]));
  c[0] = 9;
  assert.equal(a[0], 1);
});

test("u32 y lectores big-endian en los extremos", () => {
  for (const n of [0, 1, 0xff, 0x01020304, 0xffffffff]) {
    assert.equal(readU32(u32(n), 0), n);
  }
  assert.deepEqual(u32(0x01020304), new Uint8Array([1, 2, 3, 4]));
  const b = new Uint8Array(8).fill(0xff);
  assert.equal(readU64(b, 0), 0xffffffffffffffffn);
});

test("lectores sobre un subarray con byteOffset distinto de cero", () => {
  const big = new Uint8Array(32);
  new DataView(big.buffer).setUint32(10, 0xdeadbeef);
  new DataView(big.buffer).setBigUint64(16, 0x0102030405060708n);
  const sub = big.subarray(8);
  assert.notEqual(sub.byteOffset, 0);
  assert.equal(readU32(sub, 2), 0xdeadbeef);
  assert.equal(readU64(sub, 8), 0x0102030405060708n);
});

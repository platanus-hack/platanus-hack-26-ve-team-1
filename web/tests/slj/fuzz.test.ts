import { test } from "node:test";
import assert from "node:assert/strict";
import { RESULT, fromB64u, toB64u, verifyPresentation } from "../../src/lib/slj/index.ts";
import { makeWorld, rng } from "./helpers/helpers.ts";

test("fuzz: ninguna mutación del QR válido da aprobado ni lanza (3000 casos, semilla fija)", async () => {
  const w = await makeWorld();
  const good = await w.present(await w.issue());
  const orig = fromB64u(good.slice(5))!;
  const ctx = w.ctx();
  assert.equal((await verifyPresentation(good, ctx)).code, RESULT.OK);

  const rand = rng(0x5eed);
  const int = (n: number) => Math.floor(rand() * n);
  let rejected = 0;
  for (let i = 0; i < 3000; i++) {
    let b = orig.slice();
    switch (int(5)) {
      case 0: // invertir bits
        for (let k = 0, n = 1 + int(3); k < n; k++) b[int(b.length)] ^= 1 << int(8);
        break;
      case 1: // truncar
        b = b.slice(0, int(b.length));
        break;
      case 2: { // insertar un byte
        const at = int(b.length + 1);
        b = Uint8Array.from([...b.slice(0, at), int(256), ...b.slice(at)]);
        break;
      }
      case 3: { // borrar un byte
        const at = int(b.length);
        b = Uint8Array.from([...b.slice(0, at), ...b.slice(at + 1)]);
        break;
      }
      default: { // intercambiar dos bytes distintos
        const x = int(b.length), y = int(b.length);
        [b[x], b[y]] = [b[y], b[x]];
      }
    }
    if (b.length === orig.length && b.every((v, j) => v === orig[j])) continue; // la mutación no cambió nada
    const r = await verifyPresentation("SLJ1:" + toB64u(b), ctx);
    assert.notEqual(r.code, RESULT.OK, `caso ${i} aprobó una presentación alterada`);
    rejected++;
  }
  assert.ok(rejected > 2900, `solo se probaron ${rejected} mutaciones`);
});

test("fuzz: cadenas aleatorias y unicode → formato inválido, sin lanzar", async () => {
  const w = await makeWorld();
  const rand = rng(42);
  const alphabets = ["ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_", "ñ€😀\0\n ", "+/=%"];
  for (let i = 0; i < 500; i++) {
    const a = alphabets[i % alphabets.length];
    const s = "SLJ1:" + Array.from({ length: Math.floor(rand() * 400) }, () => a[Math.floor(rand() * a.length)]).join("");
    assert.equal((await verifyPresentation(s, w.ctx())).code, RESULT.BAD_FORMAT);
  }
});

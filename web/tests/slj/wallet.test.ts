import { test } from "node:test";
import assert from "node:assert/strict";
import { ATTR, ISSUER, toHex } from "../../src/lib/slj/index.ts";
import { credentialsFromResponse, toWalletCredential, usable } from "../../src/lib/slj/wallet.ts";
import { matrixToPath } from "../../src/lib/slj/qrPath.ts";
import { makeWorld, NOW, rng } from "./helpers/helpers.ts";

test("credentialsFromResponse: lee el contrato de P1 y etiqueta por atributo y emisor", async () => {
  const w = await makeWorld();
  const a1 = await w.issue({ attr: ATTR.MAYOR18 });
  const a6 = await w.issue({ attr: ATTR.DONANTE });
  const carnet = await w.issue({ issuer: ISSUER.UNIVERSIDAD, attr: ATTR.IDENTIDAD, value: new TextEncoder().encode("ANA|1|2030-01-01") });
  const out = credentialsFromResponse({}) // no es lista
    .concat(credentialsFromResponse([
      { a: "A6", blob_hex: toHex(a6) },
      { a: "A3", issuer: 2, blob_hex: toHex(carnet), expires: 1 },
      { a: "A1", blob_hex: toHex(a1) },
    ]));
  assert.deepEqual(out.map((c) => c.label), ["Mayor de 18", "Carnet universitario", "Donante y licencia"], "ordenadas por atributo");
  assert.deepEqual(out.map((c) => c.issuer), [1, 2, 1]);
});

test("credentialsFromResponse: ignora entradas rotas sin tumbar el resto", async () => {
  const w = await makeWorld();
  const ok = toHex(await w.issue());
  const out = credentialsFromResponse([
    null, 7, {}, { blob_hex: 12 }, { blob_hex: "zz" }, { blob_hex: "00" }, { blob_hex: ok.slice(0, -2) }, { blob_hex: ok },
  ]);
  assert.equal(out.length, 1);
  assert.equal(credentialsFromResponse(null).length, 0);
  assert.equal(credentialsFromResponse("x").length, 0);
});

test("toWalletCredential expone vencimiento y handle del blob (fuente de verdad)", async () => {
  const w = await makeWorld();
  const c = toWalletCredential(await w.issue({ exp: NOW + 5, handle: 0xabcdn }))!;
  assert.equal(c.exp, NOW + 5);
  assert.equal(c.handle, 0xabcdn);
  assert.equal(toWalletCredential(new Uint8Array(10)), null);
});

test("usable: descarta las vencidas (exp < ahora) y conserva la que vence justo ahora", async () => {
  const w = await makeWorld();
  const mk = async (exp: number) => toWalletCredential(await w.issue({ exp }))!;
  const list = [await mk(NOW - 1), await mk(NOW), await mk(NOW + 1)];
  assert.deepEqual(usable(list, NOW).map((c) => c.exp), [NOW, NOW + 1]);
});

// ---- matrixToPath ----
function pathToMatrix(d: string, size: number, border: number): boolean[][] {
  const m = Array.from({ length: size - border * 2 }, () => Array<boolean>(size - border * 2).fill(false));
  for (const [, x, y, w] of d.matchAll(/M(\d+) (\d+)h(\d+)v1h-\d+z/g).map((r) => r.map(Number) as [number, number, number, number])) {
    for (let i = 0; i < w; i++) m[y - border][x - border + i] = true;
  }
  return m;
}

test("matrixToPath: el trazo reproduce exactamente la matriz (ida y vuelta, 50 matrices aleatorias)", () => {
  const rand = rng(7);
  for (let k = 0; k < 50; k++) {
    const n = 21 + 4 * Math.floor(rand() * 10);
    const m = Array.from({ length: n }, () => Array.from({ length: n }, () => rand() < 0.45));
    const { d, size } = matrixToPath(m, 4);
    assert.equal(size, n + 8);
    assert.deepEqual(pathToMatrix(d, size, 4), m);
  }
});

test("matrixToPath: casos borde y fusión de tramos (los QR son cuadrados)", () => {
  const rect = (m: boolean[][]) => matrixToPath(m, 0).d.split("M").length - 1;
  const F = false, T = true;
  assert.deepEqual(matrixToPath([[F]], 4), { d: "", size: 9 });
  assert.equal(matrixToPath([[T]], 2).size, 5);
  assert.equal(rect([[T, T, T], [F, F, F], [F, F, F]]), 1, "un tramo continuo = un rectángulo");
  assert.equal(rect([[T, F, T], [F, F, F], [F, F, F]]), 2, "dos tramos separados");
  assert.equal(rect([[T, T, F], [T, T, F], [F, F, F]]), 2, "una fila = un rectángulo, aunque se repita abajo");
});

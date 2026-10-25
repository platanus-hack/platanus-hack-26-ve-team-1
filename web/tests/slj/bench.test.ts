import { test } from "node:test";
import assert from "node:assert/strict";
import { verifyPresentation } from "../../src/lib/slj/index.ts";
import { makeWorld } from "./helpers/helpers.ts";

test("rendimiento: verificar un QR toma menos de 25 ms y el QR no pasa de 249 caracteres", async () => {
  const w = await makeWorld();
  const qr = await w.present(await w.issue());
  assert.ok(qr.length <= 249, `el QR mide ${qr.length} caracteres`);
  const ctx = w.ctx();
  for (let i = 0; i < 20; i++) await verifyPresentation(qr, ctx);
  const N = 200;
  const t0 = performance.now();
  for (let i = 0; i < N; i++) await verifyPresentation(qr, ctx);
  const ms = (performance.now() - t0) / N;
  console.log(`  verifyPresentation: ${ms.toFixed(2)} ms/op · QR de ${qr.length} caracteres`);
  assert.ok(ms < 25, `demasiado lento: ${ms.toFixed(2)} ms`);
});

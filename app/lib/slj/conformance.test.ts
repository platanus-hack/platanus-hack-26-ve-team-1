// Interoperabilidad con tools/slj (Go): los vectores los firma Go y los verifica WebCrypto;
// la presentación que escribe este test la verifica Go (`go test ./...` en tools/slj).
import { test } from "node:test";
import assert from "node:assert/strict";
import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { ISSUER, RESULT, fromHex, importIssuers, toHex, verifyPresentation, type VerifyContext } from "./index.ts";
import { makeWorld, NOW } from "./test/helpers.ts";

const dir = join(dirname(fileURLToPath(import.meta.url)), "testdata");

test("vectores firmados por Go dan el código esperado", { skip: !existsSync(join(dir, "vectors.json")) }, async () => {
  const v = JSON.parse(readFileSync(join(dir, "vectors.json"), "utf8"));
  const issuers = await importIssuers(Object.fromEntries(Object.entries(v.issuers).map(([k, h]) => [k, fromHex(h as string)])));
  assert.ok(v.cases.length >= 14);
  const seen = new Set<number>();
  for (const c of v.cases) {
    const ctx: VerifyContext = {
      issuers, now: c.now,
      revoked: new Set((c.revoked ?? []).map((h: string) => BigInt("0x" + h))),
      accept: c.accept ? new Set<number>(c.accept) : undefined,
    };
    assert.equal((await verifyPresentation(c.qr, ctx)).code, c.expect, `vector ${c.name}`);
    seen.add(c.expect);
  }
  // los vectores deben ejercitar todos los códigos de resultado de la verificación por QR
  for (const code of [RESULT.OK, RESULT.BAD_SIG, RESULT.REVOKED, RESULT.EXPIRED, RESULT.NO_PERMISSION, RESULT.STALE, RESULT.BAD_FORMAT]) {
    assert.ok(seen.has(code), `ningún vector cubre el código ${code}`);
  }
});

test("presentación de WebCrypto para que la verifique Go", async () => {
  const w = await makeWorld();
  const qr = await w.present(await w.issue(), NOW);
  const path = join(dir, "ts-presentation.json");
  // Las firmas son aleatorias: solo se reescribe el fixture si falta o con SLJ_REGEN=1 (lo hace el CI), para no ensuciar el árbol.
  if (!existsSync(path) || process.env.SLJ_REGEN === "1") {
    writeFileSync(path, JSON.stringify({ now: NOW, qr, issuers: { [ISSUER.NACIONAL]: toHex(w.nacional.pub) }, expect: RESULT.OK }, null, 2) + "\n");
  }
  assert.equal((await verifyPresentation(qr, w.ctx())).code, RESULT.OK);
});

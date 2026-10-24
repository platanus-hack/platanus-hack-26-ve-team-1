// Coherencia entre el código, los docs y el despliegue.
import { test } from "node:test";
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import vm from "node:vm";
import { RESULT, ROTATE_SECONDS, WINDOW_SECONDS } from "./index.ts";

const root = join(dirname(fileURLToPath(import.meta.url)), "../../..");
const read = (p: string) => readFileSync(join(root, p), "utf8");

test("render.yaml publica una carpeta que existe y tiene index.html", () => {
  const m = read("render.yaml").match(/staticPublishPath:\s*(\S+)/);
  assert.ok(m, "render.yaml debe declarar staticPublishPath");
  assert.ok(existsSync(join(root, m[1], "index.html")), `${m[1]}/index.html no existe`);
});

test("el script del laboratorio es JavaScript válido", () => {
  const html = read("app/public/p4-lab/index.html");
  const scripts = [...html.matchAll(/<script>([\s\S]*?)<\/script>/g)].map((x) => x[1]);
  assert.ok(scripts.length > 0);
  for (const s of scripts) assert.doesNotThrow(() => new vm.Script(s));
});

test("docs/p4-qr.md lista los mismos códigos de resultado y tiempos que el código", () => {
  const doc = read("docs/p4-qr.md");
  const line = doc.split("\n").find((l) => l.includes("aprobado") && l.includes("firma inválida"))!;
  assert.ok(line, "falta la línea de códigos de resultado");
  const codes = [...line.matchAll(/\b0([1-9])\b/g)].map((m) => Number(m[1]));
  for (const c of Object.values(RESULT)) assert.ok(codes.includes(c), `el doc no menciona el código ${c}`);
  assert.ok(doc.includes(`**${ROTATE_SECONDS} s**`), "tiempo de rotación desactualizado en el doc");
  assert.ok(doc.includes(`${WINDOW_SECONDS} s`), "ventana desactualizada en el doc");
});

test("el núcleo no tiene dependencias externas", () => {
  for (const f of ["bytes", "codec", "p256", "present", "verify", "store", "tables", "index"]) {
    for (const m of read(`app/lib/slj/${f}.ts`).matchAll(/from\s+"([^"]+)"/g)) {
      assert.ok(m[1].startsWith("./"), `${f}.ts importa "${m[1]}"`);
    }
  }
});

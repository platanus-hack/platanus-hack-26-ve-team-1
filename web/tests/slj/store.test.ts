import { test, beforeEach } from "node:test";
import assert from "node:assert/strict";
import { createFakeIDB } from "./helpers/fake-idb.ts";
import { exportCompressed, generateDeviceKey } from "../../src/lib/slj/index.ts";
import * as store from "../../src/lib/slj/store.ts";

const fake = createFakeIDB();
(globalThis as any).indexedDB = fake.indexedDB;
beforeEach(() => fake.clear());

test("kv: get/set/del", async () => {
  assert.equal(await store.kvGet("a"), undefined);
  await store.kvSet("a", { n: 1 });
  assert.deepEqual(await store.kvGet("a"), { n: 1 });
  await store.kvDel("a");
  assert.equal(await store.kvGet("a"), undefined);
});

test("la llave del dispositivo se genera una vez y se reutiliza", async () => {
  let generated = 0;
  const gen = async () => {
    generated++;
    return generateDeviceKey();
  };
  const first = await store.getDeviceKeyPair(gen);
  const second = await store.getDeviceKeyPair(gen);
  assert.equal(generated, 1);
  assert.deepEqual(await exportCompressed(second.publicKey), await exportCompressed(first.publicKey));
});

test("revocaciones: se guardan y se leen como un solo blob binario", async () => {
  assert.equal(await store.loadRevocations(), undefined);
  const blob = new Uint8Array(16).map((_, i) => i);
  await store.saveRevocations(blob);
  assert.deepEqual(await store.loadRevocations(), blob);
});

test("outbox: 50 eventos empujados a la vez no se pierden", async () => {
  await Promise.all(Array.from({ length: 50 }, (_, i) => store.pushOutbox({ i })));
  const all = (await store.takeOutbox()) as { i: number }[];
  assert.equal(all.length, 50);
  assert.deepEqual(new Set(all.map((e) => e.i)).size, 50);
});

test("outbox: drainOutbox devuelve todo y lo vacía", async () => {
  await store.pushOutbox({ i: 1 });
  await store.pushOutbox({ i: 2 });
  assert.deepEqual(await store.drainOutbox(), [{ i: 1 }, { i: 2 }]);
  assert.deepEqual(await store.drainOutbox(), []);
});

test("outbox: eventos que llegan mientras se vacía no se pierden ni se duplican", async () => {
  for (let i = 0; i < 10; i++) await store.pushOutbox({ i });
  const pushes = Array.from({ length: 10 }, (_, k) => store.pushOutbox({ i: 100 + k }));
  const [drained] = await Promise.all([store.drainOutbox(), ...pushes]);
  const rest = await store.drainOutbox();
  const ids = [...drained, ...rest].map((e: any) => e.i).sort((a, b) => a - b);
  assert.deepEqual(ids, [...Array(10).keys(), ...Array.from({ length: 10 }, (_, k) => 100 + k)]);
});

test("una sola conexión a IndexedDB sin importar cuántas operaciones", async () => {
  const before = fake.opens();
  for (let i = 0; i < 20; i++) await store.kvSet("k" + i, i);
  await Promise.all(Array.from({ length: 20 }, (_, i) => store.kvGet("k" + i)));
  assert.ok(fake.opens() - before <= 1, `se abrieron ${fake.opens() - before} conexiones`);
});

test("requestPersist: sin navigator.storage devuelve false sin lanzar", async () => {
  assert.equal(await store.requestPersist(), false);
});

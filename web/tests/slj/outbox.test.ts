import { test, beforeEach } from "node:test";
import assert from "node:assert/strict";
import { createFakeIDB } from "./helpers/fake-idb.ts";
import { flushOutbox, type SendResult } from "../../src/lib/slj/outbox.ts";
import * as store from "../../src/lib/slj/store.ts";

const fake = createFakeIDB();
(globalThis as any).indexedDB = fake.indexedDB;
beforeEach(() => fake.clear());

const fill = async (n: number) => {
  for (let i = 0; i < n; i++) await store.pushOutbox({ i });
};

test("todo se envía y el outbox queda vacío", async () => {
  await fill(5);
  const sent: unknown[] = [];
  const r = await flushOutbox(async (e) => (sent.push(e), "sent"));
  assert.deepEqual(r, { sent: 5, dropped: 0, pending: 0 });
  assert.deepEqual(sent, [0, 1, 2, 3, 4].map((i) => ({ i })), "se envía en orden");
  assert.deepEqual(await store.takeOutbox(), []);
});

test("un fallo transitorio devuelve ese evento y los siguientes, sin perder ni duplicar", async () => {
  await fill(6);
  let calls = 0;
  const r = await flushOutbox(async () => (++calls <= 2 ? "sent" : "retry"));
  assert.deepEqual(r, { sent: 2, dropped: 0, pending: 4 });
  assert.equal(calls, 3, "se detiene en el primer fallo");
  assert.deepEqual(await store.takeOutbox(), [2, 3, 4, 5].map((i) => ({ i })));
});

test("un evento que la API rechaza (drop) se descarta y la cola sigue", async () => {
  await fill(4);
  const r = await flushOutbox(async (e) => ((e as { i: number }).i === 1 ? "drop" : "sent"));
  assert.deepEqual(r, { sent: 3, dropped: 1, pending: 0 });
  assert.deepEqual(await store.takeOutbox(), []);
});

test("si el envío lanza una excepción cuenta como reintento", async () => {
  await fill(3);
  const r = await flushOutbox(async () => {
    throw new Error("sin red");
  });
  assert.deepEqual(r, { sent: 0, dropped: 0, pending: 3 });
  assert.equal((await store.takeOutbox()).length, 3);
});

test("eventos que llegan mientras se envía no se pierden", async () => {
  await fill(3);
  const pending: Promise<void>[] = [];
  const r = await flushOutbox(async (e) => {
    if ((e as { i: number }).i === 0) pending.push(store.pushOutbox({ i: 99 }));
    return "sent" as SendResult;
  });
  await Promise.all(pending);
  assert.equal(r.sent, 3);
  assert.deepEqual(await store.takeOutbox(), [{ i: 99 }]);
});

test("un outbox vacío no hace nada", async () => {
  assert.deepEqual(await flushOutbox(async () => "sent"), { sent: 0, dropped: 0, pending: 0 });
});

test("el outbox conserva solo los más recientes al llegar al tope", async () => {
  await Promise.all(Array.from({ length: store.OUTBOX_MAX + 20 }, (_, i) => store.pushOutbox({ i })));
  const all = (await store.takeOutbox()) as { i: number }[];
  assert.equal(all.length, store.OUTBOX_MAX);
  assert.equal(all[all.length - 1].i, store.OUTBOX_MAX + 19);
  assert.equal(all[0].i, 20, "se descartan los más viejos");
});

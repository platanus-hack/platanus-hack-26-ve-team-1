// El service worker no se puede probar en un navegador sin red real, así que se ejecuta aquí dentro de un
// entorno simulado (Cache API, fetch y eventos) y se comprueban sus reglas, sobre todo las de seguridad.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import vm from "node:vm";

const source = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "../../public/sw-p4.js"), "utf8");
const ORIGIN = "https://sisa.test";

type Handler = (event: any) => void;

function createWorker() {
  const handlers: Record<string, Handler> = {};
  const stores = new Map<string, Map<string, Response>>();
  const state = { online: true, skipWaiting: 0, claimed: 0, fetched: [] as string[] };
  const keyOf = (r: string | Request) => new URL(typeof r === "string" ? r : r.url, ORIGIN).href;

  const cacheApi = {
    async open(name: string) {
      if (!stores.has(name)) stores.set(name, new Map());
      const store = stores.get(name)!;
      return {
        async match(r: string | Request) { return store.get(keyOf(r))?.clone(); },
        async put(r: string | Request, res: Response) { store.set(keyOf(r), res); },
        async add(r: string | Request) { const res = await sandbox.fetch(r); if (!res.ok) throw new Error("no ok"); store.set(keyOf(r), res); },
      };
    },
    async match(r: string | Request) { for (const s of stores.values()) { const hit = s.get(keyOf(r)); if (hit) return hit.clone(); } return undefined; },
    async keys() { return [...stores.keys()]; },
    async delete(name: string) { return stores.delete(name); },
  };

  const sandbox: any = {
    URL, Response, Request, Promise,
    caches: cacheApi,
    fetch: async (r: string | Request) => {
      const url = keyOf(r);
      state.fetched.push(url);
      if (!state.online) throw new TypeError("sin red");
      const path = new URL(url).pathname;
      const body = path === "/index.html" || path === "/verificar" ? "<html>app</html>" : `contenido de ${path}`;
      return new Response(body, { status: 200, headers: { "content-type": path.endsWith(".js") ? "text/javascript" : "text/html" } });
    },
    self: {
      location: { origin: ORIGIN },
      addEventListener: (type: string, fn: Handler) => { handlers[type] = fn; },
      skipWaiting: () => { state.skipWaiting++; },
      clients: { claim: async () => { state.claimed++; } },
    },
  };
  vm.runInNewContext(source, sandbox);

  /** Despacha un evento `fetch` y devuelve la respuesta, o null si el worker no la intercepta. */
  async function dispatchFetch(url: string, init: { method?: string; mode?: string } = {}) {
    let responded: Promise<Response> | null = null;
    // El Request real de Node no deja fijar `mode`; al worker solo le hacen falta estos tres campos.
    const request = { url, method: init.method ?? "GET", mode: init.mode ?? "no-cors" };
    handlers.fetch({ request, respondWith: (p: Promise<Response>) => { responded = p; } });
    return responded ? await responded : null;
  }
  async function wait(fn: (event: any) => void) {
    const pending: Promise<unknown>[] = [];
    fn({ waitUntil: (p: Promise<unknown>) => pending.push(p) });
    await Promise.all(pending);
  }
  return { handlers, stores, state, dispatchFetch, wait, sandbox };
}

test("instala con skipWaiting y al activar toma el control y borra cachés viejos sin tocar otros", async () => {
  const w = createWorker();
  await w.wait((e) => w.handlers.install(e));
  assert.equal(w.state.skipWaiting, 1);
  w.stores.set("sisa-p4-v0", new Map());
  w.stores.set("otra-app", new Map());
  await w.wait((e) => w.handlers.activate(e));
  assert.equal(w.state.claimed, 1);
  assert.ok(!w.stores.has("sisa-p4-v0"), "borra su caché de una versión anterior");
  assert.ok(w.stores.has("otra-app"), "no toca cachés ajenos");
});

test("SEGURIDAD: nunca intercepta /api/*, ni métodos que no son GET, ni otros orígenes", async () => {
  const w = createWorker();
  assert.equal(await w.dispatchFetch(`${ORIGIN}/api/wallet?public_key=02aa`), null);
  assert.equal(await w.dispatchFetch(`${ORIGIN}/api/events`, { method: "POST" }), null);
  assert.equal(await w.dispatchFetch(`${ORIGIN}/assets/app.js`, { method: "POST" }), null);
  assert.equal(await w.dispatchFetch("https://otro.example/assets/app.js"), null);
  assert.equal(w.state.fetched.length, 0, "no hizo ninguna petición por su cuenta");
  assert.equal([...w.stores.values()].reduce((n, s) => n + s.size, 0), 0, "no guardó nada");
});

test("navegación: con red devuelve la página y la guarda; sin red sirve la última guardada", async () => {
  const w = createWorker();
  const online = await w.dispatchFetch(`${ORIGIN}/verificar`, { mode: "navigate" });
  assert.equal(await online!.text(), "<html>app</html>");
  w.state.online = false;
  const offline = await w.dispatchFetch(`${ORIGIN}/verificar`, { mode: "navigate" });
  assert.equal(await offline!.text(), "<html>app</html>", "abre sin red");
});

test("navegación sin red y sin nada guardado: error de red, no una página inventada", async () => {
  const w = createWorker();
  w.state.online = false;
  const res = await w.dispatchFetch(`${ORIGIN}/verificar`, { mode: "navigate" });
  assert.equal(res!.type, "error");
});

test("navegación: solo se guarda HTML, no cualquier respuesta", async () => {
  const w = createWorker();
  w.sandbox.fetch = async () => new Response("{}", { status: 200, headers: { "content-type": "application/json" } });
  await w.dispatchFetch(`${ORIGIN}/verificar`, { mode: "navigate" });
  assert.equal([...w.stores.values()].reduce((n, s) => n + s.size, 0), 0);
});

test("/assets/* (con hash en el nombre): primero el caché, la red solo la primera vez", async () => {
  const w = createWorker();
  const url = `${ORIGIN}/assets/index-abc123.js`;
  assert.equal(await (await w.dispatchFetch(url))!.text(), "contenido de /assets/index-abc123.js");
  w.state.online = false;
  const before = w.state.fetched.length;
  assert.equal(await (await w.dispatchFetch(url))!.text(), "contenido de /assets/index-abc123.js", "sirve sin red");
  assert.equal(w.state.fetched.length, before, "no volvió a pedirlo");
});

test("cache-urls: guarda lo que la página ya cargó y el shell, y descarta /api/ y otros orígenes", async () => {
  const w = createWorker();
  await w.wait((e) =>
    w.handlers.message({
      ...e,
      data: { type: "cache-urls", urls: ["/assets/a.js", "/assets/b.css", "/api/me", `${ORIGIN}/api/wallet`, "https://otro.example/x.js", 42] },
    }),
  );
  const cached = [...(w.stores.get("sisa-p4-v1")?.keys() ?? [])].map((k) => new URL(k).pathname).sort();
  assert.deepEqual(cached, ["/assets/a.js", "/assets/b.css", "/index.html"]);
});

test("cache-urls: ignora mensajes que no son suyos o están mal formados", async () => {
  const w = createWorker();
  for (const data of [null, {}, { type: "otra-cosa" }, { type: "cache-urls", urls: "no es lista" }]) {
    await w.wait((e) => w.handlers.message({ ...e, data }));
  }
  assert.equal([...w.stores.values()].reduce((n, s) => n + s.size, 0), 0);
});

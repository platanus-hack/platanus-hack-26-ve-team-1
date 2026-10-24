// Envoltorio mínimo de IndexedDB para la billetera y el verificador (solo navegador).

const DB = "slj";
const STORE = "kv";

// Una sola conexión por pestaña: abrir la base en cada operación es lento y nunca se cerraba.
let conn: Promise<IDBDatabase> | null = null;

function open(): Promise<IDBDatabase> {
  return (conn ??= new Promise((res, rej) => {
    const r = indexedDB.open(DB, 1);
    r.onupgradeneeded = () => r.result.createObjectStore(STORE);
    r.onsuccess = () => {
      const db = r.result;
      db.onversionchange = () => {
        db.close();
        conn = null;
      };
      res(db);
    };
    r.onerror = () => {
      conn = null;
      rej(r.error);
    };
  }));
}

async function tx<T>(mode: IDBTransactionMode, f: (s: IDBObjectStore) => IDBRequest<T>): Promise<T> {
  const db = await open();
  return new Promise((res, rej) => {
    const r = f(db.transaction(STORE, mode).objectStore(STORE));
    r.onsuccess = () => res(r.result);
    r.onerror = () => rej(r.error);
  });
}

export const kvGet = <T = unknown>(k: string) => tx<T | undefined>("readonly", (s) => s.get(k));
export const kvSet = (k: string, v: unknown) => tx("readwrite", (s) => s.put(v, k));
export const kvDel = (k: string) => tx("readwrite", (s) => s.delete(k));

/** Pide almacenamiento persistente (evita que el navegador borre la llave). */
export async function requestPersist(): Promise<boolean> {
  try {
    return (await navigator.storage?.persist?.()) ?? false;
  } catch {
    return false;
  }
}

/** Llave P-256 no extraíble del dispositivo; se crea al primer uso. */
export async function getDeviceKeyPair(generate: () => Promise<CryptoKeyPair>): Promise<CryptoKeyPair> {
  const cur = await kvGet<CryptoKeyPair>("devkey");
  if (cur) return cur;
  const kp = await generate();
  await kvSet("devkey", kp);
  return kp;
}

export const saveRevocations = (b: Uint8Array) => kvSet("revocations", b);
export const loadRevocations = () => kvGet<Uint8Array>("revocations");

/** Lee y modifica una clave dentro de UNA transacción de escritura: no hay carreras entre pestañas ni escaneos. */
function update<T>(key: string, f: (cur: T | undefined) => { next: T | undefined; out: unknown }): Promise<any> {
  return open().then(
    (db) =>
      new Promise((res, rej) => {
        const t = db.transaction(STORE, "readwrite");
        const s = t.objectStore(STORE);
        let out: unknown;
        const g = s.get(key);
        g.onsuccess = () => {
          const r = f(g.result as T | undefined);
          out = r.out;
          if (r.next === undefined) s.delete(key);
          else s.put(r.next, key);
        };
        g.onerror = () => rej(g.error);
        t.oncomplete = () => res(out);
        t.onerror = () => rej(t.error);
        t.onabort = () => rej(t.error);
      }),
  );
}

/** Tope del outbox: sin red durante mucho tiempo se conservan los más recientes, no se llena el almacenamiento. */
export const OUTBOX_MAX = 500;

export const pushOutbox = (ev: unknown): Promise<void> =>
  update<unknown[]>("outbox", (cur) => ({ next: [...(cur ?? []), ev].slice(-OUTBOX_MAX), out: undefined }));

/** Vacía el outbox de forma atómica: devuelve lo que había y lo borra, sin perder eventos concurrentes. */
export const drainOutbox = (): Promise<unknown[]> =>
  update<unknown[]>("outbox", (cur) => ({ next: undefined, out: cur ?? [] }));

/** Solo lectura (p. ej. para mostrar cuántos eventos faltan por enviar). */
export const takeOutbox = async () => ((await kvGet<unknown[]>("outbox")) ?? []) as unknown[];
export const clearOutbox = () => kvDel("outbox");

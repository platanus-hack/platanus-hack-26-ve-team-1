// Envoltorio mínimo de IndexedDB para la billetera y el verificador (solo navegador).

const DB = "slj";
const STORE = "kv";

function open(): Promise<IDBDatabase> {
  return new Promise((res, rej) => {
    const r = indexedDB.open(DB, 1);
    r.onupgradeneeded = () => r.result.createObjectStore(STORE);
    r.onsuccess = () => res(r.result);
    r.onerror = () => rej(r.error);
  });
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

export async function pushOutbox(ev: unknown): Promise<void> {
  const q = (await kvGet<unknown[]>("outbox")) ?? [];
  q.push(ev);
  await kvSet("outbox", q);
}
export const takeOutbox = async () => ((await kvGet<unknown[]>("outbox")) ?? []) as unknown[];
export const clearOutbox = () => kvDel("outbox");

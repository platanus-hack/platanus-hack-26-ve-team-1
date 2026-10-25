// IndexedDB mínimo en memoria para probar store.ts en Node, sin dependencias.
// Modela lo que importa: cada petición se resuelve de forma asíncrona, las
// transacciones de escritura se serializan y una transacción termina cuando
// no le quedan peticiones pendientes (por eso un get+put dentro de una misma
// transacción es atómico y dos transacciones separadas no lo son).

export interface FakeIDB {
  indexedDB: any;
  opens: () => number;
  clear: () => void;
}

function clone<T>(v: T): T {
  try {
    return structuredClone(v);
  } catch {
    return v; // CryptoKey no siempre es clonable en Node; se guarda la referencia
  }
}

export function createFakeIDB(): FakeIDB {
  const data = new Map<string, Map<string, unknown>>();
  let opens = 0;
  let rwTail: Promise<void> = Promise.resolve();

  function makeDb() {
    return {
      createObjectStore(name: string) {
        if (!data.has(name)) data.set(name, new Map());
      },
      close() {},
      transaction(name: string, mode: IDBTransactionMode = "readonly") {
        const store = data.get(name);
        if (!store) throw new Error("almacén inexistente: " + name);
        let pending = 0;
        let release!: () => void;
        const finished = new Promise<void>((r) => (release = r));
        // las transacciones de escritura esperan a la anterior; las de lectura a las escrituras ya creadas
        const ready = rwTail;
        if (mode === "readwrite") rwTail = finished;

        const settle = () =>
          setTimeout(() => {
            if (pending === 0) release();
          }, 0);

        const request = <T>(run: () => T) => {
          const req: any = { result: undefined, error: null };
          pending++;
          ready.then(() =>
            setTimeout(() => {
              try {
                req.result = run();
                req.onsuccess?.({ target: req });
              } catch (e) {
                req.error = e;
                req.onerror?.({ target: req });
              }
              pending--;
              settle();
            }, 0),
          );
          return req;
        };

        const txObj: any = {
          oncomplete: null,
          objectStore() {
            return {
              get: (k: string) => request(() => clone(store.get(k))),
              put: (v: unknown, k: string) =>
                request(() => {
                  if (mode === "readonly") throw new Error("transacción de solo lectura");
                  store.set(k, clone(v));
                  return k;
                }),
              delete: (k: string) =>
                request(() => {
                  if (mode === "readonly") throw new Error("transacción de solo lectura");
                  store.delete(k);
                  return undefined;
                }),
            };
          },
        };
        finished.then(() => txObj.oncomplete?.());
        return txObj;
      },
    };
  }

  let created = false;
  const indexedDB = {
    open(_name: string) {
      opens++;
      const req: any = {};
      setTimeout(() => {
        const db = makeDb();
        req.result = db;
        if (!created) {
          created = true;
          req.onupgradeneeded?.({ target: req });
        }
        req.onsuccess?.({ target: req });
      }, 0);
      return req;
    },
  };

  return {
    indexedDB,
    opens: () => opens,
    clear: () => {
      for (const s of data.values()) s.clear();
    },
  };
}

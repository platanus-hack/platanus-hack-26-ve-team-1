// Verifica con node:crypto los vectores y las tarjetas/credenciales de prueba.
// Uso: node tools/verify.mjs   (Node 20, sin dependencias)
import { createPublicKey, verify } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const load = (p) => JSON.parse(readFileSync(join(root, p), 'utf8'));
const hex = (s) => Buffer.from(s, 'hex');

const issuersJwk = load('keys/issuers.json');
const names = { nacional: '0x01', universidad: '0x02' };
const pubOf = (jwk) => createPublicKey({ key: jwk, format: 'jwk' });
const check = (jwk, msg, sig) =>
  verify('sha256', msg, { key: pubOf(jwk), dsaEncoding: 'ieee-p1363' }, sig);

let fails = 0;
const report = (name, got, want) => {
  const ok = got === want;
  if (!ok) fails++;
  console.log(`${ok ? 'OK  ' : 'FAIL'} ${name} (obtuvo ${got}, esperaba ${want})`);
};

// 1. test-vectors.json
for (const v of load('docs/test-vectors.json').vectors) {
  report(`vector ${v.name}`, check(issuersJwk[names[v.issuer]], hex(v.message_hex), hex(v.sig_hex)), v.expect_valid);
}

// 2. tarjetas-prueba.json: reensambla sectores 2.. y verifica UID‖cabecera‖valor
const maxVal = (code) => (code === 3 ? 64 : 16);
for (const [name, c] of Object.entries(load('docs/tarjetas-prueba.json'))) {
  const uid = hex(c.uid);
  const index = hex(c.sectors['1'].join(''));
  const rec = Buffer.concat(
    Object.keys(c.sectors).map(Number).filter((n) => n >= 2).sort((a, b) => a - b)
      .map((n) => hex(c.sectors[n].join(''))));
  const [ver, issuer, code, len] = rec;
  const expires = rec.readUInt32BE(4);
  const handle = rec.subarray(8, 16);
  const max = maxVal(code);
  const value = rec.subarray(16, 16 + len);
  const sig = rec.subarray(16 + max, 16 + max + 64);
  const msg = Buffer.concat([uid, rec.subarray(0, 16), value]);
  const jwk = issuersJwk['0x' + issuer.toString(16).padStart(2, '0')];
  report(`${name}: versión e índice`, ver === 1 && index[0] === 1 && index.subarray(1, 9).equals(handle), true);
  report(`${name}: firma`, check(jwk, msg, sig), c.expect.signature_valid);
  report(`${name}: vencida`, expires < Date.now() / 1000, c.expect.expired);
}

// 3. credenciales-billetera-prueba.json: cabecera‖valor‖pub33 firmado, sin UID
const w = load('docs/credenciales-billetera-prueba.json');
for (const c of w.credentials) {
  const blob = hex(c.blob_hex);
  const len = blob[3];
  const msg = blob.subarray(0, 16 + len + 33);
  const sig = blob.subarray(16 + len + 33);
  report(`billetera A${c.attribute}: pub del celular en el blob`, msg.subarray(16 + len).equals(hex(w.phone_pub_hex)), true);
  report(`billetera A${c.attribute}: firma`, check(issuersJwk[names[c.issuer]], msg, sig), true);
  const bad = Buffer.from(msg); bad[16] ^= 1;
  report(`billetera A${c.attribute}: valor alterado`, check(issuersJwk[names[c.issuer]], bad, sig), false);
}

console.log(fails ? `\n${fails} fallo(s)` : '\nTodo correcto');
process.exit(fails ? 1 : 0);

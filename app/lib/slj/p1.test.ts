// Conformidad con el contrato REAL de P1 (rama p1-backend): sus credenciales de billetera de prueba, sus claves
// públicas de emisor (JWK) y la clave privada de celular de prueba que ellos publican. Los fixtures están en
// testdata/p1-*.json (copiados de docs/credenciales-billetera-prueba.json y keys/issuers.json de P1).
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import {
  ATTR, ISSUER, RESULT, createPresentation, fromB64u, fromHex, importIssuersJwk, parseCredential, toB64u,
  toVerifEvent, verifyPresentation,
} from "./index.ts";

const dir = join(dirname(fileURLToPath(import.meta.url)), "testdata");
const load = (f: string) => JSON.parse(readFileSync(join(dir, f), "utf8"));
const fx = load("p1-credenciales-billetera.json");
const jwks = load("p1-issuers.json");
const NOW = 1_800_000_000;

const issuers = await importIssuersJwk(jwks);
const { d, x, y, kty, crv } = fx.phone_private_jwk;
const phoneKey = await crypto.subtle.importKey("jwk", { kty, crv, x, y, d }, { name: "ECDSA", namedCurve: "P-256" }, false, ["sign"]);
const ctx = (extra = {}) => ({ issuers, revoked: new Set<bigint>(), now: NOW, ...extra });

test("las claves de emisor de P1 (JWK) se importan: 0x01 y 0x02", () => {
  assert.deepEqual([...issuers.keys()].sort(), [ISSUER.NACIONAL, ISSUER.UNIVERSIDAD]);
});

test("cada blob de P1 se parsea y trae la clave del celular de P1", () => {
  for (const c of fx.credentials) {
    const cred = parseCredential(fromHex(c.blob_hex))!;
    assert.ok(cred, `blob A${c.attribute}`);
    assert.equal(cred.attr, c.attribute);
    assert.equal(cred.issuer, c.issuer === "nacional" ? ISSUER.NACIONAL : ISSUER.UNIVERSIDAD);
    assert.equal(cred.handle, BigInt("0x" + fx.handle_hex));
    assert.deepEqual(cred.devpub, fromHex(fx.phone_pub_hex));
    assert.deepEqual(cred.sig, fromHex(c.sig_hex));
    assert.deepEqual(cred.signed, fromHex(c.message_hex));
  }
});

test("un QR hecho con una credencial real de P1 y su celular de prueba → aprobado (01)", async () => {
  for (const c of fx.credentials) {
    const qr = await createPresentation(fromHex(c.blob_hex), phoneKey, NOW);
    const r = await verifyPresentation(qr, ctx());
    assert.equal(r.code, RESULT.OK, `A${c.attribute}`);
    assert.equal(r.attr, c.attribute);
    assert.equal(r.handle, BigInt("0x" + fx.handle_hex));
  }
});

test("credencial de P1 con el valor alterado → firma inválida (02)", async () => {
  const blob = fromHex(fx.credentials[0].blob_hex).slice();
  blob[16] ^= 1; // el byte de valor
  assert.equal((await verifyPresentation(await createPresentation(blob, phoneKey, NOW), ctx())).code, RESULT.BAD_SIG);
});

test("la clave de otro emisor no valida la credencial de P1 → 02", async () => {
  const swapped = await importIssuersJwk({ "0x01": jwks["0x02"], "0x02": jwks["0x01"] });
  const qr = await createPresentation(fromHex(fx.credentials[0].blob_hex), phoneKey, NOW);
  assert.equal((await verifyPresentation(qr, ctx({ issuers: swapped }))).code, RESULT.BAD_SIG);
});

test("la credencial de P1 en otro celular no sirve: la firma del celular no coincide → 02", async () => {
  const other = (await crypto.subtle.generateKey({ name: "ECDSA", namedCurve: "P-256" }, false, ["sign"])) as CryptoKeyPair;
  const qr = await createPresentation(fromHex(fx.credentials[0].blob_hex), other.privateKey, NOW);
  assert.equal((await verifyPresentation(qr, ctx())).code, RESULT.BAD_SIG);
});

test("revocada, vencida y sin permiso con credenciales reales de P1", async () => {
  const blob = fromHex(fx.credentials[0].blob_hex);
  const handle = BigInt("0x" + fx.handle_hex);
  const qr = await createPresentation(blob, phoneKey, NOW);
  assert.equal((await verifyPresentation(qr, ctx({ revoked: new Set([handle]) }))).code, RESULT.REVOKED);
  assert.equal((await verifyPresentation(qr, ctx({ accept: new Set([ATTR.MAYOR21]) }))).code, RESULT.NO_PERMISSION);
  const exp = parseCredential(blob)!.exp;
  const late = await createPresentation(blob, phoneKey, exp + 1);
  assert.equal((await verifyPresentation(late, ctx({ now: exp + 1 }))).code, RESULT.EXPIRED);
});

test("el evento que se enviaría a la API cumple el contrato de P1", async () => {
  const qr = await createPresentation(fromHex(fx.credentials[0].blob_hex), phoneKey, NOW);
  const r = await verifyPresentation(qr, ctx());
  assert.deepEqual(toVerifEvent(r, NOW), { t: "verif", h: fx.handle_hex, a: "A1", res: "01", ts: NOW, emg: false });
});

test("QR de P1 con el byte de atributo final cambiado → formato inválido (08)", async () => {
  const raw = fromB64u((await createPresentation(fromHex(fx.credentials[0].blob_hex), phoneKey, NOW)).slice(5))!;
  raw[raw.length - 1] ^= 1;
  assert.equal((await verifyPresentation("SLJ1:" + toB64u(raw), ctx())).code, RESULT.BAD_FORMAT);
});

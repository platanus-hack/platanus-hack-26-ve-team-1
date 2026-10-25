package sign

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
)

func newKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func sample() Attribute {
	return Attribute{Issuer: IssuerNacional, Code: AttrMayor18, Expires: 1900000000,
		Handle: [8]byte{1, 2, 3, 4, 5, 6, 7, 8}, Value: []byte{0x01}}
}

var uid = []byte{0xA1, 0xB2, 0xC3, 0xD4}

func TestSignVerify(t *testing.T) {
	k := newKey(t)
	a := sample()
	sig, err := a.Sign(k, uid)
	if err != nil || len(sig) != 64 {
		t.Fatalf("sign: %v len=%d", err, len(sig))
	}
	if !a.Verify(&k.PublicKey, uid, sig) {
		t.Fatal("firma válida rechazada")
	}
}

func TestTamperedValueFails(t *testing.T) {
	k := newKey(t)
	a := sample()
	sig, _ := a.Sign(k, uid)
	a.Value[0] ^= 0x01
	if a.Verify(&k.PublicKey, uid, sig) {
		t.Fatal("valor alterado aceptado")
	}
}

func TestOtherUIDFails(t *testing.T) {
	k := newKey(t)
	a := sample()
	sig, _ := a.Sign(k, uid)
	if a.Verify(&k.PublicKey, []byte{0xA1, 0xB2, 0xC3, 0xD5}, sig) {
		t.Fatal("otro UID aceptado")
	}
}

func TestHeaderLayout(t *testing.T) {
	h := sample().Header()
	want := []byte{1, 1, 1, 1, 0x71, 0x3F, 0xB3, 0x00, 1, 2, 3, 4, 5, 6, 7, 8}
	if string(h) != string(want) {
		t.Fatalf("cabecera %x, esperada %x", h, want)
	}
}

func TestRecordSizesAndRoundTrip(t *testing.T) {
	k := newKey(t)
	for _, c := range []struct {
		code   byte
		val    []byte
		blocks int
	}{
		{AttrMayor18, []byte{1}, 6},
		{AttrSangre, []byte("O+ penicilina"), 6},
		{AttrIdentidad, []byte("ANA PEREZ V-12345678"), 9},
	} {
		a := sample()
		a.Code, a.Value = c.code, c.val
		sig, _ := a.Sign(k, uid)
		blocks, err := a.Record(sig)
		if err != nil || len(blocks) != c.blocks {
			t.Fatalf("code %d: %v, %d bloques", c.code, err, len(blocks))
		}
		if _, err := SectorBlocks(c.code, blocks); err != nil {
			t.Fatal(err)
		}
		var rec []byte
		for _, b := range blocks {
			rec = append(rec, b...)
		}
		b, s, err := ParseRecord(rec)
		if err != nil || !b.Verify(&k.PublicKey, uid, s) {
			t.Fatalf("code %d: round trip falló: %v", c.code, err)
		}
	}
}

func TestValueTooLong(t *testing.T) {
	a := sample()
	a.Value = make([]byte, 17)
	if _, err := a.Message(uid); err == nil {
		t.Fatal("valor de 17 B aceptado")
	}
}

func TestWallet(t *testing.T) {
	k, phone := newKey(t), newKey(t)
	pub := CompressPub(&phone.PublicKey)
	a := sample()
	sig, err := a.SignWallet(k, pub)
	if err != nil {
		t.Fatal(err)
	}
	if !a.VerifyWallet(&k.PublicKey, pub, sig) {
		t.Fatal("credencial válida rechazada")
	}
	other := CompressPub(&newKey(t).PublicKey)
	if a.VerifyWallet(&k.PublicKey, other, sig) {
		t.Fatal("otra clave de celular aceptada")
	}
	a2 := sample()
	a2.Value[0] ^= 1
	if a2.VerifyWallet(&k.PublicKey, pub, sig) {
		t.Fatal("valor alterado aceptado")
	}
	blob, _ := a.WalletBlob(pub, sig)
	b, p, s, err := ParseWalletBlob(blob)
	if err != nil || !b.VerifyWallet(&k.PublicKey, p, s) {
		t.Fatalf("blob: %v", err)
	}
}

func TestCompressRoundTrip(t *testing.T) {
	k := newKey(t)
	pub, err := DecompressPub(CompressPub(&k.PublicKey))
	if err != nil || !pub.Equal(&k.PublicKey) {
		t.Fatal("compresión no es reversible")
	}
}

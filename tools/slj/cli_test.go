package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Patrón "helper process": el binario de test se vuelve a ejecutar como CLI, porque verify llama a os.Exit.
func TestMain(m *testing.M) {
	if os.Getenv("SLJ_AS_CLI") == "1" {
		os.Args = append([]string{"slj"}, strings.Split(os.Getenv("SLJ_ARGS"), "\x1f")...)
		main()
		return
	}
	os.Exit(m.Run())
}

func runCLI(t *testing.T, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "SLJ_AS_CLI=1", "SLJ_ARGS="+strings.Join(args, "\x1f"))
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return out.String(), code
}

func firstLine(s string) string { return strings.SplitN(strings.TrimSpace(s), "\n", 2)[0] }

func TestCLIPresentThenVerify(t *testing.T) {
	out, code := runCLI(t, "present")
	if code != 0 {
		t.Fatalf("present salió %d: %s", code, out)
	}
	qr := firstLine(out)
	if !strings.HasPrefix(qr, "SLJ1:") || len(qr) != 249 {
		t.Fatalf("QR inesperado (%d caracteres)", len(qr))
	}
	out, code = runCLI(t, "verify", qr)
	if code != 0 || !strings.Contains(out, "código: 01") {
		t.Fatalf("verify: salida %d\n%s", code, out)
	}
}

func TestCLIVerifyStaleWhenClockIsOff(t *testing.T) {
	qr := firstLine(mustOut(t, "present"))
	out, code := runCLI(t, "verify", "-now", strconv.FormatInt(time.Now().Unix()+40, 10), qr)
	if code != 1 || !strings.Contains(out, "código: 07") {
		t.Fatalf("esperaba salida 1 y código 07, obtuve %d\n%s", code, out)
	}
}

func TestCLIVerifyRevokedWithRevFileAndKeysFile(t *testing.T) {
	qr := firstLine(mustOut(t, "present"))
	dir := t.TempDir()
	rev := filepath.Join(dir, "rev.bin")
	if err := os.WriteFile(rev, binary.BigEndian.AppendUint64(nil, testHandle), 0o644); err != nil {
		t.Fatal(err)
	}
	keys := filepath.Join(dir, "issuers.json")
	kb, _ := json.Marshal(issuerPubHex())
	if err := os.WriteFile(keys, kb, 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := runCLI(t, "verify", "-keys", keys, "-rev", rev, qr)
	if code != 1 || !strings.Contains(out, "código: 03") {
		t.Fatalf("esperaba revocada (03), obtuve %d\n%s", code, out)
	}
	// con un archivo de claves que no incluye al emisor → 08
	only2 := filepath.Join(dir, "solo2.json")
	kb2, _ := json.Marshal(map[string]string{"2": issuerPubHex()["2"]})
	os.WriteFile(only2, kb2, 0o644)
	out, code = runCLI(t, "verify", "-keys", only2, qr)
	if code != 1 || !strings.Contains(out, "código: 08") {
		t.Fatalf("esperaba formato/emisor desconocido (08), obtuve %d\n%s", code, out)
	}
}

func TestCLIIssueProducesParseableCredential(t *testing.T) {
	out, code := runCLI(t, "issue", "-attr", "7", "-issuer", "2", "-value", hex.EncodeToString([]byte("ANA")))
	if code != 0 {
		t.Fatalf("issue salió %d: %s", code, out)
	}
	raw, err := hex.DecodeString(strings.TrimSpace(out))
	if err != nil {
		t.Fatal(err)
	}
	c, err := parseCred(raw)
	if err != nil || c.Attr != 7 || c.Issuer != 2 || string(c.Value) != "ANA" {
		t.Fatalf("credencial emitida: %+v %v", c, err)
	}
	if !verifyRaw(&testIssuer[2].PublicKey, c.Sig, c.Signed) {
		t.Fatal("la firma del emisor 2 no verifica")
	}
}

func TestCLIErrors(t *testing.T) {
	cases := map[string][]string{
		"sin argumentos":           {},
		"comando desconocido":      {"nada"},
		"issue devpub inválido":    {"issue", "-devpub", "abcd"},
		"issue handle inválido":    {"issue", "-handle", "12"},
		"issue emisor inexistente": {"issue", "-issuer", "9"},
		"verify sin QR":            {"verify"},
		"present con devpub":       {"present", "-devpub", strings.Repeat("02", 33)},
	}
	for name, args := range cases {
		if _, code := runCLI(t, args...); code != 2 {
			t.Errorf("%s: salió %d, esperaba 2", name, code)
		}
	}
}

func TestCLIVectorsWritesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v.json")
	if out, code := runCLI(t, "vectors", "-out", path); code != 0 {
		t.Fatalf("vectors salió %d: %s", code, out)
	}
	var f vFile
	b, _ := os.ReadFile(path)
	if err := json.Unmarshal(b, &f); err != nil || len(f.Cases) < 14 {
		t.Fatalf("archivo de vectores inválido: %v (%d casos)", err, len(f.Cases))
	}
}

func mustOut(t *testing.T, args ...string) string {
	t.Helper()
	out, code := runCLI(t, args...)
	if code != 0 {
		t.Fatalf("%v salió %d: %s", args, code, out)
	}
	return out
}

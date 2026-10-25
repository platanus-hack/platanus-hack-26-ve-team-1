package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
)

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func atoi(s string) (int64, bool) {
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstNonZero(vals ...int64) int64 {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}

// readOptionalJSON lee el cuerpo si lo hay; un cuerpo vacío es válido.
func readOptionalJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "cuerpo inválido")
		return false
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return true
	}
	if err := json.Unmarshal(b, v); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON inválido")
		return false
	}
	return true
}

// flexInt lee un id que el front puede mandar como número o como texto ("17").
type flexInt int64

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := string(bytes.Trim(bytes.TrimSpace(b), `"`))
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*f = flexInt(n)
	return nil
}

var attrKeys = map[int]string{
	1: "mayor_de_18", 2: "mayor_de_21", 3: "identidad",
	4: "sangre_alergias", 5: "contacto_emergencia", 6: "donante_licencia",
}

// attrKey da el nombre que usa el front ("mayor_de_18", ...).
func attrKey(code int) string { return attrKeys[code] }

// parseAttrKey acepta "mayor_de_18" o "A1".
func parseAttrKey(s string) (int, bool) {
	for code, k := range attrKeys {
		if k == s {
			return code, true
		}
	}
	return parseAttr(s)
}

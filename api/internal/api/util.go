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

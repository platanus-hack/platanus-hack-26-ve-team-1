package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/sign"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/terminal"
)

const linkCodeTTL = 2 * time.Minute

// Atributos que se derivan a la billetera: mayor de 18, mayor de 21 y donante/licencia.
var walletCodes = []int{sign.AttrMayor18, sign.AttrMayor21, sign.AttrDonante}

// linkStart: el terminal en modo vincular leyó una cédula; devuelve el código de 6 dígitos.
func (s *Server) linkStart(w http.ResponseWriter, r *http.Request) {
	term, _ := terminal.From(r.Context())
	if term.Modo != "vincular" {
		writeErr(w, http.StatusForbidden, "el terminal no está en modo vincular")
		return
	}
	var in struct {
		H string `json:"h"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	handle, err := parseHandle(in.H)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	var status string
	err = s.DB.QueryRow(r.Context(), `SELECT status FROM cards WHERE handle = $1`, handle).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "cédula desconocida")
		return
	}
	if err != nil {
		serverErr(w, err)
		return
	}
	if status != "activa" {
		writeErr(w, http.StatusConflict, "la cédula está revocada")
		return
	}
	if _, err := s.DB.Exec(r.Context(), `DELETE FROM link_codes WHERE handle = $1 OR expires_at < now()`, handle); err != nil {
		serverErr(w, err)
		return
	}
	for try := 0; try < 5; try++ {
		n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
		if err != nil {
			serverErr(w, err)
			return
		}
		code := fmt.Sprintf("%06d", n.Int64())
		_, err = s.DB.Exec(r.Context(), `INSERT INTO link_codes (code, handle, expires_at) VALUES ($1,$2,$3)`,
			code, handle, s.now().Add(linkCodeTTL))
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			continue // código repetido: sortear otro
		}
		if err != nil {
			serverErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "c": code})
		return
	}
	writeErr(w, http.StatusServiceUnavailable, "no se pudo generar un código")
}

// linkConfirm: la billetera envía el código y la clave pública del celular.
func (s *Server) linkConfirm(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code      string `json:"code"`
		PublicKey string `json:"public_key"` // 33 B comprimida, hex
	}
	if !readJSON(w, r, &in) {
		return
	}
	pub, err := parsePub33(in.PublicKey)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	user := userFrom(r.Context())

	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		serverErr(w, err)
		return
	}
	defer tx.Rollback(r.Context())

	var handle []byte
	var exp time.Time
	err = tx.QueryRow(r.Context(), `DELETE FROM link_codes WHERE code = $1 RETURNING handle, expires_at`, in.Code).Scan(&handle, &exp)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !exp.After(s.now())) {
		writeErr(w, http.StatusNotFound, "código inválido o vencido")
		return
	}
	if err != nil {
		serverErr(w, err)
		return
	}
	var owner *int64
	var status string
	if err := tx.QueryRow(r.Context(), `SELECT citizen_id, status FROM cards WHERE handle = $1`, handle).Scan(&owner, &status); err != nil {
		serverErr(w, err)
		return
	}
	if status != "activa" {
		writeErr(w, http.StatusConflict, "la cédula está revocada")
		return
	}
	if owner == nil || *owner != user.ID {
		writeErr(w, http.StatusForbidden, "esta cédula pertenece a otra cuenta")
		return
	}
	var deviceID int64
	err = tx.QueryRow(r.Context(),
		`INSERT INTO devices (citizen_id, handle, public_key) VALUES ($1,$2,$3)
		 ON CONFLICT (public_key) DO UPDATE SET handle = EXCLUDED.handle WHERE devices.citizen_id = EXCLUDED.citizen_id
		 RETURNING id`, user.ID, handle, pub).Scan(&deviceID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusConflict, "ese dispositivo ya está vinculado a otra cuenta")
		return
	}
	if err != nil {
		serverErr(w, err)
		return
	}
	creds, err := s.issueWallet(r.Context(), tx, deviceID, handle, pub)
	if err != nil {
		serverErr(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		serverErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "credentials": creds})
}

type credential struct {
	A       string `json:"a"`
	BlobHex string `json:"blob_hex"`
	Expires int64  `json:"expires"`
}

// issueWallet firma las credenciales derivadas atadas a la clave del celular y las guarda.
func (s *Server) issueWallet(ctx context.Context, tx pgx.Tx, deviceID int64, handle, pub []byte) ([]credential, error) {
	rows, err := tx.Query(ctx,
		`SELECT code, issuer, value, expires_at FROM attributes
		  WHERE handle = $1 AND code = ANY($2) AND expires_at > $3 ORDER BY code`,
		handle, walletCodes, s.now())
	if err != nil {
		return nil, err
	}
	type row struct {
		code, issuer int
		value        []byte
		exp          time.Time
	}
	var got []row
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.code, &x.issuer, &x.value, &x.exp); err != nil {
			rows.Close()
			return nil, err
		}
		got = append(got, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := []credential{}
	var h [8]byte
	copy(h[:], handle)
	for _, x := range got {
		key := s.Issuers[byte(x.issuer)]
		if key == nil {
			return nil, fmt.Errorf("falta la clave del emisor 0x%02x", x.issuer)
		}
		at := sign.Attribute{Issuer: byte(x.issuer), Code: byte(x.code), Expires: uint32(x.exp.Unix()), Handle: h, Value: x.value}
		sig, err := at.SignWallet(key, pub)
		if err != nil {
			return nil, err
		}
		blob, err := at.WalletBlob(pub, sig)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO wallet_credentials (device_id, handle, code, blob, expires_at) VALUES ($1,$2,$3,$4,$5)
			 ON CONFLICT (device_id, code) DO UPDATE SET handle = EXCLUDED.handle, blob = EXCLUDED.blob,
			   expires_at = EXCLUDED.expires_at, created_at = now()`,
			deviceID, handle, x.code, blob, x.exp); err != nil {
			return nil, err
		}
		out = append(out, credential{attrLabel(x.code), hex.EncodeToString(blob), x.exp.Unix()})
	}
	return out, nil
}

// wallet devuelve las credenciales vigentes del ciudadano (opcional: ?public_key=<hex> para un solo celular).
func (s *Server) wallet(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r.Context())
	q := `SELECT wc.code, wc.blob, wc.expires_at
	        FROM wallet_credentials wc
	        JOIN devices d ON d.id = wc.device_id
	        JOIN cards c ON c.handle = wc.handle
	       WHERE d.citizen_id = $1 AND c.status = 'activa' AND wc.expires_at > $2`
	args := []any{user.ID, s.now()}
	if pk := r.URL.Query().Get("public_key"); pk != "" {
		pub, err := parsePub33(pk)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		q += ` AND d.public_key = $3`
		args = append(args, pub)
	}
	rows, err := s.DB.Query(r.Context(), q+` ORDER BY wc.code`, args...)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	out := []credential{}
	for rows.Next() {
		var code int
		var blob []byte
		var exp time.Time
		if err := rows.Scan(&code, &blob, &exp); err != nil {
			serverErr(w, err)
			return
		}
		out = append(out, credential{attrLabel(code), hex.EncodeToString(blob), exp.Unix()})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "credentials": out})
}

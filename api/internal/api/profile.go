package api

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/api/internal/sign"
)

// Profile son los datos declarados de un ciudadano; de aquí salen los atributos del chip.
//
// Codificación de los valores (UTF-8):
//
//	A1, A2: 0x01 (solo se emiten si la persona cumple la edad)
//	A3: "NOMBRE|V-12345678|AAAA-MM-DD" (nombre | número | vigencia), máx. 64 B;
//	    el carnet universitario usa el mismo formato con el número de carnet y emisor 0x02
//	A4: texto libre "O+ penicilina", máx. 16 B
//	A5: texto libre "Mama 04141234567", máx. 16 B
//	A6: texto libre "D;LIC-B2", máx. 16 B
type Profile struct {
	Birthdate string `json:"birthdate"` // AAAA-MM-DD
	Identity  *struct {
		Name       string `json:"name"`
		ID         string `json:"id"`
		ValidUntil string `json:"valid_until"`
	} `json:"identity,omitempty"`
	// Carnet universitario: A3 firmado por la universidad (emisor 0x02), solo en la billetera.
	Carnet *struct {
		Name       string `json:"name"`
		ID         string `json:"id"` // número de carnet
		ValidUntil string `json:"valid_until"`
	} `json:"carnet,omitempty"`
	// Datos opcionales que declara el propio ciudadano; si existen, mandan sobre blood/contact/donor_license.
	Optional     *OptionalData `json:"optional,omitempty"`
	Blood        string        `json:"blood,omitempty"`
	Contact      string        `json:"contact,omitempty"`
	DonorLicense string        `json:"donor_license,omitempty"`
}

// OptionalData son los datos que el ciudadano declara desde su portal.
type OptionalData struct {
	Sangre   string `json:"sangre"`
	Alergias string `json:"alergias"`
	Contacto *struct {
		Nombre   string `json:"nombre"`
		Telefono string `json:"telefono"`
	} `json:"contacto,omitempty"`
	Donante  bool   `json:"donante"`
	Licencia string `json:"licencia"`
}

// derived calcula los textos de A4, A5 y A6 a partir de los datos opcionales.
func (o *OptionalData) derived() (blood, contact, donorLicense string) {
	blood = strings.TrimSpace(strings.TrimSpace(o.Sangre) + " " + strings.TrimSpace(o.Alergias))
	if c := o.Contacto; c != nil && (strings.TrimSpace(c.Nombre) != "" || strings.TrimSpace(c.Telefono) != "") {
		contact = strings.TrimSpace(strings.TrimSpace(c.Nombre) + " " + chipPhone(c.Telefono))
	}
	if lic := strings.TrimSpace(o.Licencia); o.Donante || lic != "" {
		d := "N"
		if o.Donante {
			d = "D"
		}
		donorLicense = d + ";" + lic
	}
	return
}

type attrValue struct {
	Code  byte
	Value []byte
}

// Attributes calcula los atributos que corresponden al perfil en la fecha dada.
func (p Profile) Attributes(now time.Time) ([]attrValue, error) {
	if o := p.Optional; o != nil {
		b, c, d := o.derived()
		p.Blood, p.Contact, p.DonorLicense = b, c, d
	}
	var out []attrValue
	add := func(code byte, v []byte) error {
		if len(v) > sign.MaxValueLen(code) {
			return fmt.Errorf("el valor de %s excede %d bytes", attrLabel(int(code)), sign.MaxValueLen(code))
		}
		out = append(out, attrValue{code, v})
		return nil
	}

	if p.Birthdate != "" {
		b, err := time.Parse("2006-01-02", p.Birthdate)
		if err != nil {
			return nil, errors.New("birthdate debe ser AAAA-MM-DD")
		}
		if !b.AddDate(18, 0, 0).After(now) {
			if err := add(sign.AttrMayor18, []byte{1}); err != nil {
				return nil, err
			}
		}
		if !b.AddDate(21, 0, 0).After(now) {
			if err := add(sign.AttrMayor21, []byte{1}); err != nil {
				return nil, err
			}
		}
	}
	if id := p.Identity; id != nil && id.Name != "" && id.ID != "" {
		if err := add(sign.AttrIdentidad, []byte(id.Name+"|"+id.ID+"|"+id.ValidUntil)); err != nil {
			return nil, err
		}
	}
	for _, f := range []struct {
		code byte
		v    string
	}{{sign.AttrSangre, p.Blood}, {sign.AttrEmergencia, p.Contact}, {sign.AttrDonante, p.DonorLicense}} {
		if f.v != "" {
			if err := add(f.code, []byte(f.v)); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

// CarnetValue devuelve el valor del carnet universitario (A3 del emisor 0x02), si el perfil lo tiene.
func (p Profile) CarnetValue() ([]byte, bool, error) {
	c := p.Carnet
	if c == nil || c.Name == "" || c.ID == "" {
		return nil, false, nil
	}
	v := []byte(c.Name + "|" + c.ID + "|" + c.ValidUntil)
	if len(v) > sign.MaxValueLen(sign.AttrIdentidad) {
		return nil, false, errors.New("el valor del carnet excede 64 bytes")
	}
	return v, true, nil
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// chipPhone da el teléfono como se escribe en el chip: un número venezolano +58 414... pasa a 0414...
// (11 dígitos) para dejar más bytes al nombre; los demás van en dígitos.
func chipPhone(s string) string {
	d := digitsOnly(s)
	if strings.HasPrefix(d, "58") && len(d) == 12 {
		return "0" + d[2:]
	}
	return d
}

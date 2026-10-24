package api

import (
	"testing"
	"time"
)

func TestProfileAttributes(t *testing.T) {
	now := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	codes := func(p Profile) map[byte]bool {
		a, err := p.Attributes(now)
		if err != nil {
			t.Fatal(err)
		}
		m := map[byte]bool{}
		for _, x := range a {
			m[x.Code] = true
		}
		return m
	}
	if m := codes(Profile{Birthdate: "2010-01-01"}); len(m) != 0 {
		t.Fatalf("menor de edad: %v", m)
	}
	if m := codes(Profile{Birthdate: "2007-10-24"}); !m[1] || m[2] {
		t.Fatalf("recién 19: %v", m) // 2007-10-24 cumple 19, no 21
	}
	if m := codes(Profile{Birthdate: "2000-01-01", Blood: "O+"}); !m[1] || !m[2] || !m[4] {
		t.Fatalf("adulto: %v", m)
	}
	if _, err := (Profile{Blood: "01234567890123456"}).Attributes(now); err == nil {
		t.Fatal("valor de 17 B aceptado")
	}
	if _, err := (Profile{Birthdate: "ayer"}).Attributes(now); err == nil {
		t.Fatal("fecha inválida aceptada")
	}
}

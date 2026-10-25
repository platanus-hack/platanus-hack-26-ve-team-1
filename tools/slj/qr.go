package main

import "errors"

// printQR se completa si se añade rsc.io/qr; por ahora el QR se genera en la PWA.
func printQR(string) error {
	return errors.New("-qr no disponible: copia la cadena SLJ1 al verificador")
}

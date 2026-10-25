// Gateway del terminal sIsA: une el lector (ESP32 por USB) con la API de p1-backend.
// Contrato: docs/contrato-terminal.md, enlaces 1 (lector ↔ gateway) y 4 (gateway ↔ API).
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"go.bug.st/serial"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/gateway/internal/api"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/gateway/internal/cola"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/gateway/internal/config"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/gateway/internal/puente"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/gateway/internal/voz"
)

func main() {
	ruta := flag.String("config", "config.local.json", "archivo de configuración")
	flag.Parse()

	c, err := config.Cargar(*ruta)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	logger := log.New(os.Stdout, "", log.Ltime)
	if err := os.MkdirAll(c.Logs, 0o755); err == nil {
		nombre := filepath.Join(c.Logs, "gateway-"+time.Now().Format("2006-01-02")+".log")
		if f, err := os.OpenFile(nombre, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			defer f.Close()
			logger.SetOutput(io.MultiWriter(os.Stdout, f))
		}
	}

	if err := os.MkdirAll(c.Datos, 0o755); err != nil {
		logger.Fatalf("Carpeta de datos: %v", err)
	}
	q, err := cola.Abrir(filepath.Join(c.Datos, "cola"))
	if err != nil {
		logger.Fatalf("Cola: %v", err)
	}

	cliente := &api.Cliente{Base: c.API, HTTP: &http.Client{Timeout: 90 * time.Second}}
	p := puente.Nuevo(cliente, q, voz.Nueva(c.Audio), logger, c.Tokens, c.Terminal, c.Datos)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	logger.Printf("Gateway sIsA: API %s, lector en %s, terminal %s, %d evento(s) en cola",
		c.API, c.Puerto, c.Terminal, q.Pendientes())
	go q.Correr(ctx, p.EnviarEvento(ctx), logger.Printf)
	go p.Consola(ctx, os.Stdin, stop)

	p.Conectar(ctx, func() (io.ReadWriteCloser, error) {
		port, err := serial.Open(c.Puerto, &serial.Mode{
			BaudRate: c.Baudios,
			DataBits: 8,
			// DTR y RTS en bajo: activos, reinician el ESP32 o lo dejan en modo de grabación
			InitialStatusBits: &serial.ModemOutputBits{DTR: false, RTS: false},
		})
		if err != nil {
			return nil, err
		}
		return port, nil
	})
	logger.Printf("Gateway detenido")
}

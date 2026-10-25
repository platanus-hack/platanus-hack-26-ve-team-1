package puente

import (
	"bufio"
	"context"
	"io"
	"strconv"
	"strings"
	"time"
)

const ayuda = `Comandos:
  terminales           lista los terminales con token
  terminal <n|nombre>  cambia el terminal activo (bar, clínica, ambulancia…)
  sync                 reenvía time, rev y cfg al lector
  estado               lector, terminal activo y eventos en cola
  salir                cierra el gateway`

// Consola atiende los comandos del operador por la entrada estándar.
func (p *Puente) Consola(ctx context.Context, entrada io.Reader, salir func()) {
	p.log.Printf("Escribe «ayuda» para ver los comandos.")
	sc := bufio.NewScanner(entrada)
	for sc.Scan() {
		campos := strings.Fields(sc.Text())
		if len(campos) == 0 {
			continue
		}
		arg := strings.Join(campos[1:], " ")
		switch strings.ToLower(campos[0]) {
		case "ayuda", "?":
			p.log.Print(ayuda)
		case "terminales":
			activo, _ := p.Activo()
			for i, nombre := range p.Nombres() {
				marca := " "
				if nombre == activo {
					marca = "*"
				}
				p.log.Printf("%s %d. %s", marca, i+1, nombre)
			}
		case "terminal":
			nombre, ok := p.buscarTerminal(arg)
			if !ok {
				p.log.Printf("No hay un terminal %q; usa «terminales» para ver la lista", arg)
				continue
			}
			if err := p.CambiarTerminal(nombre); err != nil {
				p.log.Print(err)
				continue
			}
			if !p.lectorVivo() {
				p.log.Printf("Terminal activo: %s; su configuración sale con el primer latido del lector", nombre)
				continue
			}
			p.log.Printf("Terminal activo: %s; enviando su configuración al lector", nombre)
			go p.Sincronizar(ctx, true)
		case "sync":
			// El contrato pide no enviar nada antes del primer latido
			if !p.lectorVivo() {
				p.log.Printf("El lector aún no envió su primer latido; no hay a quién sincronizar")
				continue
			}
			go p.Sincronizar(ctx, true)
		case "estado":
			p.log.Print(p.estado())
		case "salir":
			salir()
			return
		default:
			p.log.Printf("Comando desconocido %q; escribe «ayuda»", campos[0])
		}
	}
}

// buscarTerminal acepta el número de la lista, el nombre o un prefijo único del nombre.
func (p *Puente) buscarTerminal(arg string) (string, bool) {
	nombres := p.Nombres()
	if n, err := strconv.Atoi(arg); err == nil && n >= 1 && n <= len(nombres) {
		return nombres[n-1], true
	}
	hallado := ""
	for _, nombre := range nombres {
		if strings.EqualFold(nombre, arg) {
			return nombre, true
		}
		if arg != "" && strings.HasPrefix(strings.ToLower(nombre), strings.ToLower(arg)) {
			if hallado != "" {
				return "", false
			}
			hallado = nombre
		}
	}
	return hallado, hallado != ""
}

func (p *Puente) estado() string {
	p.mu.Lock()
	terminal, conectado, sincronizado, ultimoHb := p.terminal, p.lector != nil, p.sincronizado, p.ultimoHb
	p.mu.Unlock()

	lector := "desconectado"
	if conectado {
		lector = "conectado, esperando su primer latido"
		if !ultimoHb.IsZero() {
			lector = "conectado, último latido hace " + time.Since(ultimoHb).Round(time.Second).String()
		}
		if !sincronizado {
			lector += " (sin sincronizar)"
		}
	}
	return "Terminal activo: " + terminal + "\nLector: " + lector +
		"\nCola: " + strconv.Itoa(p.cola.Pendientes()) + " evento(s) por enviar"
}

// Package puente une el lector (ESP32, enlace 1) con la API (enlace 4) según
// docs/contrato-terminal.md: sincroniza hora, revocadas y configuración, encola los
// eventos y conduce la emisión y la vinculación.
package puente

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/gateway/internal/api"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/gateway/internal/cola"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/gateway/internal/protocolo"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/gateway/internal/voz"
)

// Cada cuánto se reenvían la hora y la lista de revocadas y se revisa la config (contrato).
const periodo = time.Minute

var (
	reHandle    = regexp.MustCompile(`^[0-9A-Fa-f]{16}$`)
	reAtributo  = regexp.MustCompile(`^A[1-6]$`)
	reResultado = regexp.MustCompile(`^0[1-6]$`)

	nombresAtributo = map[string]string{
		"A1": "Mayor de 18", "A2": "Mayor de 21", "A3": "Identidad",
		"A4": "Sangre y alergias", "A5": "Contacto de emergencia", "A6": "Donante y licencia",
	}
	nombresResultado = map[string]string{
		"01": "aprobado", "02": "firma inválida", "03": "cédula revocada",
		"04": "cédula vencida", "05": "sin permiso", "06": "rechazado por el ciudadano",
	}
)

type Puente struct {
	api    *api.Cliente
	cola   *cola.Cola
	voz    *voz.Voz
	log    *log.Logger
	datos  string            // copias locales de la última config y lista de revocadas
	tokens map[string]string // nombre del terminal → token; no cambia después de Nuevo

	mu                                   sync.Mutex
	terminal                             string
	lector                               *protocolo.Lector // nil sin conexión
	sincronizado                         bool              // el lector ya recibió time, rev y cfg
	cfgEnviada                           api.Config
	ultimoHb                             time.Time
	sincronizando, otraVez, otraCompleta bool // pedidos de sincronización durante una en curso

	emitiendo  atomic.Bool
	respuestas chan protocolo.Mensaje // ack y written para la emisión en curso
}

func Nuevo(cl *api.Cliente, q *cola.Cola, v *voz.Voz, logger *log.Logger,
	tokens map[string]string, terminal, datos string) *Puente {
	return &Puente{
		api: cl, cola: q, voz: v, log: logger, datos: datos, tokens: tokens,
		terminal: terminal, respuestas: make(chan protocolo.Mensaje, 8),
	}
}

// ─── Conexión con el lector ──────────────────────────────────────────────────

// Abrir abre el puerto del lector.
type Abrir func() (io.ReadWriteCloser, error)

// Conectar mantiene la conexión con el lector: si el puerto no está o se cae, reintenta
// cada 2 s hasta que ctx termine.
func (p *Puente) Conectar(ctx context.Context, abrir Abrir) {
	avisado := false
	for ctx.Err() == nil {
		rw, err := abrir()
		if err != nil {
			if !avisado {
				p.log.Printf("Esperando al lector: %v", err)
				avisado = true
			}
			select {
			case <-ctx.Done():
			case <-time.After(2 * time.Second):
			}
			continue
		}
		avisado = false
		p.log.Printf("Lector conectado; esperando su primer latido")
		err = p.Atender(ctx, rw)
		rw.Close()
		if ctx.Err() == nil {
			p.log.Printf("Lector desconectado: %v", err)
		}
	}
}

// Atender atiende una conexión hasta que se corte. El lector se reinicia al abrir el
// puerto, así que nada se le envía antes de su primer latido.
func (p *Puente) Atender(ctx context.Context, rw io.ReadWriter) error {
	l := protocolo.NuevoLector(rw)
	p.mu.Lock()
	p.lector, p.sincronizado, p.cfgEnviada, p.ultimoHb = l, false, api.Config{}, time.Time{}
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		p.lector, p.sincronizado = nil, false
		p.mu.Unlock()
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go p.sincronizarCada(ctx)
	go p.avisarSinLatido(ctx)

	errc := make(chan error, 1)
	go func() { errc <- l.Leer(func(m protocolo.Mensaje) { p.manejar(ctx, m) }) }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errc:
		return err
	}
}

// avisarSinLatido: el lector late cada 5 s; si en 15 s no llegó ninguno, lo más probable
// es que el puerto no sea el del ESP32 (en una laptop, COM10 puede ser un puerto Bluetooth).
func (p *Puente) avisarSinLatido(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(15 * time.Second):
	}
	if !p.lectorVivo() {
		p.log.Printf("Sin latido del lector en 15 s: revisa que el puerto configurado sea el del ESP32 " +
			"(pio device list) y que ningún monitor serie lo tenga abierto")
	}
}

// lectorVivo indica si hay un lector conectado que ya envió su primer latido.
func (p *Puente) lectorVivo() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lector != nil && !p.ultimoHb.IsZero()
}

func (p *Puente) sincronizarCada(ctx context.Context) {
	t := time.NewTicker(periodo)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p.mu.Lock()
			listo := p.sincronizado
			p.mu.Unlock()
			if listo {
				p.Sincronizar(ctx, false)
			}
		}
	}
}

// manejar procesa cada mensaje del lector sin bloquear la lectura: lo que espera a la
// API corre en otra goroutine.
func (p *Puente) manejar(ctx context.Context, m protocolo.Mensaje) {
	switch m.T {
	case "hb":
		p.mu.Lock()
		p.ultimoHb = time.Now()
		pendiente := !p.sincronizado
		p.mu.Unlock()
		if pendiente || (m.Sync != nil && !*m.Sync) {
			go p.Sincronizar(ctx, true)
		}
	case "verif":
		p.registrarLectura(m)
	case "tamper":
		p.registrarManipulacion()
	case "uid":
		if !p.emitiendo.CompareAndSwap(false, true) {
			p.log.Printf("Emisión: ya hay una en curso; se ignora la tarjeta %s", m.UID)
			return
		}
		go func() {
			defer p.emitiendo.Store(false)
			p.emitir(ctx, m.UID)
		}()
	case "link":
		go p.vincular(ctx, m.H)
	case "rol":
		go p.cambiarPorRol(ctx, m.Rol)
	case "ack", "written":
		if p.emitiendo.Load() {
			select {
			case p.respuestas <- m:
			default:
			}
		}
	case "log":
		p.log.Printf("[lector] %s", m.M)
	}
}

// enviar recibe directamente lo que devuelven los constructores de protocolo.
func (p *Puente) enviar(linea []byte, err error) error {
	if err != nil {
		return err
	}
	p.mu.Lock()
	l := p.lector
	p.mu.Unlock()
	if l == nil {
		return fmt.Errorf("no hay lector conectado")
	}
	return l.Enviar(linea)
}

// ─── Sincronización (time, rev, cfg) ─────────────────────────────────────────

// Sincronizar envía al lector la hora, la lista de revocadas y la configuración del
// terminal activo (esta última si cambió, o siempre con completa). Sin API usa la
// última copia guardada, para que el lector nunca se quede sin lista ni sin rol.
// Los pedidos que llegan durante una sincronización se juntan en una sola repetición.
func (p *Puente) Sincronizar(ctx context.Context, completa bool) {
	p.mu.Lock()
	if p.sincronizando {
		p.otraVez, p.otraCompleta = true, p.otraCompleta || completa
		p.mu.Unlock()
		return
	}
	p.sincronizando = true
	p.mu.Unlock()

	for {
		p.sincronizarUnaVez(ctx, completa)
		p.mu.Lock()
		if !p.otraVez {
			p.sincronizando = false
			p.mu.Unlock()
			return
		}
		completa, p.otraVez, p.otraCompleta = p.otraCompleta, false, false
		p.mu.Unlock()
	}
}

func (p *Puente) sincronizarUnaVez(ctx context.Context, completa bool) {
	if err := p.enviar(protocolo.Time(time.Now().Unix())); err != nil {
		p.log.Printf("Sincronización: %v", err)
		return
	}

	hs, err := p.api.Revocaciones(ctx)
	switch {
	case err == nil:
		p.guardar("revocaciones.json", hs)
	case p.recuperar("revocaciones.json", &hs):
		p.log.Printf("Revocadas: la API no responde (%v); uso la copia local (%d)", err, len(hs))
		err = nil
	default:
		p.log.Printf("Revocadas: la API no responde (%v) y no hay copia local; el lector conserva su lista", err)
	}
	if err == nil {
		trozos, err := protocolo.Rev(hs)
		if err != nil {
			p.log.Printf("Revocadas: %v", err)
		}
		for _, t := range trozos {
			if err := p.enviar(t, nil); err != nil {
				p.log.Printf("Sincronización: %v", err)
				return
			}
		}
	}

	terminal, token := p.Activo()
	c, err := p.api.Config(ctx, token)
	switch {
	case err == nil:
		p.guardar(archivoConfig(terminal), c)
	case p.recuperar(archivoConfig(terminal), &c):
		p.log.Printf("Config: la API no responde (%v); uso la última de %s", err, terminal)
	default:
		// sincronizado sigue en false: el próximo latido lo vuelve a intentar
		p.log.Printf("Config de %s: %v; reintento con el próximo latido", terminal, err)
		return
	}

	p.mu.Lock()
	cambio := completa || c != p.cfgEnviada
	p.mu.Unlock()
	if cambio {
		if err := p.enviar(protocolo.Cfg(c.Rol, c.A, c.Local, c.Modo)); err != nil {
			p.log.Printf("Sincronización: %v", err)
			return
		}
		p.log.Printf("Terminal activo: %s (rol %s, modo %s%s)", terminal, c.Rol, c.Modo, textoAtributo(c.A))
	}

	p.mu.Lock()
	if p.terminal == terminal {
		p.cfgEnviada, p.sincronizado = c, true
	}
	p.mu.Unlock()
}

func textoAtributo(a string) string {
	if a == "" {
		return ""
	}
	return ", pide " + a + " " + nombresAtributo[a]
}

func archivoConfig(terminal string) string {
	return "config-" + strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			return r
		}
		return '_'
	}, terminal) + ".json"
}

func (p *Puente) guardar(nombre string, v any) {
	datos, err := json.Marshal(v)
	if err == nil {
		err = os.WriteFile(filepath.Join(p.datos, nombre), datos, 0o600)
	}
	if err != nil {
		p.log.Printf("No se pudo guardar la copia local %s: %v", nombre, err)
	}
}

func (p *Puente) recuperar(nombre string, v any) bool {
	datos, err := os.ReadFile(filepath.Join(p.datos, nombre))
	return err == nil && json.Unmarshal(datos, v) == nil
}

// ─── Eventos (verif y tamper) ────────────────────────────────────────────────

func (p *Puente) registrarLectura(m protocolo.Mensaje) {
	terminal, _ := p.Activo()
	a := string(m.A)
	if !reHandle.MatchString(m.H) || !reAtributo.MatchString(a) || !reResultado.MatchString(m.Res) {
		p.log.Printf("Lectura fuera del contrato, no se envía: h=%q a=%q res=%q", m.H, a, m.Res)
		return
	}

	emergencia := ""
	if m.Emg {
		emergencia = " (emergencia)"
	}
	p.log.Printf("%s: %s → %s %s%s", terminal, nombresAtributo[a], m.Res, nombresResultado[m.Res], emergencia)
	p.voz.Decir(voz.ParaResultado(m.Res, m.Emg))

	evento, _ := json.Marshal(struct {
		T   string `json:"t"`
		H   string `json:"h"`
		A   string `json:"a"`
		Res string `json:"res"`
		TS  int64  `json:"ts"`
		Emg bool   `json:"emg"`
	}{"verif", m.H, a, m.Res, m.TS, m.Emg})
	p.encolar(terminal, evento)
}

func (p *Puente) registrarManipulacion() {
	terminal, _ := p.Activo()
	p.log.Printf("ALERTA: %s detectó manipulación; el lector queda bloqueado hasta el rearme", terminal)
	p.voz.Decir(voz.Bloqueado)

	evento, _ := json.Marshal(struct {
		T  string `json:"t"`
		TS int64  `json:"ts"`
	}{"tamper", time.Now().Unix()})
	p.encolar(terminal, evento)
}

func (p *Puente) encolar(terminal string, evento json.RawMessage) {
	if err := p.cola.Agregar(cola.Item{Terminal: terminal, Evento: evento, Creado: time.Now()}); err != nil {
		p.log.Printf("No se pudo guardar el evento en la cola: %v", err)
	}
}

// EnviarEvento es el envío de la cola: cada evento con el token del terminal que lo
// generó. Un rechazo de la API (4xx) no se reintenta.
func (p *Puente) EnviarEvento(ctx context.Context) func(cola.Item) error {
	return func(it cola.Item) error {
		token, ok := p.tokens[it.Terminal]
		if !ok {
			return cola.Rechazado{Err: fmt.Errorf("no hay token para el terminal %q", it.Terminal)}
		}
		err := p.api.Evento(ctx, token, it.Evento)
		if api.EsPermanente(err) {
			return cola.Rechazado{Err: err}
		}
		return err
	}
}

// ─── Terminal activo ─────────────────────────────────────────────────────────

// Activo devuelve el terminal activo y su token.
func (p *Puente) Activo() (string, string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.terminal, p.tokens[p.terminal]
}

// Nombres devuelve los terminales con token, en orden alfabético.
func (p *Puente) Nombres() []string {
	out := make([]string, 0, len(p.tokens))
	for nombre := range p.tokens {
		out = append(out, nombre)
	}
	sort.Strings(out)
	return out
}

// cambiarPorRol atiende el menú oculto de la Nextion: activa el terminal que tiene ese
// rol (si el activo ya lo tiene, se queda) y le envía su cfg al lector. Si ninguno lo
// tiene, reenvía la cfg actual para que el lector no se quede esperando.
func (p *Puente) cambiarPorRol(ctx context.Context, rol string) {
	actual, _ := p.Activo()
	elegido := ""
	for _, nombre := range p.Nombres() {
		c, ok := p.configDe(ctx, nombre)
		if ok && c.Rol == rol && (elegido == "" || nombre == actual) {
			elegido = nombre
		}
	}

	if elegido == "" {
		p.log.Printf("Menú del lector: ningún terminal tiene el rol %q; sigue %s", rol, actual)
	} else if err := p.CambiarTerminal(elegido); err == nil {
		p.log.Printf("Menú del lector: terminal activo %s (rol %s)", elegido, rol)
	}
	p.Sincronizar(ctx, true)
}

// configDe devuelve la config de un terminal desde la API o, si no responde, desde la copia local.
func (p *Puente) configDe(ctx context.Context, nombre string) (api.Config, bool) {
	c, err := p.api.Config(ctx, p.tokens[nombre])
	if err == nil {
		p.guardar(archivoConfig(nombre), c)
		return c, true
	}
	return c, p.recuperar(archivoConfig(nombre), &c)
}

// CambiarTerminal cambia el terminal activo (en la demo, un mismo lector hace de bar,
// clínica o ambulancia). Su cfg sale en la próxima sincronización.
func (p *Puente) CambiarTerminal(nombre string) error {
	if _, ok := p.tokens[nombre]; !ok {
		return fmt.Errorf("no hay token para %q", nombre)
	}
	p.mu.Lock()
	p.terminal, p.sincronizado = nombre, false
	p.mu.Unlock()
	return nil
}

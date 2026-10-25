package puente

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/gateway/internal/api"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/gateway/internal/cola"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/gateway/internal/voz"
)

// apiFalsa imita las rutas de terminal de p1-backend (api/internal/api y terminal).
type apiFalsa struct {
	mu       sync.Mutex
	configs  map[string]api.Config // token → config
	rev      []string
	emisor   string // token con una emisión pendiente; vacío = 409
	bloques  []api.Bloque
	codigo   string
	caida    bool
	eventos  []string // "token cuerpo"
	uidVisto string

	// Consentimiento: consentInicio es la respuesta de POST /api/consent (0 = 200 con el id 42)
	// y consentEstados lo que devuelve GET /api/consent/42, uno por consulta (el último se repite)
	consentInicio  int
	consentEstados []string
	consentPedidos []string // "token h a" de cada POST
	consultas      int
}

func (f *apiFalsa) servidor(t *testing.T) *httptest.Server {
	responder := func(w http.ResponseWriter, estado int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(estado)
		json.NewEncoder(w).Encode(v)
	}
	terminal := func(h func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			caida := f.caida
			f.mu.Unlock()
			if caida {
				responder(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": "dormida"})
				return
			}
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if _, ok := f.configs[token]; !ok && r.URL.Path != "/api/revocations" {
				responder(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "token inválido"})
				return
			}
			h(w, r, token)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/terminal/config", terminal(func(w http.ResponseWriter, r *http.Request, token string) {
		responder(w, http.StatusOK, f.configs[token])
	}))
	mux.HandleFunc("GET /api/revocations", terminal(func(w http.ResponseWriter, r *http.Request, _ string) {
		responder(w, http.StatusOK, map[string]any{"hs": f.rev})
	}))
	mux.HandleFunc("POST /api/events", terminal(func(w http.ResponseWriter, r *http.Request, token string) {
		cuerpo, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.eventos = append(f.eventos, token+" "+string(cuerpo))
		f.mu.Unlock()
		responder(w, http.StatusOK, map[string]any{"ok": true})
	}))
	mux.HandleFunc("POST /api/emision/start", terminal(func(w http.ResponseWriter, r *http.Request, token string) {
		var in struct{ UID string }
		json.NewDecoder(r.Body).Decode(&in)
		f.mu.Lock()
		f.uidVisto = in.UID
		emisor, bloques := f.emisor, f.bloques
		f.mu.Unlock()
		if f.configs[token].Modo != "emitir" {
			responder(w, http.StatusForbidden, map[string]any{"ok": false, "error": "el terminal no está en modo emitir"})
			return
		}
		if token != emisor {
			responder(w, http.StatusConflict, map[string]any{"ok": false, "error": "no hay una emisión pendiente"})
			return
		}
		responder(w, http.StatusOK, map[string]any{"ok": true, "h": "9e429475799fc8ed", "blocks": bloques})
	}))
	mux.HandleFunc("POST /api/link/start", terminal(func(w http.ResponseWriter, r *http.Request, _ string) {
		responder(w, http.StatusOK, map[string]any{"ok": true, "c": f.codigo})
	}))
	mux.HandleFunc("POST /api/consent", terminal(func(w http.ResponseWriter, r *http.Request, token string) {
		var in struct{ H, A string }
		json.NewDecoder(r.Body).Decode(&in)
		f.mu.Lock()
		f.consentPedidos = append(f.consentPedidos, token+" "+in.H+" "+in.A)
		inicio := f.consentInicio
		f.mu.Unlock()
		switch inicio {
		case 0:
			responder(w, http.StatusOK, map[string]any{"ok": true, "id": 42, "estado": "pendiente"})
		case http.StatusUnprocessableEntity:
			responder(w, inicio, map[string]any{"ok": false, "estado": "sin_whatsapp"})
		default:
			responder(w, inicio, map[string]any{"ok": false, "error": "cédula revocada"})
		}
	}))
	mux.HandleFunc("GET /api/consent/{id}", terminal(func(w http.ResponseWriter, r *http.Request, _ string) {
		if r.PathValue("id") != "42" {
			responder(w, http.StatusNotFound, map[string]any{"ok": false, "error": "solicitud no encontrada"})
			return
		}
		f.mu.Lock()
		estado := "pendiente"
		if n := len(f.consentEstados); n > 0 {
			estado = f.consentEstados[min(f.consultas, n-1)]
		}
		f.consultas++
		f.mu.Unlock()
		responder(w, http.StatusOK, map[string]any{"ok": true, "id": 42, "estado": estado})
	}))

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func (f *apiFalsa) eventosRecibidos() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.eventos...)
}

func (f *apiFalsa) caer(caida bool) {
	f.mu.Lock()
	f.caida = caida
	f.mu.Unlock()
}

// lectorFalso hace de ESP32 del otro lado del puerto.
type lectorFalso struct {
	t      *testing.T
	conn   net.Conn
	lineas chan map[string]any
}

func (l *lectorFalso) enviar(linea string) {
	l.t.Helper()
	if _, err := l.conn.Write([]byte(linea + "\n")); err != nil {
		l.t.Fatalf("el lector falso no pudo enviar %s: %v", linea, err)
	}
}

// recibir espera la próxima línea del gateway y comprueba su tipo.
func (l *lectorFalso) recibir(tipo string) map[string]any {
	l.t.Helper()
	select {
	case m := <-l.lineas:
		if m["t"] != tipo {
			l.t.Fatalf("se esperaba %q y llegó %v", tipo, m)
		}
		return m
	case <-time.After(3 * time.Second):
		l.t.Fatalf("el gateway no envió %q", tipo)
		return nil
	}
}

// sincronizar envía el primer latido y consume time, rev y cfg.
func (l *lectorFalso) sincronizar() map[string]any {
	l.t.Helper()
	l.enviar(`{"t":"hb"}`)
	l.recibir("time")
	for {
		if r := l.recibir("rev"); r["more"] == false {
			break
		}
	}
	return l.recibir("cfg")
}

type registro struct {
	mu sync.Mutex
	b  strings.Builder
}

func (r *registro) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.b.Write(p)
}

func (r *registro) contiene(s string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Contains(r.b.String(), s)
}

func esperarHasta(t *testing.T, plazo time.Duration, que string, cond func() bool) {
	t.Helper()
	limite := time.Now().Add(plazo)
	for !cond() {
		if time.Now().After(limite) {
			t.Fatalf("no se cumplió a tiempo: %s", que)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func armar(t *testing.T, f *apiFalsa, tokens map[string]string, terminal string) (*Puente, *lectorFalso, *registro) {
	t.Helper()
	srv := f.servidor(t)
	dir := t.TempDir()
	q, err := cola.Abrir(filepath.Join(dir, "cola"))
	if err != nil {
		t.Fatal(err)
	}
	reg := &registro{}
	logger := log.New(reg, "", 0)
	cl := &api.Cliente{Base: srv.URL, HTTP: &http.Client{Timeout: 5 * time.Second}}
	p := Nuevo(cl, q, voz.Nueva(""), logger, tokens, terminal, dir)

	ctx, cancel := context.WithCancel(context.Background())
	gw, esp := net.Pipe()
	go p.Atender(ctx, gw)
	go q.Correr(ctx, p.EnviarEvento(ctx), logger.Printf)

	l := &lectorFalso{t: t, conn: esp, lineas: make(chan map[string]any, 64)}
	go func() {
		sc := bufio.NewScanner(esp)
		for sc.Scan() {
			var m map[string]any
			if json.Unmarshal(sc.Bytes(), &m) == nil {
				l.lineas <- m
			}
		}
	}()
	t.Cleanup(func() {
		cancel()
		gw.Close()
		esp.Close()
	})
	return p, l, reg
}

var (
	cfgBar      = api.Config{Rol: "comercio", A: "A1", Local: "Bar La Esquina", Modo: "verificar"}
	cfgClinica  = api.Config{Rol: "salud", A: "A4", Local: "Clinica Salud", Modo: "verificar"}
	cfgRegistro = api.Config{Rol: "registro", A: "", Local: "Registro", Modo: "emitir"}
)

func TestSincronizaAlPrimerLatidoYEnviaLasLecturas(t *testing.T) {
	f := &apiFalsa{
		configs: map[string]api.Config{"tok-bar": cfgBar},
		rev:     []string{"7f94683ed2fff69d", "0000000000000001", "0000000000000002"},
	}
	_, l, _ := armar(t, f, map[string]string{"Bar La Esquina": "tok-bar"}, "Bar La Esquina")

	l.enviar("rst:0x1 (POWERON_RESET),boot:0x13") // basura del bootloader: se ignora
	l.enviar(`{"t":"hb","sync":false}`)
	if m := l.recibir("time"); m["ts"].(float64) < 1e9 {
		t.Errorf("hora: %v", m)
	}
	if r := l.recibir("rev"); len(r["hs"].([]any)) != 2 || r["more"] != true {
		t.Errorf("primer trozo: %v", r)
	}
	if r := l.recibir("rev"); len(r["hs"].([]any)) != 1 || r["more"] != false {
		t.Errorf("último trozo: %v", r)
	}
	if c := l.recibir("cfg"); c["rol"] != "comercio" || c["a"] != "A1" || c["modo"] != "verificar" || c["local"] != "Bar La Esquina" {
		t.Errorf("cfg: %v", c)
	}

	l.enviar(`{"t":"verif","h":"9E429475799FC8ED","a":"A1","res":"01","ts":1761300000,"emg":false}`)
	l.enviar(`{"t":"tamper"}`)
	esperarHasta(t, 3*time.Second, "dos eventos en la API", func() bool { return len(f.eventosRecibidos()) == 2 })
	ev := f.eventosRecibidos()
	if ev[0] != `tok-bar {"t":"verif","h":"9E429475799FC8ED","a":"A1","res":"01","ts":1761300000,"emg":false}` {
		t.Errorf("verif enviado: %s", ev[0])
	}
	if !strings.HasPrefix(ev[1], `tok-bar {"t":"tamper","ts":`) {
		t.Errorf("tamper enviado: %s", ev[1])
	}
}

func TestCambiarDeTerminalEnviaSuCfgYSuToken(t *testing.T) {
	f := &apiFalsa{configs: map[string]api.Config{"tok-bar": cfgBar, "tok-cli": cfgClinica}}
	p, l, _ := armar(t, f, map[string]string{"Bar La Esquina": "tok-bar", "Clinica Salud": "tok-cli"}, "Bar La Esquina")
	l.sincronizar()

	if err := p.CambiarTerminal("Clinica Salud"); err != nil {
		t.Fatal(err)
	}
	l.enviar(`{"t":"hb"}`) // el próximo latido envía la cfg del terminal nuevo
	l.recibir("time")
	l.recibir("rev")
	if c := l.recibir("cfg"); c["rol"] != "salud" || c["a"] != "A4" {
		t.Errorf("cfg de la clínica: %v", c)
	}

	l.enviar(`{"t":"verif","h":"9E429475799FC8ED","a":"A4","res":"05","ts":1761300000,"emg":false}`)
	esperarHasta(t, 3*time.Second, "evento de la clínica", func() bool { return len(f.eventosRecibidos()) == 1 })
	if ev := f.eventosRecibidos()[0]; !strings.HasPrefix(ev, "tok-cli ") {
		t.Errorf("el evento debía ir con el token de la clínica: %s", ev)
	}
}

func TestConsolaNoEnviaNadaAntesDelPrimerLatido(t *testing.T) {
	f := &apiFalsa{configs: map[string]api.Config{"tok-bar": cfgBar, "tok-cli": cfgClinica}}
	p, l, reg := armar(t, f, map[string]string{"Bar La Esquina": "tok-bar", "Clinica Salud": "tok-cli"}, "Bar La Esquina")

	p.Consola(context.Background(), strings.NewReader("terminal clinica\nsync\n"), func() {})
	select {
	case m := <-l.lineas:
		t.Fatalf("el gateway envió %v antes del primer latido", m)
	case <-time.After(300 * time.Millisecond):
	}
	if !reg.contiene("sale con el primer latido") || !reg.contiene("aún no envió su primer latido") {
		t.Errorf("faltan los avisos al operador:\n%s", reg.b.String())
	}
	if c := l.sincronizar(); c["rol"] != "salud" {
		t.Errorf("con el primer latido debía llegar la cfg de la clínica: %v", c)
	}
}

func TestSinAPIUsaLaCopiaLocalYReintentaLosEventos(t *testing.T) {
	f := &apiFalsa{configs: map[string]api.Config{"tok-bar": cfgBar}, rev: []string{"7f94683ed2fff69d"}}
	p, l, _ := armar(t, f, map[string]string{"Bar La Esquina": "tok-bar"}, "Bar La Esquina")
	l.sincronizar()

	f.caer(true)
	go p.Sincronizar(context.Background(), true)
	l.recibir("time")
	if r := l.recibir("rev"); len(r["hs"].([]any)) != 1 {
		t.Errorf("rev desde la copia local: %v", r)
	}
	if c := l.recibir("cfg"); c["rol"] != "comercio" {
		t.Errorf("cfg desde la copia local: %v", c)
	}

	l.enviar(`{"t":"verif","h":"9E429475799FC8ED","a":"A1","res":"01","ts":1761300000,"emg":false}`)
	time.Sleep(300 * time.Millisecond)
	if n := len(f.eventosRecibidos()); n != 0 {
		t.Fatalf("con la API caída no debía llegar nada; llegaron %d", n)
	}
	f.caer(false)
	esperarHasta(t, 6*time.Second, "reintento de la cola", func() bool { return len(f.eventosRecibidos()) == 1 })
}

func TestEmisionEscribeLosBloquesYSella(t *testing.T) {
	bloques := []api.Bloque{
		{S: 1, B: 0, D: "019e429475799fc8ed00000000000000"},
		{S: 1, B: 1, D: strings.Repeat("00", 16)},
		{S: 1, B: 2, D: strings.Repeat("00", 16)},
		{S: 2, B: 0, D: "010101016cbe09819e429475799fc8ed"},
	}
	f := &apiFalsa{configs: map[string]api.Config{"tok-reg": cfgRegistro}, emisor: "tok-reg", bloques: bloques}
	_, l, reg := armar(t, f, map[string]string{"Registro": "tok-reg"}, "Registro")
	l.sincronizar()

	l.enviar(`{"t":"uid","uid":"A1B2C3D4"}`)
	for _, b := range bloques {
		w := l.recibir("write")
		if int(w["s"].(float64)) != b.S || int(w["b"].(float64)) != b.B || w["d"] != b.D {
			t.Fatalf("write %v, se esperaba %+v", w, b)
		}
		l.enviar(`{"t":"ack","s":` + strconv.Itoa(b.S) + `,"b":` + strconv.Itoa(b.B) + `,"ok":true}`)
	}
	l.recibir("seal")
	l.enviar(`{"t":"written","ok":true}`)

	esperarHasta(t, 3*time.Second, "emisión terminada", func() bool { return reg.contiene("Cédula emitida y sellada") })
	if f.uidVisto != "A1B2C3D4" {
		t.Errorf("UID enviado a la API: %q", f.uidVisto)
	}
}

func TestEmisionFallidaLiberaAlLector(t *testing.T) {
	f := &apiFalsa{configs: map[string]api.Config{"tok-reg": cfgRegistro}} // sin emisión pendiente: 409
	p, l, reg := armar(t, f, map[string]string{"Registro": "tok-reg"}, "Registro")
	l.sincronizar()

	l.enviar(`{"t":"uid","uid":"A1B2C3D4"}`)
	l.recibir("seal")
	l.enviar(`{"t":"written","ok":false}`)
	esperarHasta(t, 3*time.Second, "aviso al operador", func() bool { return reg.contiene("Pide la emisión desde el panel") })
	esperarHasta(t, 3*time.Second, "fin de la primera emisión", func() bool { return !p.emitiendo.Load() })

	// Un bloque que el lector no confirma también corta la emisión
	f.mu.Lock()
	f.emisor, f.bloques = "tok-reg", []api.Bloque{{S: 1, B: 0, D: strings.Repeat("00", 16)}, {S: 1, B: 1, D: strings.Repeat("00", 16)}}
	f.mu.Unlock()
	l.enviar(`{"t":"uid","uid":"A1B2C3D4"}`)
	l.recibir("write")
	l.enviar(`{"t":"ack","s":1,"b":0,"ok":false}`)
	l.recibir("seal")
	l.enviar(`{"t":"written","ok":false}`)
	esperarHasta(t, 3*time.Second, "emisión cortada", func() bool { return reg.contiene("Emisión fallida en el bloque 1 de 2") })
}

func TestVinculacionEnviaElCodigo(t *testing.T) {
	vincular := cfgRegistro
	vincular.Modo = "vincular"
	f := &apiFalsa{configs: map[string]api.Config{"tok-reg": vincular}, codigo: "123456"}
	_, l, _ := armar(t, f, map[string]string{"Registro": "tok-reg"}, "Registro")
	l.sincronizar()

	l.enviar(`{"t":"link","h":"9E429475799FC8ED"}`)
	if c := l.recibir("code"); c["c"] != "123456" {
		t.Errorf("código: %v", c)
	}
}

// ─── Consentimiento ──────────────────────────────────────────────────────────

const pedidoAna = `{"t":"pedido","h":"DB3A215CECA2B951","a":"A1"}`

// consultasRapidas acorta los plazos del consentimiento. Va antes de armar: así se
// restauran después de cerrar la conexión y sus goroutines.
func consultasRapidas(t *testing.T, plazo time.Duration) {
	antes, cada := plazoConsentimiento, cadaConsulta
	plazoConsentimiento, cadaConsulta = plazo, 10*time.Millisecond
	t.Cleanup(func() { plazoConsentimiento, cadaConsulta = antes, cada })
}

// sinMasLineas comprueba que el gateway no le envía nada más al lector.
func (l *lectorFalso) sinMasLineas() {
	l.t.Helper()
	select {
	case m := <-l.lineas:
		l.t.Fatalf("el gateway envió %v de más", m)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestConsentimientoAprobadoPorWhatsApp(t *testing.T) {
	consultasRapidas(t, 2*time.Second)
	f := &apiFalsa{configs: map[string]api.Config{"tok-bar": cfgBar},
		consentEstados: []string{"pendiente", "pendiente", "aprobado"}}
	_, l, _ := armar(t, f, map[string]string{"Bar La Esquina": "tok-bar"}, "Bar La Esquina")
	l.sincronizar()

	l.enviar(pedidoAna)
	if c := l.recibir("consent"); c["approved"] != true {
		t.Errorf("consent: %v", c)
	}
	l.sinMasLineas()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.consentPedidos) != 1 || f.consentPedidos[0] != "tok-bar DB3A215CECA2B951 A1" {
		t.Errorf("pedido a la API: %v", f.consentPedidos)
	}
	if f.consultas != 3 {
		t.Errorf("se consultó %d veces; tras el aprobado no debía seguir", f.consultas)
	}
}

func TestConsentimientoRechazadoOVencidoDa06(t *testing.T) {
	for _, estado := range []string{"rechazado", "vencido"} {
		t.Run(estado, func(t *testing.T) {
			consultasRapidas(t, 2*time.Second)
			f := &apiFalsa{configs: map[string]api.Config{"tok-bar": cfgBar}, consentEstados: []string{"pendiente", estado}}
			_, l, _ := armar(t, f, map[string]string{"Bar La Esquina": "tok-bar"}, "Bar La Esquina")
			l.sincronizar()

			l.enviar(pedidoAna)
			if c := l.recibir("consent"); c["approved"] != false {
				t.Errorf("consent: %v", c)
			}
		})
	}
}

func TestConsentimientoSinRespuestaDejaQueElLectorDe06(t *testing.T) {
	consultasRapidas(t, 300*time.Millisecond)
	f := &apiFalsa{configs: map[string]api.Config{"tok-bar": cfgBar}} // siempre pendiente
	_, l, reg := armar(t, f, map[string]string{"Bar La Esquina": "tok-bar"}, "Bar La Esquina")
	l.sincronizar()

	l.enviar(pedidoAna)
	esperarHasta(t, 3*time.Second, "aviso de plazo vencido", func() bool { return reg.contiene("no respondió a tiempo") })
	l.sinMasLineas()
}

func TestConsentimientoDeCedulaRevocadaSeRechaza(t *testing.T) {
	consultasRapidas(t, 2*time.Second)
	f := &apiFalsa{configs: map[string]api.Config{"tok-bar": cfgBar}, consentInicio: http.StatusConflict}
	p, l, _ := armar(t, f, map[string]string{"Bar La Esquina": "tok-bar"}, "Bar La Esquina")
	l.sincronizar()

	l.enviar(pedidoAna)
	if c := l.recibir("consent"); c["approved"] != false {
		t.Errorf("consent: %v", c)
	}
	if p.ResponderConsentimiento(true) {
		t.Error("un rechazo de la API no debía quedar esperando la consola")
	}
}

func TestConsentimientoSinWhatsAppPideRespuestaEnLaConsola(t *testing.T) {
	consultasRapidas(t, 3*time.Second)
	f := &apiFalsa{configs: map[string]api.Config{"tok-bar": cfgBar}, consentInicio: http.StatusUnprocessableEntity}
	p, l, reg := armar(t, f, map[string]string{"Bar La Esquina": "tok-bar"}, "Bar La Esquina")
	l.sincronizar()

	l.enviar(pedidoAna)
	esperarHasta(t, 3*time.Second, "pedido al operador", func() bool { return reg.contiene("responda «si» o «no»") })
	p.Consola(context.Background(), strings.NewReader("no\nsi\n"), func() {})
	if c := l.recibir("consent"); c["approved"] != false {
		t.Errorf("consent: %v", c)
	}
	if !reg.contiene("No hay ningún pedido esperando") {
		t.Error("el segundo comando no debía responder nada")
	}
	l.sinMasLineas()
}

func TestConsentimientoConLaAPICaidaPideRespuestaEnLaConsola(t *testing.T) {
	consultasRapidas(t, 3*time.Second)
	f := &apiFalsa{configs: map[string]api.Config{"tok-bar": cfgBar}}
	p, l, reg := armar(t, f, map[string]string{"Bar La Esquina": "tok-bar"}, "Bar La Esquina")
	l.sincronizar()

	f.caer(true)
	l.enviar(pedidoAna)
	esperarHasta(t, 3*time.Second, "pedido al operador", func() bool { return reg.contiene("la API no responde") })
	p.Consola(context.Background(), strings.NewReader("sí\n"), func() {})
	if c := l.recibir("consent"); c["approved"] != true {
		t.Errorf("consent: %v", c)
	}
}

func TestConsolaSinPedidoNoRespondeNada(t *testing.T) {
	f := &apiFalsa{configs: map[string]api.Config{"tok-bar": cfgBar}, consentEstados: []string{"pendiente"}}
	p, l, reg := armar(t, f, map[string]string{"Bar La Esquina": "tok-bar"}, "Bar La Esquina")
	l.sincronizar()

	// Mientras se espera la respuesta por WhatsApp, el operador no puede responder por el titular
	l.enviar(pedidoAna)
	esperarHasta(t, 3*time.Second, "consulta a la API", func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		return len(f.consentPedidos) == 1
	})
	p.Consola(context.Background(), strings.NewReader("si\n"), func() {})
	if !reg.contiene("No hay ningún pedido esperando") {
		t.Error("el «si» no debía aceptarse mientras se espera WhatsApp")
	}
	l.sinMasLineas()
}

func TestUnPedidoNuevoAnulaAlAnterior(t *testing.T) {
	f := &apiFalsa{configs: map[string]api.Config{"tok-bar": cfgBar}}
	p, l, _ := armar(t, f, map[string]string{"Bar La Esquina": "tok-bar"}, "Bar La Esquina")
	l.sincronizar()

	ctx1, n1, fin1 := p.empezarConsentimiento(context.Background())
	_, n2, fin2 := p.empezarConsentimiento(context.Background())
	if ctx1.Err() == nil {
		t.Error("el pedido anterior debía quedar cancelado")
	}
	p.responderConsentimiento(n1, true) // respuesta vieja: no debe llegar al lector
	p.terminarConsentimiento(n1, fin1)
	p.responderConsentimiento(n2, false)
	if c := l.recibir("consent"); c["approved"] != false {
		t.Errorf("llegó la respuesta del pedido anterior: %v", c)
	}
	p.terminarConsentimiento(n2, fin2)
}

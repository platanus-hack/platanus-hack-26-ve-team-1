package puente

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/gateway/internal/api"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/gateway/internal/protocolo"
)

// Plazos del consentimiento por WhatsApp (docs/consentimiento-whatsapp.md): la API y el
// lector esperan 60 s; el gateway consulta hasta 65 s para que el lector no se adelante
// con su propio 06. Son variables para que las pruebas no esperen tanto.
var (
	plazoConsentimiento = 65 * time.Second
	cadaConsulta        = 1500 * time.Millisecond
)

// consentir atiende un pedido del lector (una lectura 01 que espera la autorización del
// titular): la API le pregunta por WhatsApp, el gateway consulta hasta tener respuesta y
// se la entrega al lector como consent. Si la API no puede preguntar (titular sin
// WhatsApp, sin red), se la pide al operador por la consola.
func (p *Puente) consentir(ctx context.Context, h, a string) {
	ctx, n, cancelar := p.empezarConsentimiento(ctx)
	defer p.terminarConsentimiento(n, cancelar)

	if !reHandle.MatchString(h) || !reAtributo.MatchString(a) {
		p.log.Printf("Consentimiento: pedido fuera del contrato (h=%q a=%q); se rechaza", h, a)
		p.responderConsentimiento(n, false)
		return
	}
	terminal, token := p.Activo()
	p.log.Printf("Consentimiento: %s pide %s; se le pregunta al titular por WhatsApp", terminal, nombresAtributo[a])

	id, err := p.api.ConsentStart(ctx, token, h, a)
	switch {
	case err == nil:
	case api.Estado(err) == http.StatusUnprocessableEntity:
		p.consentimientoManual(ctx, n, "el titular no tiene WhatsApp o tiene los avisos apagados")
		return
	case api.EsPermanente(err):
		// Cédula desconocida o revocada: la API no la deja compartir
		p.log.Printf("Consentimiento rechazado: %v", err)
		p.responderConsentimiento(n, false)
		return
	default:
		if ctx.Err() == nil {
			p.consentimientoManual(ctx, n, "la API no responde: "+err.Error())
		} else if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			p.log.Printf("Consentimiento: la API no respondió a tiempo (¿dormida en Render?); el lector da 06")
		}
		return
	}

	p.log.Printf("Consentimiento: solicitud %d creada; la API le envía el WhatsApp al titular", id)
	avisado := false
	t := time.NewTicker(cadaConsulta)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				p.log.Printf("Consentimiento: el titular no respondió a tiempo; el lector da 06")
			}
			return
		case <-t.C:
		}

		estado, err := p.api.ConsentStatus(ctx, token, id)
		switch {
		case err == nil && estado == "aprobado":
			p.log.Printf("Consentimiento: el titular aprobó por WhatsApp")
			p.responderConsentimiento(n, true)
			return
		case err == nil && estado == "rechazado":
			p.log.Printf("Consentimiento: el titular rechazó por WhatsApp; el lector da 06")
			p.responderConsentimiento(n, false)
			return
		case err == nil && estado == "vencido":
			p.log.Printf("Consentimiento: la solicitud venció sin respuesta; el lector da 06")
			p.responderConsentimiento(n, false)
			return
		case err == nil:
			// pendiente: se sigue consultando
		case api.EsPermanente(err):
			p.log.Printf("Consentimiento: %v; el lector da 06", err)
			p.responderConsentimiento(n, false)
			return
		case !avisado && ctx.Err() == nil:
			// Sin red o API caída: no es definitivo, se sigue consultando
			p.log.Printf("Consentimiento: la API no responde (%v); sigo consultando", err)
			avisado = true
		}
	}
}

// consentimientoManual espera la respuesta del operador («si» o «no» en la consola) hasta
// que venza el plazo.
func (p *Puente) consentimientoManual(ctx context.Context, n uint64, motivo string) {
	respuesta := make(chan bool, 1)
	p.mu.Lock()
	vigente := p.consentimiento == n
	if vigente {
		p.respuestaManual = respuesta
	}
	p.mu.Unlock()
	if !vigente {
		return
	}

	p.log.Printf("Consentimiento: no se puede preguntar por WhatsApp (%s). "+
		"Pregúntele al ciudadano y responda «si» o «no» en esta consola", motivo)
	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			p.log.Printf("Consentimiento: nadie respondió en la consola; el lector da 06")
		}
	case aprobado := <-respuesta:
		p.responderConsentimiento(n, aprobado)
	}
}

// ResponderConsentimiento entrega la respuesta del operador. false = no hay ningún pedido
// esperando una respuesta en la consola.
func (p *Puente) ResponderConsentimiento(aprobado bool) bool {
	p.mu.Lock()
	r := p.respuestaManual
	p.respuestaManual = nil
	p.mu.Unlock()
	if r == nil {
		return false
	}
	r <- aprobado
	return true
}

// empezarConsentimiento registra un pedido nuevo con su plazo y cancela el anterior.
func (p *Puente) empezarConsentimiento(ctx context.Context) (context.Context, uint64, context.CancelFunc) {
	ctx, cancelar := context.WithTimeout(ctx, plazoConsentimiento)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancelarConsentimiento != nil {
		p.cancelarConsentimiento()
	}
	p.consentimiento++
	p.cancelarConsentimiento, p.respuestaManual = cancelar, nil
	return ctx, p.consentimiento, cancelar
}

func (p *Puente) terminarConsentimiento(n uint64, cancelar context.CancelFunc) {
	cancelar()
	p.mu.Lock()
	if p.consentimiento == n {
		p.cancelarConsentimiento, p.respuestaManual = nil, nil
	}
	p.mu.Unlock()
}

// responderConsentimiento envía consent al lector, solo si el pedido sigue siendo el vigente.
func (p *Puente) responderConsentimiento(n uint64, aprobado bool) {
	p.mu.Lock()
	vigente := p.consentimiento == n
	p.mu.Unlock()
	if !vigente {
		return
	}
	if err := p.enviar(protocolo.Consent(aprobado)); err != nil {
		p.log.Printf("Consentimiento: %v", err)
	}
}

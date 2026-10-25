package puente

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/platanus-hack/platanus-hack-26-ve-team-1/gateway/internal/api"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/gateway/internal/protocolo"
	"github.com/platanus-hack/platanus-hack-26-ve-team-1/gateway/internal/voz"
)

// Plazos del lector: escribir y releer un bloque toma milisegundos; sellar hasta 14
// sectores, alrededor de un segundo.
const (
	plazoAck     = 5 * time.Second
	plazoWritten = 30 * time.Second
)

// emitir conduce la emisión de una tarjeta virgen: pide los bloques firmados a la API,
// los envía uno a uno esperando cada ack y al final pide al lector sellar la tarjeta.
func (p *Puente) emitir(ctx context.Context, uid string) {
	for len(p.respuestas) > 0 {
		<-p.respuestas // respuestas de una emisión anterior
	}

	terminal, token := p.Activo()
	p.log.Printf("Emisión: tarjeta %s en %s; pidiendo los bloques firmados a la API", uid, terminal)
	em, err := p.api.EmisionStart(ctx, token, uid)
	if err != nil {
		p.log.Printf("Emisión cancelada: %v%s", err, pistaEmision(err))
		p.liberarLector(ctx)
		return
	}

	for i, b := range em.Blocks {
		err := p.enviar(protocolo.Write(b.S, b.B, b.D))
		if err == nil {
			var r protocolo.Mensaje
			r, err = p.esperar(ctx, "ack", plazoAck)
			if err == nil && (!r.OK || r.B != b.B || (r.S != 0 && r.S != b.S)) {
				err = fmt.Errorf("el lector no pudo escribir el bloque %d del sector %d", b.B, b.S)
			}
		}
		if err != nil {
			p.log.Printf("Emisión fallida en el bloque %d de %d: %v. La API ya registró la cédula %s: "+
				"pide otra emisión y vuelve a acercar la tarjeta", i+1, len(em.Blocks), err, em.H)
			p.liberarLector(ctx)
			return
		}
	}

	if err := p.enviar(protocolo.Seal()); err != nil {
		p.log.Printf("Emisión: %v", err)
		return
	}
	r, err := p.esperar(ctx, "written", plazoWritten)
	if err != nil || !r.OK {
		p.log.Printf("Emisión: los datos quedaron escritos pero la tarjeta no quedó sellada (%v); "+
			"pide otra emisión para reescribirla", motivo(err))
		return
	}
	p.log.Printf("Cédula emitida y sellada: handle %s, %d bloques", em.H, len(em.Blocks))
}

// liberarLector envía seal sin una emisión completa: el lector no sella nada, responde
// written con ok:false y vuelve a reposo sin esperar su timeout de 30 s.
func (p *Puente) liberarLector(ctx context.Context) {
	if p.enviar(protocolo.Seal()) == nil {
		p.esperar(ctx, "written", plazoAck)
	}
}

// esperar espera la próxima respuesta del lector de ese tipo.
func (p *Puente) esperar(ctx context.Context, tipo string, plazo time.Duration) (protocolo.Mensaje, error) {
	t := time.NewTimer(plazo)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return protocolo.Mensaje{}, ctx.Err()
		case <-t.C:
			return protocolo.Mensaje{}, fmt.Errorf("el lector no respondió %s en %s", tipo, plazo)
		case m := <-p.respuestas:
			if m.T == tipo {
				return m, nil
			}
		}
	}
}

func motivo(err error) string {
	if err != nil {
		return err.Error()
	}
	return "el lector respondió written con ok:false"
}

func pistaEmision(err error) string {
	switch api.Estado(err) {
	case http.StatusConflict:
		return ". Pide la emisión desde el panel del emisor (vale 10 min) y vuelve a acercar la tarjeta"
	case http.StatusForbidden:
		return ". El terminal activo no está en modo emitir: usa «terminal Registro»"
	case http.StatusBadRequest:
		return ". El UID debe tener 4, 7 o 10 bytes"
	}
	return ""
}

// vincular pide el código de 6 dígitos para la billetera y lo envía al lector.
func (p *Puente) vincular(ctx context.Context, h string) {
	_, token := p.Activo()
	c, err := p.api.LinkStart(ctx, token, h)
	if err != nil {
		p.log.Printf("Vinculación: %v%s", err, pistaVinculo(err))
		return
	}
	if err := p.enviar(protocolo.Code(c)); err != nil {
		p.log.Printf("Vinculación: %v", err)
		return
	}
	p.log.Printf("Código de vinculación para la billetera: %s (vale 2 minutos)", c)
	p.voz.Decir(voz.Vinculacion)
}

func pistaVinculo(err error) string {
	switch api.Estado(err) {
	case http.StatusForbidden:
		return ". El terminal activo no está en modo vincular (se cambia con PATCH /api/terminals/{id})"
	case http.StatusNotFound:
		return ". La API no conoce esa cédula"
	case http.StatusConflict:
		return ". La cédula está revocada"
	}
	return ""
}

// Package voz reproduce los audios del terminal (roadmap: gateway/audio/, generados una
// vez con ElevenLabs). Si falta la carpeta o un archivo, el gateway sigue sin voz.
package voz

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Nombres de los audios del roadmap (archivo .wav o .mp3 con ese nombre).
const (
	Concedido   = "acceso_concedido"
	Denegado    = "acceso_denegado"
	Revocada    = "cedula_revocada"
	Vencida     = "cedula_vencida"
	SinPermiso  = "sin_permiso"
	Emergencia  = "acceso_emergencia"
	Vinculacion = "vinculacion_lista"
	Bloqueado   = "terminal_bloqueado"
)

// ParaResultado elige el audio de un código de resultado del contrato.
func ParaResultado(res string, emg bool) string {
	switch res {
	case "01":
		if emg {
			return Emergencia
		}
		return Concedido
	case "03":
		return Revocada
	case "04":
		return Vencida
	case "05":
		return SinPermiso
	default:
		return Denegado
	}
}

type Voz struct {
	dir string
	mu  sync.Mutex
}

func Nueva(dir string) *Voz {
	return &Voz{dir: dir}
}

// Decir reproduce el audio en segundo plano, uno a la vez.
func (v *Voz) Decir(nombre string) {
	if v == nil || v.dir == "" {
		return
	}
	ruta := v.buscar(nombre)
	if ruta == "" {
		return
	}
	go func() {
		v.mu.Lock()
		defer v.mu.Unlock()
		_ = reproducir(ruta)
	}()
}

func (v *Voz) buscar(nombre string) string {
	for _, ext := range []string{".wav", ".mp3"} {
		ruta := filepath.Join(v.dir, nombre+ext)
		if _, err := os.Stat(ruta); err == nil {
			return ruta
		}
	}
	return ""
}

// reproducir usa los reproductores de Windows a través de PowerShell (la laptop del gateway).
func reproducir(ruta string) error {
	r := strings.ReplaceAll(ruta, "'", "''")
	var script string
	if strings.EqualFold(filepath.Ext(ruta), ".wav") {
		script = fmt.Sprintf("(New-Object Media.SoundPlayer '%s').PlaySync()", r)
	} else {
		script = fmt.Sprintf("Add-Type -AssemblyName PresentationCore; "+
			"$p = New-Object System.Windows.Media.MediaPlayer; $p.Open([uri]'%s'); $p.Play(); "+
			"$i = 0; while (-not $p.NaturalDuration.HasTimeSpan -and $i -lt 100) { Start-Sleep -Milliseconds 50; $i++ }; "+
			"if ($p.NaturalDuration.HasTimeSpan) { Start-Sleep -Milliseconds ([int]$p.NaturalDuration.TimeSpan.TotalMilliseconds) }", r)
	}
	return exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Run()
}

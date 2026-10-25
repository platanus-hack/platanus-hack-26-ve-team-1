# Gateway del terminal

Une el lector (ESP32 por USB) con la API de `p1-backend`. Corre en la laptop del terminal y sigue los enlaces 1 (lector ↔ gateway) y 4 (gateway ↔ API) de [`docs/contrato-terminal.md`](../docs/contrato-terminal.md).

## Qué hace

- **Sincroniza al lector.** Espera su primer latido y le envía `time`, `rev` (de a 2 handles) y `cfg`; cada 60 s repite `time` y `rev`, y `cfg` si cambió. Si el lector late con `"sync":false` (se reinició), le reenvía todo.
- **Funciona sin internet.** Guarda la última `cfg` de cada terminal y la última lista de revocadas en `data/`, y las usa si la API no responde.
- **No pierde eventos.** Cada `verif` y `tamper` se guarda en `data/cola/` antes de enviarse a `POST /api/events`, con el token del terminal que lo generó. Reintenta con espera creciente; lo que la API rechaza (4xx) queda en `data/cola/rechazados/`.
- **Emite cédulas.** `uid` → `POST /api/emision/start` → un `write` por bloque, esperando cada `ack` → `seal` → `written`. Si la API no tiene una emisión pendiente o falla un bloque, libera al lector con `seal` sin sellar nada.
- **Vincula billeteras.** `link` → `POST /api/link/start` → `code`.
- **Pide el consentimiento.** `pedido` (una lectura `01` en modo verificar) → `POST /api/consent`, que le pregunta al titular por WhatsApp → consulta `GET /api/consent/{id}` cada 1,5 s, hasta 65 s → `consent` con `approved` en true o false. Con 422 (titular sin WhatsApp o con avisos apagados) o sin red, le pide la respuesta al operador: `si` o `no` en la consola. Detalle en [`docs/consentimiento-whatsapp.md`](../docs/consentimiento-whatsapp.md).
- **Voz (opcional).** Si existe `audio/<nombre>.wav` o `.mp3` con los nombres del roadmap (`acceso_concedido`, `cedula_revocada`, `terminal_bloqueado`…), lo reproduce.

## Configurar

1. Guarda `secrets/demo.json` (lo genera P1 con `go -C api run ./cmd/admin demo`) en la raíz del repo. Está fuera de git.
2. Copia `config.example.json` a `config.local.json`, también fuera de git, y ajusta:
   - `puerto`: el COM del ESP32 según `pio device list`. En una laptop, COM10 puede ser un puerto Bluetooth.
   - `terminal`: el terminal activo al arrancar (`Bar La Esquina`, `Clinica Salud`, `Ambulancia 1`, `Transito` o `Registro`).

Los tokens también pueden ir en `tokens` (`{"nombre": "token"}`) y reemplazan a los de `tokens_demo`.

## Correr

```bash
cd gateway
go run ./cmd/gateway                                  # o bien:
go build -o gateway.exe ./cmd/gateway && ./gateway.exe -config config.local.json
```

Compilar requiere Go 1.26 o más nuevo; `gateway.exe` corre sin Go. Solo un programa puede tener abierto el puerto: cierra el monitor serie de PlatformIO antes de abrir el gateway. El gateway abre el puerto con DTR y RTS en bajo, porque activos reinician el ESP32.

## Comandos de la consola

| Comando | Qué hace |
|-|-|
| `terminales` | Lista los terminales con token (`*` = activo) |
| `terminal <n o nombre>` | Cambia el terminal activo; acepta el número, el nombre o el inicio del nombre (`terminal clin`) |
| `sync` | Reenvía `time`, `rev` y `cfg` al lector |
| `estado` | Lector, terminal activo y eventos en cola |
| `si` / `no` | Responde un pedido de consentimiento cuando la API no pudo preguntarle al titular por WhatsApp. Mientras se espera WhatsApp no hacen nada |
| `salir` | Cierra el gateway (también Ctrl+C) |

Un mismo lector hace de bar, clínica o ambulancia en la demo: al cambiar de terminal, el gateway usa su token y le envía su `cfg` al lector.

## Pruebas

```bash
go test ./...
```

Usan un lector falso y una API falsa con las rutas y errores de `p1-backend`, así que no necesitan hardware ni red.

## Archivos

| Ruta | Contenido |
|-|-|
| `config.local.json` | Configuración local (fuera de git) |
| `data/cola/` | Eventos por enviar; `rechazados/`, los que la API no aceptó |
| `data/*.json` | Última config de cada terminal y última lista de revocadas |
| `logs/` | Un registro por día |
| `audio/` | Audios de voz (opcional) |

# 05 · Arquitectura de software

## Nivel 1 · Contexto

```mermaid
flowchart TB
  Ciudadano([Ciudadano])
  Comercio([Comercio / Salud])
  Emisor([Emisor])
  SLJ[["sIsA<br/>(cédula, terminal, PWA, nube)"]]
  Kapso[(Kapso<br/>WhatsApp)]

  Ciudadano -->|cédula NFC, portal, billetera| SLJ
  Comercio -->|terminal, verificador| SLJ
  Emisor -->|panel /emisor| SLJ
  SLJ -->|avisos| Kapso
  Kapso -->|webhook BLOQUEAR| SLJ
```

## Nivel 2 · Contenedores

```mermaid
flowchart LR
  subgraph Campo[Terminal físico]
    Card[Tarjeta MIFARE 1K]
    FW[ESP32 + RC522<br/>firmware PlatformIO]
    UNO[UNO + ILI9486<br/>comercio]
    PIC[PIC16F13145<br/>guardián]
    Card <-->|NFC 13,56 MHz| FW
    FW -->|serial 9600, solo códigos| UNO
    FW <-->|HB / BLOQ| PIC
  end
  GW[Gateway Go<br/>cola de eventos en disco]
  FW <-->|USB serial JSON| GW
  subgraph Render
    Web[Static Site slj-web<br/>React + Vite + PWA]
    API[Web Service slj-api<br/>Go + chi]
    DB[(Postgres)]
    Web -->|rewrite /api/*| API
    API --> DB
  end
  GW -->|HTTPS Bearer| API
  API <-->|REST + webhook HMAC| K[(Kapso)]
  Phone[Celular<br/>PWA billetera] --> Web
  Verif[Celular comercio<br/>PWA verificador] --> Web
```

| Contenedor | Tecnología | Ubicación en el repo |
|---|---|---|
| Firmware del lector | C++ / PlatformIO, MFRC522, micro-ecc, ArduinoJson | `firmware/esp32`, `firmware/rc522-diag` |
| Guardián | PIC16F13145, MPLAB X, CLB | `firmware/pic` |
| Pantalla del comercio | Arduino UNO + MCUFRIEND_kbv | `firmware/uno` |
| Gateway | Go, go.bug.st/serial, cola de eventos en archivos | `gateway/` |
| API | Go (chi, pgx), JWT, bcrypt | `api/` |
| Base de datos | Postgres (migraciones SQL) | `db/migrations` |
| Front y PWAs | React, TypeScript, Vite, Tailwind, WebCrypto, IndexedDB | `web/` (núcleo QR en `web/src/lib/slj`) |
| Herramientas | Verificador independiente en Node, claves públicas | `tools/`, `keys/` |

## Nivel 3 · Componentes de la API

```mermaid
flowchart TB
  subgraph slj-api
    R[chi Router + CORS + sesión + roles]
    TA[terminal.Auth<br/>Bearer → SHA-256]
    AUTH[auth.go<br/>registro / login / JWT]
    EV[events.go<br/>verif y tamper]
    NO[notify.go<br/>useMessage + Notifier]
    HUB[history.go<br/>SSE + LISTEN/NOTIFY]
    EM[emision.go / cards.go<br/>firma y bloques]
    LK[link.go<br/>códigos de 6 dígitos]
    SG[internal/sign<br/>ECDSA P-256 crudo]
    KP[internal/kapso<br/>cliente + webhook]
  end
  R --> AUTH & HUB & EM & LK
  R --> TA --> EV
  EV -->|publishEvent| HUB
  EV --> NO --> KP
  EM --> SG
  KP -->|webhook BLOQUEAR| NO
```

## Decisiones de arquitectura (ADR resumidos)

| ADR | Decisión | Motivo | Consecuencia |
|---|---|---|---|
| 1 | ESP32 + RC522 en vez de Mega + PN532 | Disponibilidad del kit; ESP32 con más memoria para micro-ecc | Pines y alimentación documentados en `rc522-diag`; el RC522 solo lee MIFARE |
| 2 | Firma ECDSA P-256 `r‖s` de 64 B con el UID dentro del mensaje | Mismo algoritmo en Go, WebCrypto y micro-ecc; impide copiar un atributo a otra tarjeta | Vectores compartidos en `docs/test-vectors.json` |
| 3 | Handle seudónimo en vez de datos personales en eventos | El historial no revela identidad al terminal ni al comercio | La API une handle → ciudadano solo para el dueño |
| 4 | Verificación offline en el terminal | Sin dependencia de red en el punto de uso | Revocaciones sincronizadas cada 60 s por el gateway |
| 5 | Avisos en segundo plano (`goNotify`) | Un fallo o demora de Kapso no frena al terminal | El evento queda guardado aunque falle el aviso |
| 6 | Aviso en **todos** los resultados | El ciudadano detecta intentos denegados (copias, terminales sin permiso) | Más mensajes; respeta `notifications` por cuenta |
| 7 | Todo en Render (Go + Postgres + estático) | Un `render.yaml`, URL pública para el webhook | Arranque en frío del plan gratuito |
| 8 | Guardián PIC independiente | Bloqueo por hardware aunque el firmware sea comprometido | Requiere REARME tras cada grabación |

## Despliegue

`render.yaml` define `slj-api` (Go, `rootDir: api`, health `/api/health`) y `slj-web` (estático, rewrite `/api/*` hacia la API y `/*` a `index.html`). Variables en [08](08-despliegue-operacion.md).

## Mapa con el grafo de conocimiento (graphify)

Comunidades detectadas: utilidades binarias (`bytes.ts`), códec y criptografía del QR (`web/src/lib/slj`), CLI y pruebas en Go, almacén IndexedDB, verificación QR y laboratorio, despliegue Render. Nodos centrales: `verifyPresentation()`, `parsePresentation()`, `Presentacion QR SLJ1`. Tras la fusión se recomienda re-ejecutar `/graphify` para incorporar la API, el firmware y estos documentos.

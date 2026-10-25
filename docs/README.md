# Documentación de sIsA

Cédula electrónica que **sabe más y muestra menos**. Este directorio es la documentación del proyecto; la descripción corta está en [`../project-description.md`](../project-description.md).

| # | Documento | Para qué sirve |
|---|---|---|
| 01 | [Brief](01-brief.md) | Problema, propuesta, usuarios, alcance y métricas |
| 02 | [SRS](02-srs.md) | Requisitos funcionales y no funcionales, con trazabilidad |
| 03 | [Casos de uso](03-casos-de-uso.md) | Actores, diagrama y fichas de casos de uso |
| 04 | [Flujos](04-flujos.md) | Diagramas de flujo y de secuencia de cada proceso |
| 05 | [Arquitectura](05-arquitectura.md) | Contexto, contenedores, componentes, despliegue y decisiones |
| 06 | [Modelo de datos](06-modelo-datos.md) | Diagrama entidad-relación y retención |
| 07 | [Seguridad y privacidad](07-seguridad-privacidad.md) | Amenazas, controles y minimización de datos |
| 08 | [Despliegue y operación](08-despliegue-operacion.md) | Variables, Render, datos demo y runbook |
| 09 | [Pruebas](09-pruebas.md) | Estrategia, matriz y checklist con hardware |

Fuentes de verdad técnicas (no se duplican aquí):

- [`contrato.md`](contrato.md): bytes de la cédula, firma, QR, API y avisos.
- [`contrato-terminal.md`](contrato-terminal.md): serial lector↔gateway, lector↔PIC, lector↔pantalla, gateway↔API.
- [`roadmap.md`](roadmap.md): plan de 36 h y reparto del equipo.
- [`p4-qr.md`](p4-qr.md) (rama `p4-pwa`): formato de presentación QR `SLJ1`.

Los diagramas usan [Mermaid](https://mermaid.js.org/); GitHub los renderiza directamente.

## Formatos y diagramas

| Qué | Dónde |
|---|---|
| Documentos Word (uno por documento y uno completo, con los diagramas incrustados) | [`word/`](word/) |
| Diagramas como imagen (`.png`, `.svg`) y su fuente (`.mmd`) | [`diagramas/`](diagramas/) |
| Fuente editable de todo | los `.md` de esta carpeta (los diagramas son bloques ```` ```mermaid ````) |

**Regenerar** (desde la raíz del repo; requiere Python con `python-docx` y Node.js):

```bash
python tools/docs/build_docs.py
```

**Editar o rehacer un diagrama con una herramienta visual:** abre el archivo `.mmd` de `diagramas/` (o el bloque Mermaid del `.md`) en [mermaid.live](https://mermaid.live) para editarlo y exportar PNG/SVG/PDF; en [draw.io](https://app.diagrams.net) usa *Organizar → Insertar → Avanzado → Mermaid*; en Figma/FigJam se puede generar el diagrama editable a partir del mismo texto. Los SVG se abren en Figma, draw.io o Illustrator.

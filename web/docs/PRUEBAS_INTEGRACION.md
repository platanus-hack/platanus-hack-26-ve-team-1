# Pruebas de integración en vivo

Checklist para probar el frontend contra el backend real desplegado en otra instancia. Compila con `VITE_USE_MOCK` **sin definir** (o en `false`) y `VITE_API_URL` apuntando al backend: así toda la capa `src/api` llama a `${VITE_API_URL}/api/*` (cross-origin, con credenciales). Antes de empezar, en DevTools > Network comprueba que las peticiones van a la URL del backend y que no hay errores de CORS en la consola.

> Durante las pruebas abre la consola del navegador: en modo real el frontend **no** escribe logs con datos de eventos, de perfil ni de cédulas. Si ves alguno, es un bug.

## a) SSE entre la CDN y el backend

1. Inicia sesión como ciudadano y abre `/` > pestaña **Historial**.
2. Confirma que el indicador muestra **En vivo**.
3. Deja la pestaña abierta **2 minutos** (que haya pasado al menos un keep-alive de 25 s).
4. Genera un evento nuevo (por ejemplo, una lectura desde un terminal) y comprueba que aparece arriba **sin recargar**.
5. Si el evento no aparece hasta cerrar la conexión o recargar, es **buffering** en algún proxy:
   - Pide al backend las cabeceras `Content-Type: text/event-stream`, `Cache-Control: no-cache` y `X-Accel-Buffering: no`.
   - Revisa los proxies entre el navegador y el backend (balanceador, CDN del backend) para que no comprimen ni almacenan la respuesta del stream.
   - Comprueba que la respuesta del stream trae `Access-Control-Allow-Origin` (origen exacto del front) y `Access-Control-Allow-Credentials: true`.
6. Con DevTools > Network, filtra por `events/stream`: debe quedar una sola conexión abierta de tipo `eventsource`. Si cae, el frontend reconecta solo (1 s, 2 s, 4 s… máximo 15 s) y el indicador vuelve a **En vivo**.

## b) Cookie de sesión con `credentials: "include"`

1. Abre DevTools > Network y inicia sesión.
2. En la respuesta de `POST /api/auth/login`, revisa `Set-Cookie`: debe ser **HttpOnly** y **Secure**, con **SameSite=Lax** si front y back comparten dominio registrable, o **SameSite=None** si son dominios distintos. Revisa también que la respuesta traiga `Access-Control-Allow-Origin` (origen exacto del front) y `Access-Control-Allow-Credentials: true`, y que el *preflight* `OPTIONS` responda bien.
3. Comprueba que `GET /api/me` **envía la cookie** (pestaña Cookies de la petición o cabecera `Cookie`). Si el navegador la bloquea (cookie de terceros), el login funciona pero `GET /api/me` responde 401 y la sesión no se mantiene: pon front y back bajo el mismo dominio.
4. **Recarga** la página (F5): la sesión se mantiene y vuelves a tu pantalla sin pasar por `/login`.
5. Cierra sesión: `POST /api/auth/logout` borra la cookie y vuelves a `/login`.
6. En Application > Storage verifica que **no** hay tokens en `localStorage` ni en `sessionStorage`.

## c) Emisión de una cuenta nueva

1. Registra una cuenta nueva en `/registro` (ciudadano) y cierra sesión.
2. Entra como emisor y abre `/emisor` > **Cédulas**. La cuenta nueva aparece **Sin emitir**, con el botón "Emitir cédula" deshabilitado y el texto "Falta fecha de nacimiento y cédula".
3. Pulsa **Completar perfil**, ingresa fecha de nacimiento y número de cédula, y guarda. La fila se actualiza sin recargar y "Emitir cédula" se habilita. La fecha y la cédula no se muestran en la tabla.
4. Pulsa **Emitir cédula**, confirma y acerca una tarjeta virgen al terminal en modo emisión. Espera a que el panel diga que terminó (consulta el estado cada 2 s; se rinde a los 2 minutos).
5. Cierra sesión y entra con la cuenta nueva: en `/` > **Mi cédula** debe verse **Cédula activa** con sus datos.

## Datos opcionales (límite de 16 bytes)

1. En `/` > **Datos opcionales**, escribe alergias de más de 16 bytes (por ejemplo `penicilina y maní`): el contador marca rojo, aparece "Máximo 16 bytes (te pasaste por N)…" y el botón no guarda.
2. Repite con nombre y teléfono de contacto que, juntos, pasen de 16 bytes.
3. Prueba con tildes y emojis: `á`, `ñ` y los emojis cuentan más de 1 byte.
4. Si el backend responde `400` por este motivo, el mensaje sale en un toast y lo escrito se conserva.

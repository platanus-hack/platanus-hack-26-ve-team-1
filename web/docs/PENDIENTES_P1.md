# Pendientes con P1 (backend)

Endpoints y formas JSON que el frontend de sIsA necesita y que **no están en el contrato** acordado. Hay que confirmarlos o ajustar el frontend.

> Las rutas de auth están definidas en `AUTH_ENDPOINTS` (`api/auth.ts`). Todas son relativas a la base `/api` y se llaman con `credentials: "include"` (JWT en cookie HttpOnly).

## Endpoints por confirmar

| Método | Ruta (con `/api`) | Uso | Respuesta esperada |
|--------|-------------------|-----|--------------------|
| `GET` | `/api/me` | Restaurar la sesión al cargar la app. | `200` con un `User`; `401` si no hay sesión. |
| `POST` | `/api/auth/logout` | Cerrar sesión (el backend debe borrar la cookie). | `204` sin cuerpo, o `200`. |

Ya están en el contrato: `POST /api/auth/register` y `POST /api/auth/login` (ambos responden con un `User`).

## Forma JSON de `User`

Respuesta de `register`, `login` y `GET /me`:

```json
{
  "id": "u-123",
  "email": "ana@sisa.demo",
  "nombre": "Ana Pérez",
  "rol": "ciudadano",
  "telefono": "+56 9 1111 1111",
  "avisosActivados": true
}
```

- `id`: string.
- `rol`: exactamente `"ciudadano"`, `"emisor"` o `"comercio"` (minúscula, sin tilde).
- `telefono`: string o `null` si no hay.
- `avisosActivados`: booleano.

## Errores

El frontend lee el mensaje de `{ "error": "..." }` o `{ "message": "..." }` y lo muestra al usuario, por lo que debe venir **en español**. Un `401` en cualquier endpoint (salvo `login`) cierra la sesión localmente.

| Caso | Estado esperado |
|------|-----------------|
| Credenciales incorrectas en `login` | `401` |
| Correo ya registrado en `register` | `409` |
| Datos inválidos (p. ej. contraseña corta) | `400` |

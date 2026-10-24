# Pendientes con P1 (backend)

Endpoints y formas JSON que el frontend de sIsA necesita y que **no están en el contrato** acordado. Hay que confirmarlos o ajustar el frontend.

> Las rutas de auth están definidas en `AUTH_ENDPOINTS` (`api/auth.ts`). Todas son relativas a la base `/api` y se llaman con `credentials: "include"` (JWT en cookie HttpOnly).

## Endpoints por confirmar

| Método | Ruta (con `/api`) | Uso | Respuesta esperada |
|--------|-------------------|-----|--------------------|
| `GET` | `/api/me` | Restaurar la sesión al cargar la app. | `200` con un `User`; `401` si no hay sesión. |
| `POST` | `/api/auth/logout` | Cerrar sesión (el backend debe borrar la cookie). | `204` sin cuerpo, o `200`. |
| `GET` | `/api/card` | Cédula del usuario autenticado (portal ciudadano). Ruta en `CARD_ENDPOINTS` (`api/card.ts`). | `200` con un `CardInfo`; `404` si aún no tiene cédula. |
| `GET` | `/api/me/optional-data` | Precargar el formulario "Mis datos opcionales". Ruta en `OPTIONAL_DATA_ENDPOINTS` (`api/optionalData.ts`). | `200` con un `OptionalData` (valores vacíos si no hay datos guardados). |

Ya están en el contrato: `POST /api/auth/register`, `POST /api/auth/login` (ambos responden con un `User`) y `PUT /api/me/optional-data` (recibe y devuelve un `OptionalData`).

## Forma JSON de `OptionalData`

```json
{
  "tipoSangre": "O+",
  "alergias": "penicilina, maní",
  "contactoEmergencia": { "nombre": "Rosa Pérez", "telefono": "+56912345678" },
  "donante": true
}
```

- `tipoSangre`: `"A+"`, `"A-"`, `"B+"`, `"B-"`, `"AB+"`, `"AB-"`, `"O+"`, `"O-"` o `null`.
- `alergias`: string de hasta 200 caracteres (puede ser vacío).
- `contactoEmergencia.telefono`: formato internacional sin espacios (`+` y 8 a 15 dígitos), o vacío si no hay contacto. Nombre y teléfono van juntos o ambos vacíos.
- `donante`: booleano.

## Forma JSON de `CardInfo`

```json
{
  "id": "c-ana",
  "estado": "activa",
  "emitidaEn": "2026-03-12T10:00:00Z",
  "atributos": ["mayor_de_18", "mayor_de_21", "identidad", "sangre_alergias", "contacto_emergencia", "donante_licencia"]
}
```

- `estado`: `"activa"` o `"revocada"`. Una cédula revocada devuelve `atributos: []`.
- `atributos`: valores exactos (minúscula, con guion bajo) de la lista del ejemplo.

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

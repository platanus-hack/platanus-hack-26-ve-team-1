import { ApiError, emitUnauthorized } from './client'
import type {
  Attribute,
  CardInfo,
  Citizen,
  LoginInput,
  NotificationSettings,
  OptionalData,
  RegisterInput,
  Terminal,
  User,
} from './types'
import { EMPTY_OPTIONAL_DATA } from '../lib/optionalData'

// Datos de demostración: las contraseñas en texto plano solo existen en el mock.
type MockAccount = User & { password: string; edad: number }

const SESSION_KEY = 'sisa:mock-session'
const REGISTERED_KEY = 'sisa:mock-registered'
const MIN_PASSWORD_LENGTH = 8

const CHIP_ATTRIBUTES: Attribute[] = [
  'mayor_de_18',
  'mayor_de_21',
  'identidad',
  'sangre_alergias',
  'contacto_emergencia',
  'donante_licencia',
]
const demoAccounts: MockAccount[] = [
  {
    id: 'u-ana',
    email: 'ana@sisa.demo',
    nombre: 'Ana Pérez',
    rol: 'ciudadano',
    telefono: '+56 9 1111 1111',
    avisosActivados: true,
    password: 'demo1234',
    edad: 24,
  },
  {
    id: 'u-luis',
    email: 'luis@sisa.demo',
    nombre: 'Luis Gómez',
    rol: 'ciudadano',
    telefono: '+56 9 2222 2222',
    avisosActivados: false,
    password: 'demo1234',
    edad: 16,
  },
  {
    id: 'u-maria',
    email: 'maria@sisa.demo',
    nombre: 'María Rojas',
    rol: 'ciudadano',
    telefono: null,
    avisosActivados: true,
    password: 'demo1234',
    edad: 35,
  },
  {
    id: 'u-emisor',
    email: 'emisor@sisa.demo',
    nombre: 'Registro Civil',
    rol: 'emisor',
    telefono: null,
    avisosActivados: false,
    password: 'demo1234',
    edad: 40,
  },
  {
    id: 'u-comercio',
    email: 'comercio@sisa.demo',
    nombre: 'Bar El Faro',
    rol: 'comercio',
    telefono: null,
    avisosActivados: false,
    password: 'demo1234',
    edad: 40,
  },
]

export const mockCards: Record<string, CardInfo> = {
  'u-ana': {
    id: 'c-ana',
    estado: 'activa',
    emitidaEn: '2026-03-12T10:00:00Z',
    atributos: [...CHIP_ATTRIBUTES],
  },
  'u-luis': {
    id: 'c-luis',
    estado: 'activa',
    emitidaEn: '2026-05-02T09:30:00Z',
    atributos: [...CHIP_ATTRIBUTES],
  },
  // Una cédula revocada ya no expone ningún atributo.
  'u-maria': {
    id: 'c-maria',
    estado: 'revocada',
    emitidaEn: '2025-11-20T15:45:00Z',
    atributos: [],
  },
}

export const mockCitizens: Citizen[] = demoAccounts
  .filter((account) => account.rol === 'ciudadano')
  .map(({ id, nombre, email, edad }) => ({
    id,
    nombre,
    email,
    edad,
    cedula: mockCards[id]?.estado ?? 'revocada',
  }))

// Formato del backend: id numérico, atributo "A1"–"A6", resultado 1–6, emergencia booleana.
type MockBackendEvent = {
  id: number
  local: string
  atributo: string
  resultado: number
  hora: string
  emergencia: boolean
}

function backendEvent(
  id: number,
  local: string,
  atributo: string,
  resultado: number,
  hora: string,
  emergencia = false,
): MockBackendEvent {
  return { id, local, atributo, resultado, hora, emergencia }
}

// Del más reciente al más antiguo, como responde el backend.
const mockEventsByUser: Record<string, MockBackendEvent[]> = {
  'u-ana': [
    backendEvent(104, 'Botillería Central', 'A2', 6, '2026-10-24T10:20:00Z'),
    backendEvent(103, 'Hotel Costanera', 'A3', 4, '2026-10-24T09:30:00Z'),
    backendEvent(102, 'Bar El Faro', 'A1', 1, '2026-10-23T22:15:00Z'),
    backendEvent(101, 'Hospital Central', 'A4', 1, '2026-10-22T14:05:00Z', true),
  ],
  'u-luis': [
    backendEvent(202, 'Banco Andes', 'A3', 5, '2026-10-24T11:10:00Z'),
    backendEvent(201, 'Botillería Central', 'A1', 5, '2026-10-24T01:40:00Z'),
  ],
  'u-maria': [backendEvent(301, 'Supermercado Norte', 'A1', 3, '2026-10-23T18:20:00Z')],
}

const MOCK_PLACES = [
  'Bar El Faro',
  'Botillería Central',
  'Hotel Costanera',
  'Banco Andes',
  'Farmacia Sur',
  'Estación de Metro',
]

// Atributos que lee un comercio: A1 Mayor de 18, A2 Mayor de 21, A3 Identidad.
const MOCK_COMMERCE_ATTRIBUTES = ['A1', 'A2', 'A3'] as const

let nextMockEventId = 1000

function pick<T>(items: readonly T[]): T {
  return items[Math.floor(Math.random() * items.length)]
}

function randomMockEvent(userId: string): MockBackendEvent {
  const id = nextMockEventId++
  const hora = new Date().toISOString()
  // Ana recibe a veces una lectura de emergencia; los demás reflejan el estado de su cédula.
  if (userId === 'u-ana' && Math.random() < 0.2) {
    return backendEvent(id, 'Hospital Central', 'A4', 1, hora, true)
  }
  const resultado =
    userId === 'u-ana' ? 1 : userId === 'u-luis' ? 5 : userId === 'u-maria' ? 3 : 2
  return backendEvent(id, pick(MOCK_PLACES), pick(MOCK_COMMERCE_ATTRIBUTES), resultado, hora)
}

export const mockTerminals: Terminal[] = [
  {
    id: 't-1',
    nombre: 'Caja principal',
    rol: 'comercio',
    atributo: 'mayor_de_18',
  },
]

function delay(): Promise<void> {
  const ms = 300 + Math.random() * 300
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function toUser({ password: _password, edad: _edad, ...user }: MockAccount): User {
  return user
}

function loadRegisteredUsers(): User[] {
  try {
    const raw = sessionStorage.getItem(REGISTERED_KEY)
    const data: unknown = raw ? JSON.parse(raw) : []
    return Array.isArray(data) ? (data as User[]) : []
  } catch {
    return []
  }
}

function saveRegisteredUser(user: User): void {
  sessionStorage.setItem(
    REGISTERED_KEY,
    JSON.stringify([...loadRegisteredUsers(), user]),
  )
}

function findUserById(id: string | null): User | undefined {
  const demo = demoAccounts.find((account) => account.id === id)
  const user = demo ? toUser(demo) : loadRegisteredUsers().find((u) => u.id === id)
  return user && withNotificationOverride(user)
}

// Los cambios de WhatsApp y de bloqueo viven solo en sessionStorage (por usuario).
function notificationsKey(userId: string): string {
  return `sisa:mock-notifications:${userId}`
}

function revokedKey(userId: string): string {
  return `sisa:mock-revoked:${userId}`
}

function withNotificationOverride(user: User): User {
  try {
    const raw = sessionStorage.getItem(notificationsKey(user.id))
    return raw ? { ...user, ...(JSON.parse(raw) as NotificationSettings) } : user
  } catch {
    return user
  }
}

function emailTaken(email: string): boolean {
  return (
    demoAccounts.some((account) => account.email === email) ||
    loadRegisteredUsers().some((user) => user.email === email)
  )
}

export async function mockLogin({ email, password }: LoginInput): Promise<User> {
  await delay()
  const normalized = email.trim().toLowerCase()
  const demo = demoAccounts.find((account) => account.email === normalized)
  if (demo) {
    if (demo.password !== password) throw invalidCredentials()
    sessionStorage.setItem(SESSION_KEY, demo.id)
    return withNotificationOverride(toUser(demo))
  }
  // Las cuentas registradas no guardan contraseña: el mock acepta cualquiera de 8+ caracteres.
  const registered = loadRegisteredUsers().find((user) => user.email === normalized)
  if (!registered || password.length < MIN_PASSWORD_LENGTH) {
    throw invalidCredentials()
  }
  sessionStorage.setItem(SESSION_KEY, registered.id)
  return withNotificationOverride(registered)
}

function invalidCredentials(): ApiError {
  return new ApiError(401, 'Correo o contraseña incorrectos.')
}

export async function mockRegister(input: RegisterInput): Promise<User> {
  await delay()
  const email = input.email.trim().toLowerCase()
  if (emailTaken(email)) {
    throw new ApiError(409, 'Ya existe una cuenta con este correo.')
  }
  if (input.password.length < MIN_PASSWORD_LENGTH) {
    throw new ApiError(400, 'La contraseña debe tener al menos 8 caracteres.')
  }
  const user: User = {
    id: `u-${crypto.randomUUID()}`,
    email,
    nombre: input.nombre.trim(),
    rol: 'ciudadano',
    telefono: input.telefono?.trim() || null,
    avisosActivados: false,
  }
  saveRegisteredUser(user)
  sessionStorage.setItem(SESSION_KEY, user.id)
  return user
}

export async function mockLogout(): Promise<void> {
  await delay()
  sessionStorage.removeItem(SESSION_KEY)
}

export async function mockGetMe(): Promise<User> {
  await delay()
  const user = findUserById(sessionStorage.getItem(SESSION_KEY))
  if (!user) {
    emitUnauthorized()
    throw new ApiError(401, 'Tu sesión no es válida. Inicia sesión de nuevo.')
  }
  return user
}

export async function mockGetCardInfo(): Promise<CardInfo> {
  await delay()
  const user = findUserById(sessionStorage.getItem(SESSION_KEY))
  if (!user) {
    emitUnauthorized()
    throw new ApiError(401, 'Tu sesión no es válida. Inicia sesión de nuevo.')
  }
  const card = mockCards[user.id]
  if (!card) throw new ApiError(404, 'Aún no tienes una cédula emitida.')
  const revoked = sessionStorage.getItem(revokedKey(user.id)) === '1'
  return revoked ? { ...card, estado: 'revocada', atributos: [] } : card
}

function requireSessionUser(): User {
  const user = findUserById(sessionStorage.getItem(SESSION_KEY))
  if (!user) {
    emitUnauthorized()
    throw new ApiError(401, 'Tu sesión no es válida. Inicia sesión de nuevo.')
  }
  return user
}

function optionalDataKey(userId: string): string {
  return `sisa:mock-optional:${userId}`
}

export async function mockGetOptionalData(): Promise<OptionalData> {
  await delay()
  const user = requireSessionUser()
  try {
    const raw = sessionStorage.getItem(optionalDataKey(user.id))
    return raw ? (JSON.parse(raw) as OptionalData) : EMPTY_OPTIONAL_DATA
  } catch {
    return EMPTY_OPTIONAL_DATA
  }
}

export async function mockSaveOptionalData(data: OptionalData): Promise<OptionalData> {
  await delay()
  const user = requireSessionUser()
  sessionStorage.setItem(optionalDataKey(user.id), JSON.stringify(data))
  return data
}

/** Respuesta de GET /api/events: { ok, events }, del más reciente al más antiguo. */
export async function mockGetEvents(): Promise<{ ok: true; events: MockBackendEvent[] }> {
  await delay()
  const user = requireSessionUser()
  return { ok: true, events: mockEventsByUser[user.id] ?? [] }
}

const MOCK_STREAM_MIN_MS = 4000
const MOCK_STREAM_JITTER_MS = 3000

/**
 * Simula el stream SSE: abre tras un instante y entrega cada evento nuevo como el `data`
 * (texto JSON) del evento "evento", igual que el EventSource real.
 */
export function mockOpenEventStream(handlers: {
  onOpen: () => void
  onMessage: (data: string) => void
}): () => void {
  const user = findUserById(sessionStorage.getItem(SESSION_KEY))
  let timer: ReturnType<typeof setTimeout> | undefined

  const schedule = () => {
    timer = setTimeout(() => {
      if (user) handlers.onMessage(JSON.stringify(randomMockEvent(user.id)))
      schedule()
    }, MOCK_STREAM_MIN_MS + Math.random() * MOCK_STREAM_JITTER_MS)
  }

  timer = setTimeout(() => {
    handlers.onOpen()
    schedule()
  }, 400)

  return () => clearTimeout(timer)
}

export async function mockRevokeCard(): Promise<void> {
  await delay()
  const user = requireSessionUser()
  if (!mockCards[user.id]) throw new ApiError(404, 'Aún no tienes una cédula emitida.')
  sessionStorage.setItem(revokedKey(user.id), '1')
}

export async function mockSaveNotifications(
  settings: NotificationSettings,
): Promise<NotificationSettings> {
  await delay()
  const user = requireSessionUser()
  sessionStorage.setItem(notificationsKey(user.id), JSON.stringify(settings))
  return settings
}

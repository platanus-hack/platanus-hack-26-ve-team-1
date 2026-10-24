import { ApiError, emitUnauthorized } from './client'
import type {
  Attribute,
  CardInfo,
  CardStatus,
  Citizen,
  EmissionStatus,
  LoginInput,
  NotificationSettings,
  OptionalData,
  RegisterInput,
  Student,
  Terminal,
  TerminalInput,
  TerminalStats,
  User,
} from './types'
import { EMPTY_OPTIONAL_DATA } from '../lib/optionalData'
import { ROLES } from '../lib/roles'

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

// Los estados iniciales salen de mockCards; los cambios (bloquear, emitir) se guardan por usuario.

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

// Recorre todas las combinaciones lugar + dato en orden aleatorio antes de repetir ninguna,
// para que los eventos simulados no se vean como filas idénticas.
let mockComboQueue: Array<{ local: string; atributo: string }> = []

function nextMockCombo(): { local: string; atributo: string } {
  if (mockComboQueue.length === 0) {
    mockComboQueue = MOCK_PLACES.flatMap((local) =>
      MOCK_COMMERCE_ATTRIBUTES.map((atributo) => ({ local, atributo })),
    ).sort(() => Math.random() - 0.5)
  }
  return mockComboQueue.pop()!
}

let mockStreamCount = 0

function randomMockEvent(userId: string): MockBackendEvent {
  const id = nextMockEventId++
  const hora = new Date().toISOString()
  mockStreamCount += 1
  // Ana recibe una lectura de emergencia cada 6 eventos; los demás reflejan el estado de su cédula.
  if (userId === 'u-ana' && mockStreamCount % 6 === 0) {
    return backendEvent(id, 'Hospital Central', 'A4', 1, hora, true)
  }
  const resultado =
    userId === 'u-ana' ? 1 : userId === 'u-luis' ? 5 : userId === 'u-maria' ? 3 : 2
  const { local, atributo } = nextMockCombo()
  return backendEvent(id, local, atributo, resultado, hora)
}

const MOCK_SEED_TERMINALS: Terminal[] = [
  { id: 't-1', nombre: 'Caja principal', rol: 'comercio', atributos: ['mayor_de_18'] },
  {
    id: 't-2',
    nombre: 'Ficha clínica',
    rol: 'salud',
    atributos: ['sangre_alergias', 'contacto_emergencia', 'donante_licencia'],
  },
]

// Los terminales del mock viven en sessionStorage (por usuario) y nunca guardan el token.
function terminalsKey(userId: string): string {
  return `sisa:mock-terminals:${userId}`
}

function loadTerminals(userId: string): Terminal[] {
  try {
    const raw = sessionStorage.getItem(terminalsKey(userId))
    return raw ? (JSON.parse(raw) as Terminal[]) : MOCK_SEED_TERMINALS
  } catch {
    return MOCK_SEED_TERMINALS
  }
}

function saveTerminals(userId: string, terminals: Terminal[]): void {
  sessionStorage.setItem(terminalsKey(userId), JSON.stringify(terminals))
}

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

// Los cambios de WhatsApp y de estado de la cédula viven solo en sessionStorage (por usuario).
function notificationsKey(userId: string): string {
  return `sisa:mock-notifications:${userId}`
}

/** Estado de la cédula tras bloquear o emitir; si no existe, vale el de mockCards. */
type CardOverride = { estado: CardStatus; emitidaEn?: string }

function cardOverrideKey(userId: string): string {
  return `sisa:mock-card:${userId}`
}

function readCardOverride(userId: string): CardOverride | undefined {
  try {
    const raw = sessionStorage.getItem(cardOverrideKey(userId))
    return raw ? (JSON.parse(raw) as CardOverride) : undefined
  } catch {
    return undefined
  }
}

function writeCardOverride(userId: string, override: CardOverride): void {
  sessionStorage.setItem(cardOverrideKey(userId), JSON.stringify(override))
}

/** Cédula vigente del usuario en el mock (base + cambios), o undefined si aún no tiene. */
function currentCard(userId: string): CardInfo | undefined {
  const base = mockCards[userId]
  const override = readCardOverride(userId)
  if (!override) return base
  return {
    id: base?.id ?? `c-${userId}`,
    estado: override.estado,
    emitidaEn: override.emitidaEn ?? base?.emitidaEn ?? new Date().toISOString(),
    atributos: override.estado === 'activa' ? [...CHIP_ATTRIBUTES] : [],
  }
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
  const card = currentCard(user.id)
  if (!card) throw new ApiError(404, 'Aún no tienes una cédula emitida.')
  return card
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

function requireEmisor(): User {
  const user = requireSessionUser()
  if (user.rol !== ROLES.EMISOR) {
    throw new ApiError(403, 'No tienes permiso para hacer esto.')
  }
  return user
}

/** Sin `citizenId`, el ciudadano bloquea su propia cédula; con él, lo hace el emisor. */
export async function mockRevokeCard(citizenId?: string): Promise<void> {
  await delay()
  let targetId: string
  if (citizenId === undefined) {
    targetId = requireSessionUser().id
  } else {
    requireEmisor()
    targetId = citizenId
  }
  const card = currentCard(targetId)
  if (!card) throw new ApiError(404, 'Ese ciudadano aún no tiene una cédula emitida.')
  writeCardOverride(targetId, { estado: 'revocada', emitidaEn: card.emitidaEn })
}

export async function mockGetCitizens(): Promise<Citizen[]> {
  await delay()
  requireEmisor()
  const citizens: User[] = [
    ...demoAccounts.filter((account) => account.rol === ROLES.CIUDADANO).map(toUser),
    ...loadRegisteredUsers().filter((user) => user.rol === ROLES.CIUDADANO),
  ]
  return citizens.map(({ id, nombre, email }) => {
    const card = currentCard(id)
    return {
      id,
      nombre,
      email,
      estadoCedula: card?.estado ?? 'sin_emitir',
      emitidaEn: card?.emitidaEn ?? null,
    }
  })
}

const MOCK_EMISSION_DURATION_MS = 7000

function requireComercio(): User {
  const user = requireSessionUser()
  if (user.rol !== ROLES.COMERCIO) {
    throw new ApiError(403, 'No tienes permiso para hacer esto.')
  }
  return user
}

function validateTerminalInput(input: TerminalInput): void {
  if (!input.nombre.trim()) throw new ApiError(400, 'El nombre del terminal es obligatorio.')
  if (input.atributos.length === 0) {
    throw new ApiError(400, 'Elige el dato que pide el terminal.')
  }
}

export async function mockGetTerminals(): Promise<Terminal[]> {
  await delay()
  return loadTerminals(requireComercio().id)
}

/** Devuelve el token en claro una sola vez; no se guarda en ningún lado. */
export async function mockCreateTerminal(input: TerminalInput): Promise<Terminal> {
  await delay()
  const user = requireComercio()
  validateTerminalInput(input)
  const terminal: Terminal = {
    id: `t-${crypto.randomUUID().slice(0, 8)}`,
    nombre: input.nombre.trim(),
    rol: input.rol,
    atributos: input.atributos,
  }
  saveTerminals(user.id, [...loadTerminals(user.id), terminal])
  const token = `sisa_demo_${crypto.randomUUID().replaceAll('-', '')}`
  return { ...terminal, token }
}

export async function mockUpdateTerminal(id: string, input: TerminalInput): Promise<Terminal> {
  await delay()
  const user = requireComercio()
  validateTerminalInput(input)
  const terminals = loadTerminals(user.id)
  const current = terminals.find((terminal) => terminal.id === id)
  if (!current) throw new ApiError(404, 'No encontramos ese terminal.')
  const updated: Terminal = {
    id,
    nombre: input.nombre.trim(),
    rol: input.rol,
    atributos: input.atributos,
  }
  saveTerminals(
    user.id,
    terminals.map((terminal) => (terminal.id === id ? updated : terminal)),
  )
  return updated
}

export async function mockDeleteTerminal(id: string): Promise<void> {
  await delay()
  const user = requireComercio()
  const terminals = loadTerminals(user.id)
  if (!terminals.some((terminal) => terminal.id === id)) {
    throw new ApiError(404, 'No encontramos ese terminal.')
  }
  saveTerminals(
    user.id,
    terminals.filter((terminal) => terminal.id !== id),
  )
}

/** Totales inventados pero estables por terminal (derivados del id). */
export async function mockGetStats(): Promise<TerminalStats[]> {
  await delay()
  return loadTerminals(requireComercio().id).map((terminal) => {
    const seed = [...terminal.id].reduce((sum, char) => sum + char.charCodeAt(0), 0)
    const aprobadas = 12 + (seed % 30)
    const rechazadas = 1 + (seed % 7)
    return { terminalId: terminal.id, hoy: 5 + (seed % 9), aprobadas, rechazadas }
  })
}

// Carnets universitarios del mock: Luis ya tiene uno; el resto se emite en sessionStorage.
const MOCK_STUDENT_BASE_ISSUED: Record<string, string> = {
  'u-luis': '2026-06-01T12:00:00Z',
}

function studentKey(studentId: string): string {
  return `sisa:mock-student:${studentId}`
}

function studentIssuedAt(studentId: string): string | null {
  return sessionStorage.getItem(studentKey(studentId)) ?? MOCK_STUDENT_BASE_ISSUED[studentId] ?? null
}

export async function mockGetStudents(): Promise<Student[]> {
  await delay()
  requireEmisor()
  return demoAccounts
    .filter((account) => account.rol === ROLES.CIUDADANO)
    .map(({ id, nombre, email }) => {
      const emitidoEn = studentIssuedAt(id)
      return {
        id,
        nombre,
        email,
        estadoCarnet: emitidoEn ? 'emitido' : 'sin_carnet',
        emitidoEn,
      }
    })
}

export async function mockIssueUniversityCredential(studentId: string): Promise<void> {
  await delay()
  requireEmisor()
  if (!demoAccounts.some((account) => account.id === studentId)) {
    throw new ApiError(404, 'No encontramos a ese estudiante.')
  }
  if (studentIssuedAt(studentId)) {
    throw new ApiError(409, 'Ese estudiante ya tiene un carnet universitario.')
  }
  sessionStorage.setItem(studentKey(studentId), new Date().toISOString())
}

let mockEmission: { citizenId: string; startedAt: number } | null = null

/** Inicia una emisión simulada: el terminal "lee" la tarjeta a los pocos segundos. */
export async function mockOrderEmission(citizenId: string): Promise<void> {
  await delay()
  requireEmisor()
  mockEmission = { citizenId, startedAt: Date.now() }
}

export async function mockGetEmissionStatus(): Promise<EmissionStatus> {
  await delay()
  requireEmisor()
  if (!mockEmission) {
    return { estado: 'error', mensaje: 'No hay una emisión en curso.' }
  }
  if (Date.now() - mockEmission.startedAt < MOCK_EMISSION_DURATION_MS) {
    return { estado: 'pendiente' }
  }
  writeCardOverride(mockEmission.citizenId, {
    estado: 'activa',
    emitidaEn: new Date().toISOString(),
  })
  mockEmission = null
  return { estado: 'completado' }
}

export async function mockSaveNotifications(
  settings: NotificationSettings,
): Promise<NotificationSettings> {
  await delay()
  const user = requireSessionUser()
  sessionStorage.setItem(notificationsKey(user.id), JSON.stringify(settings))
  return settings
}

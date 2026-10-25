interface ImportMetaEnv {
  /** "true" para usar datos de demostración en lugar del backend. */
  readonly VITE_USE_MOCK?: 'true' | 'false'
  /** Origen del backend, p. ej. "https://api.sisa.example.com" (sin "/api" ni barra final). */
  readonly VITE_API_URL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

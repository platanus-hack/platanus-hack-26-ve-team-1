interface ImportMetaEnv {
  /** "true" para usar datos de demostración en lugar del backend. */
  readonly VITE_USE_MOCK?: 'true' | 'false'
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

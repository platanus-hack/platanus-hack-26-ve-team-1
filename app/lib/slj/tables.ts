// Constantes del contrato (docs/contrato.md) y propuestas de P4 (docs/p4-qr.md).

export const ISSUER = { NACIONAL: 0x01, UNIVERSIDAD: 0x02 } as const;

export const ISSUER_NAME: Record<number, string> = {
  1: "Autoridad nacional (simulada)",
  2: "Universidad (simulada)",
};

export const ATTR = {
  MAYOR18: 1,
  MAYOR21: 2,
  IDENTIDAD: 3,
  SANGRE: 4,
  EMERGENCIA: 5,
  LICENCIA: 6,
  CARNET: 7,
} as const;

export const ATTR_LABEL: Record<number, string> = {
  1: "Mayor de 18",
  2: "Mayor de 21",
  3: "Identidad",
  4: "Tipo de sangre y alergias",
  5: "Contacto de emergencia",
  6: "Licencia de conducir",
  7: "Carnet universitario",
};

// Códigos de resultado: 01-05 vienen del contrato; 07 y 08 son propuesta de P4.
export const RESULT = {
  OK: 1,
  BAD_SIG: 2,
  REVOKED: 3,
  EXPIRED: 4,
  NO_PERMISSION: 5,
  STALE: 7,
  BAD_FORMAT: 8,
} as const;

export type ResultCode = (typeof RESULT)[keyof typeof RESULT];

export const VER_CHIP = 0x01;
export const VER_WALLET = 0x02;
export const PREFIX = "SLJ1:";
export const TAG = new Uint8Array([0x53, 0x4c, 0x4a, 0x31]); // "SLJ1"

export const ROTATE_SECONDS = 20; // el QR se regenera cada 20 s
export const WINDOW_SECONDS = 30; // el verificador acepta ±30 s

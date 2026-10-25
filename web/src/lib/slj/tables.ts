// Constantes del contrato de P1 (docs/contrato.md en la rama p1-backend). P4 no define formatos propios:
// solo implementa el QR, que el contrato le asigna.

export const ISSUER = { NACIONAL: 0x01, UNIVERSIDAD: 0x02 } as const;

export const ISSUER_NAME: Record<number, string> = {
  1: "Autoridad nacional (simulada)",
  2: "Universidad (simulada)",
};

// Atributos del contrato (A1..A6). El carnet universitario NO es un atributo nuevo:
// es A3 (Identidad) firmado por el emisor 0x02, solo en la billetera.
export const ATTR = {
  MAYOR18: 1,
  MAYOR21: 2,
  IDENTIDAD: 3,
  SANGRE: 4,
  EMERGENCIA: 5,
  DONANTE: 6,
} as const;

export const ATTR_LABEL: Record<number, string> = {
  1: "Mayor de 18",
  2: "Mayor de 21",
  3: "Identidad",
  4: "Tipo de sangre y alergias",
  5: "Contacto de emergencia",
  6: "Donante y licencia",
};

/** Nombre para mostrar: A3 firmado por la universidad es el carnet. */
export function attrLabel(attr: number, issuer: number): string {
  if (attr === ATTR.IDENTIDAD && issuer === ISSUER.UNIVERSIDAD) return "Carnet universitario";
  return ATTR_LABEL[attr] ?? `Atributo ${attr}`;
}

// Códigos del contrato: 01..06. Los terminales y la API solo conocen estos.
// 07 y 08 son INTERNOS de la pantalla del verificador: nunca se envían a la API ni al UNO.
export const RESULT = {
  OK: 1,
  BAD_SIG: 2,
  REVOKED: 3,
  EXPIRED: 4,
  NO_PERMISSION: 5,
  STALE: 7, // interno: QR caducado o ya usado
  BAD_FORMAT: 8, // interno: no es un QR SLJ1 válido o el emisor no se conoce
} as const;

export type ResultCode = (typeof RESULT)[keyof typeof RESULT];

export const VER = 0x01; // versión de la cabecera (contrato)
export const PREFIX = "SLJ1:";

export const ROTATE_SECONDS = 20; // el QR se regenera cada 20 s
export const WINDOW_SECONDS = 30; // el verificador acepta ±30 s (contrato)

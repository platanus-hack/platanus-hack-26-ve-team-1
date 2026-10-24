import { RESULT } from "./tables.ts";
import type { VerifyResult } from "./verify.ts";

/** Cuerpo de `POST /api/events` (contrato de P1): `{t:"verif",h,a,res,ts,emg}`. */
export interface VerifEvent {
  t: "verif";
  h: string; // handle, 16 caracteres hex
  a: string; // "A1".."A6"
  res: string; // "01".."06"
  ts: number; // hora Unix en segundos
  emg: boolean;
}

/**
 * Evento que se envía a la API, o null si no debe enviarse.
 * Solo se envían resultados que la API conoce (01–05) y que ya pasaron las firmas, así el historial del
 * ciudadano nunca contiene lecturas atribuidas a un QR falsificado. 02 (firma inválida: el handle no es de
 * fiar), 07 y 08 (internos de la pantalla del verificador) no se envían.
 */
export function toVerifEvent(r: VerifyResult, ts: number): VerifEvent | null {
  const ok =
    r.code === RESULT.OK || r.code === RESULT.REVOKED || r.code === RESULT.EXPIRED || r.code === RESULT.NO_PERMISSION;
  if (!ok || r.handle === undefined || r.attr === undefined || r.attr < 1 || r.attr > 6) return null;
  return {
    t: "verif",
    h: r.handle.toString(16).padStart(16, "0"),
    a: `A${r.attr}`,
    res: String(r.code).padStart(2, "0"),
    ts,
    emg: false,
  };
}

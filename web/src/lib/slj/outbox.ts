import { drainOutbox, pushOutbox } from "./store.ts";

/** Resultado de intentar enviar un evento: enviado, descartado (la API lo rechaza siempre) o a reintentar. */
export type SendResult = "sent" | "drop" | "retry";

export interface FlushSummary {
  sent: number;
  dropped: number;
  pending: number;
}

/**
 * Vacía el outbox y envía los eventos en orden. Si uno falla de forma transitoria se detiene y devuelve ese
 * evento y los siguientes al outbox, para no perder nada ni reordenar de más. Un evento que la API rechaza
 * por inválido (`drop`) se descarta: reintentarlo para siempre bloquearía la cola.
 */
export async function flushOutbox(send: (event: unknown) => Promise<SendResult>): Promise<FlushSummary> {
  const batch = await drainOutbox();
  let sent = 0;
  let dropped = 0;
  for (let i = 0; i < batch.length; i++) {
    let result: SendResult;
    try {
      result = await send(batch[i]);
    } catch {
      result = "retry";
    }
    if (result === "sent") sent++;
    else if (result === "drop") dropped++;
    else {
      for (const event of batch.slice(i)) await pushOutbox(event);
      return { sent, dropped, pending: batch.length - i };
    }
  }
  return { sent, dropped, pending: 0 };
}

import { useMemo } from 'react'
import { encode } from 'uqr'
import { matrixToPath } from '../../lib/slj/qrPath.ts'

type QrCodeProps = {
  value: string
  /** Texto para lectores de pantalla: describe qué es, nunca el contenido. */
  label: string
}

/** QR como un solo trazo SVG: sin imágenes ni HTML inyectado, y nítido a cualquier tamaño. */
export function QrCode({ value, label }: QrCodeProps) {
  const { d, size } = useMemo(() => matrixToPath(encode(value, { ecc: 'M', border: 0 }).data), [value])
  return (
    <svg
      viewBox={`0 0 ${size} ${size}`}
      role="img"
      aria-label={label}
      shapeRendering="crispEdges"
      className="w-full max-w-sm rounded-xl border border-neutral-200"
    >
      <rect width={size} height={size} fill="#fff" />
      <path d={d} fill="#000" />
    </svg>
  )
}

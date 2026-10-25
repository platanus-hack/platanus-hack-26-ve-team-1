/**
 * Convierte la matriz de módulos de un QR en un único trazo SVG (una pasada por filas, con tramos
 * horizontales fusionados). Evita `dangerouslySetInnerHTML` y un nodo por módulo.
 * `border` es la zona de silencio en módulos (el estándar pide 4).
 */
export function matrixToPath(matrix: boolean[][], border = 4): { d: string; size: number } {
  const n = matrix.length;
  let d = "";
  for (let y = 0; y < n; y++) {
    const row = matrix[y];
    let x = 0;
    while (x < n) {
      if (!row[x]) {
        x++;
        continue;
      }
      const start = x;
      while (x < n && row[x]) x++;
      d += `M${start + border} ${y + border}h${x - start}v1h-${x - start}z`;
    }
  }
  return { d, size: n + border * 2 };
}

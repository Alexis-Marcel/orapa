import type { Catalog, Layout, Point } from "../api/types";
import { cellsOf, pieceDef } from "./pieces";

/**
 * Découpe un trajet en segments, chacun avec les couleurs déjà prises par
 * l'onde : à chaque sommet, la gemme touchée est celle de la case où l'onde
 * arrivait.
 */
export function coloredSegments(cat: Catalog, layout: Layout, path: Point[]) {
  const colorAt = new Map<string, number>();
  for (const p of layout) {
    const c = pieceDef(cat, p.kind).color;
    for (const cell of cellsOf(cat, p)) colorAt.set(`${cell.x},${cell.y}`, c);
  }
  const out: { points: Point[]; colors: number; length: number }[] = [];
  let colors = 0;
  for (let i = 0; i + 1 < path.length; i++) {
    const a = path[i];
    const b = path[i + 1];
    if (i > 0) {
      const prev = path[i - 1];
      const dx = Math.sign(a.x - prev.x);
      const dy = Math.sign(a.y - prev.y);
      colors |= colorAt.get(`${Math.floor(a.x + dx * 0.01)},${Math.floor(a.y + dy * 0.01)}`) ?? 0;
    }
    out.push({ points: [a, b], colors, length: Math.abs(b.x - a.x) + Math.abs(b.y - a.y) });
  }
  return out;
}

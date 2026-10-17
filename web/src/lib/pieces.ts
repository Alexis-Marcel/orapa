import type { Catalog, Fill, Kind, Layout, Orientation, PieceDef, Placement, Point } from "../api/types";

export const W = 10;
export const H = 8;
export const ENTRIES = 2 * (W + H);

export const KINDS: Kind[] = ["white-triangle", "blue-triangle", "yellow-triangle", "white-diamond", "red-parallelogram"];

export function pieceDef(cat: Catalog, kind: Kind): PieceDef {
  const d = cat.pieces.find((p) => p.kind === kind);
  if (!d) throw new Error(`gemme inconnue ${kind}`);
  return d;
}

export function orientationOf(cat: Catalog, p: Placement): Orientation {
  return pieceDef(cat, p.kind).orientations[p.o];
}

/** Cases occupées par une gemme posée. */
export function cellsOf(cat: Catalog, p: Placement) {
  return orientationOf(cat, p).cells.map((c) => ({ x: p.x + c.x, y: p.y + c.y, fill: c.fill }));
}

export function outlineOf(cat: Catalog, p: Placement): Point[] {
  return orientationOf(cat, p).outline.map((q) => ({ x: p.x + q.x, y: p.y + q.y }));
}

export function fits(cat: Catalog, p: Placement): boolean {
  const o = orientationOf(cat, p);
  return p.x >= 0 && p.y >= 0 && p.x + o.w <= W && p.y + o.h <= H;
}

/** Gemme d'une pose qui occupe la case (x, y), -1 sinon. */
export function pieceAt(cat: Catalog, layout: Layout, x: number, y: number): number {
  return layout.findIndex((p) => cellsOf(cat, p).some((c) => c.x === x && c.y === y));
}

export function overlaps(cat: Catalog, layout: Layout, p: Placement): boolean {
  const taken = new Set(layout.flatMap((q) => cellsOf(cat, q).map((c) => `${c.x},${c.y}`)));
  return cellsOf(cat, p).some((c) => taken.has(`${c.x},${c.y}`));
}

// Rotation et miroir, comme le moteur : on retrouve l'orientation du catalogue.
const ROT: Record<number, Fill> = { 1: 1, 2: 3, 3: 4, 4: 5, 5: 2 };
const MIR: Record<number, Fill> = { 1: 1, 2: 5, 5: 2, 3: 4, 4: 3 };

function key(o: { cells: { x: number; y: number; fill: Fill }[] }) {
  return [...o.cells]
    .sort((a, b) => a.y - b.y || a.x - b.x)
    .map((c) => `${c.x},${c.y},${c.fill}`)
    .join(";");
}

function find(def: PieceDef, cells: { x: number; y: number; fill: Fill }[]): number {
  const k = key({ cells });
  return def.orientations.findIndex((o) => key(o) === k);
}

export function rotated(def: PieceDef, o: number): number {
  const src = def.orientations[o];
  const i = find(def, src.cells.map((c) => ({ x: src.h - 1 - c.y, y: c.x, fill: ROT[c.fill] })));
  return i < 0 ? o : i;
}

export function flipped(def: PieceDef, o: number): number {
  const src = def.orientations[o];
  const i = find(def, src.cells.map((c) => ({ x: src.w - 1 - c.x, y: c.y, fill: MIR[c.fill] })));
  return i < 0 ? o : i;
}

// Points d'entrée : haut 1–10, droite 11–18, bas A–J, gauche K–R.
export function entryLabel(e: number): string {
  return e < W + H ? String(e + 1) : String.fromCharCode(65 + e - W - H);
}

/** Centre du repère d'un point d'entrée, en unités de case (la mine va de 0 à W). */
export function entryPos(e: number): Point {
  if (e < W) return { x: e + 0.5, y: -0.5 };
  if (e < W + H) return { x: W + 0.5, y: e - W + 0.5 };
  if (e < 2 * W + H) return { x: e - W - H + 0.5, y: H + 0.5 };
  return { x: -0.5, y: e - 2 * W - H + 0.5 };
}

export function cellName(x: number, y: number): string {
  return `${String.fromCharCode(97 + x)}${y + 1}`;
}

/** Ancre (coin haut gauche) qui centre une gemme sur la case visée, sans sortir de la mine. */
export function anchorFor(o: Orientation, x: number, y: number): { x: number; y: number } {
  return {
    x: Math.min(Math.max(x - Math.floor((o.w - 1) / 2), 0), W - o.w),
    y: Math.min(Math.max(y - Math.floor((o.h - 1) / 2), 0), H - o.h),
  };
}

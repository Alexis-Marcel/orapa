import { useRef, type PointerEvent } from "react";
import type { Catalog, Layout, Placement, Point } from "../api/types";
import { GEM_HEX, waveHex } from "../lib/colors";
import { ENTRIES, H, W, entryLabel, entryPos, outlineOf, pieceDef } from "../lib/pieces";

// La mine en SVG, en unités de case : la grille va de (0,0) à (W,H). Autour,
// un anneau de résultats (pastilles des ondes) puis un anneau de repères.

export interface EdgeMark {
  n: number; // numéro de l'onde
  colors: number;
  absorbed?: boolean;
  back?: boolean; // ressortie par son entrée
}

export interface CellMark {
  kind: "empty" | "probe";
  color?: number;
}

export interface WavePath {
  points: Point[];
  colors: number;
  strong?: boolean;
  dim?: boolean;
  /** Tracé animé : début et durée en millisecondes. */
  draw?: { delay: number; duration: number };
}

interface Props {
  cat: Catalog;
  gems?: Layout;
  ghost?: Layout; // gemmes en pointillé (proposition superposée à la vraie pose)
  preview?: { p: Placement; ok: boolean } | null;
  marks?: Map<number, EdgeMark>;
  cellMarks?: Map<string, CellMark>;
  paths?: WavePath[];
  selectedEntry?: number | null;
  selectedCell?: { x: number; y: number } | null;
  hotEntries?: Set<number>;
  entryDisabled?: (e: number) => boolean;
  onEntry?: (e: number) => void;
  onCell?: (x: number, y: number) => void;
  onHover?: (cell: { x: number; y: number } | null) => void;
  label: string;
  compact?: boolean;
  /** Illustration : une image pour les lecteurs d'écran, sans le détail. */
  decorative?: boolean;
}

const RING = 0.55; // centre des pastilles, hors de la mine
const LABEL = 1.25; // centre des repères

function ringPos(e: number, d: number): Point {
  const p = entryPos(e); // à 0,5 du bord
  const k = d / 0.5;
  if (p.y < 0) return { x: p.x, y: -0.5 * k };
  if (p.x > W) return { x: W + 0.5 * k, y: p.y };
  if (p.y > H) return { x: p.x, y: H + 0.5 * k };
  return { x: -0.5 * k, y: p.y };
}

function textOn(hex: string) {
  const n = parseInt(hex.slice(1), 16);
  const l = 0.299 * (n >> 16) + 0.587 * ((n >> 8) & 255) + 0.114 * (n & 255);
  return l > 140 ? "#1b1d2e" : "#f4f4f8";
}

function Gem({ cat, p, mode }: { cat: Catalog; p: Placement; mode: "solid" | "ghost" | "ok" | "bad" }) {
  const def = pieceDef(cat, p.kind);
  const pts = outlineOf(cat, p);
  const cx = pts.reduce((a, q) => a + q.x, 0) / pts.length;
  const cy = pts.reduce((a, q) => a + q.y, 0) / pts.length;
  const inner = pts.map((q) => `${cx + (q.x - cx) * 0.55},${cy + (q.y - cy) * 0.55}`).join(" ");
  const outer = pts.map((q) => `${q.x},${q.y}`).join(" ");
  const fill = GEM_HEX[def.color];
  if (mode === "ghost") {
    return <polygon points={outer} className="gem-ghost" style={{ stroke: fill }} />;
  }
  return (
    <g className={`gem gem-${mode}`}>
      <polygon points={outer} fill={fill} className="gem-body" />
      {/* Facettes : un éclat au centre et des arêtes vers les sommets. */}
      <polygon points={inner} className="gem-table" />
      {pts.map((q, i) => (
        <line key={i} x1={q.x} y1={q.y} x2={cx + (q.x - cx) * 0.55} y2={cy + (q.y - cy) * 0.55} className="gem-facet" />
      ))}
    </g>
  );
}

export function MineBoard(props: Props) {
  const { cat, gems = [], ghost = [], preview, marks, cellMarks, paths = [], onEntry, onCell, onHover } = props;
  const svg = useRef<SVGSVGElement>(null);
  const M = 1.75;

  const cellFromEvent = (ev: PointerEvent): { x: number; y: number } | null => {
    const el = svg.current;
    const ctm = el?.getScreenCTM();
    if (!el || !ctm) return null;
    const pt = new DOMPoint(ev.clientX, ev.clientY).matrixTransform(ctm.inverse());
    const x = Math.floor(pt.x);
    const y = Math.floor(pt.y);
    return x >= 0 && x < W && y >= 0 && y < H ? { x, y } : null;
  };

  return (
    <svg
      ref={svg}
      className={`mine ${props.compact ? "mine-compact" : ""} ${onCell ? "mine-interactive" : ""}`}
      viewBox={`${-M} ${-M} ${W + 2 * M} ${H + 2 * M}`}
      role={props.decorative ? "img" : "group"}
      aria-label={props.label}
      onPointerLeave={() => onHover?.(null)}
    >
      <rect x={0} y={0} width={W} height={H} className="mine-floor" rx={0.08} />
      {Array.from({ length: W - 1 }, (_, i) => (
        <line key={`v${i}`} x1={i + 1} y1={0} x2={i + 1} y2={H} className="mine-line" />
      ))}
      {Array.from({ length: H - 1 }, (_, i) => (
        <line key={`h${i}`} x1={0} y1={i + 1} x2={W} y2={i + 1} className="mine-line" />
      ))}

      {gems.map((p) => (
        <Gem key={p.kind} cat={cat} p={p} mode="solid" />
      ))}
      {ghost.map((p) => (
        <Gem key={`g-${p.kind}`} cat={cat} p={p} mode="ghost" />
      ))}
      {preview && <Gem cat={cat} p={preview.p} mode={preview.ok ? "ok" : "bad"} />}

      {cellMarks &&
        [...cellMarks].map(([k, m]) => {
          const [x, y] = k.split(",").map(Number);
          if (m.kind === "empty") {
            return <circle key={k} cx={x + 0.5} cy={y + 0.5} r={0.07} className="cell-empty" />;
          }
          return (
            <g key={k} className="cell-probe">
              <rect x={x + 0.12} y={y + 0.12} width={0.76} height={0.76} rx={0.1} className="cell-probe-frame" />
              {m.color ? (
                <circle cx={x + 0.5} cy={y + 0.5} r={0.17} fill={GEM_HEX[m.color]} className="cell-probe-dot" />
              ) : (
                <path d={`M${x + 0.32} ${y + 0.32} L${x + 0.68} ${y + 0.68} M${x + 0.68} ${y + 0.32} L${x + 0.32} ${y + 0.68}`} className="cell-probe-none" />
              )}
            </g>
          );
        })}

      {paths.map((w, i) => (
        <polyline
          key={i}
          points={w.points.map((q) => `${q.x},${q.y}`).join(" ")}
          pathLength={w.draw ? 1 : undefined}
          className={`wave-path ${w.strong ? "wave-path-strong" : ""} ${w.dim ? "wave-path-dim" : ""} ${w.colors ? "" : "wave-path-clear"} ${w.draw ? "wave-path-draw" : ""}`}
          style={{
            stroke: w.colors ? waveHex(w.colors) : undefined,
            animationDelay: w.draw ? `${w.draw.delay}ms` : undefined,
            animationDuration: w.draw ? `${w.draw.duration}ms` : undefined,
          }}
        />
      ))}

      {props.selectedCell && (
        <rect x={props.selectedCell.x + 0.04} y={props.selectedCell.y + 0.04} width={0.92} height={0.92} className="cell-selected" rx={0.08} />
      )}

      {onCell && (
        <rect
          x={0}
          y={0}
          width={W}
          height={H}
          className="mine-hit"
          onPointerMove={(ev) => onHover?.(cellFromEvent(ev))}
          onPointerDown={(ev) => {
            const c = cellFromEvent(ev);
            if (c) {
              onHover?.(c);
              onCell(c.x, c.y);
            }
          }}
          onContextMenu={(ev) => ev.preventDefault()}
        />
      )}

      {Array.from({ length: ENTRIES }, (_, e) => {
        const r = ringPos(e, RING);
        const l = ringPos(e, LABEL);
        const mark = marks?.get(e);
        const disabled = !onEntry || props.entryDisabled?.(e);
        const hex = mark ? (mark.absorbed ? "#101118" : waveHex(mark.colors)) : "";
        const cls = [
          "entry",
          mark ? "entry-marked" : "",
          props.selectedEntry === e ? "entry-selected" : "",
          props.hotEntries?.has(e) ? "entry-hot" : "",
          disabled ? "entry-disabled" : "",
          mark && !mark.colors && !mark.absorbed ? "entry-clear" : "",
        ].join(" ");
        return (
          <g
            key={e}
            className={cls}
            role={disabled ? undefined : "button"}
            tabIndex={disabled ? undefined : 0}
            aria-label={`Point ${entryLabel(e)}${mark ? `, onde ${mark.n}` : ""}`}
            onClick={disabled ? undefined : () => onEntry?.(e)}
            onKeyDown={
              disabled
                ? undefined
                : (ev) => {
                    if (ev.key === "Enter" || ev.key === " ") {
                      ev.preventDefault();
                      onEntry?.(e);
                    }
                  }
            }
          >
            <circle cx={r.x} cy={r.y} r={0.36} className="entry-disc" style={mark ? { fill: hex } : undefined} />
            {mark && (
              <text x={r.x} y={r.y} className="entry-n" style={{ fill: mark.colors || mark.absorbed ? textOn(hex) : undefined }}>
                {mark.absorbed ? "×" : mark.n}
              </text>
            )}
            {mark?.back && <circle cx={r.x} cy={r.y} r={0.44} className="entry-back" />}
            <text x={l.x} y={l.y} className="entry-label">
              {entryLabel(e)}
            </text>
          </g>
        );
      })}
    </svg>
  );
}

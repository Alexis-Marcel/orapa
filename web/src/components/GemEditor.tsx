import { useCallback, useEffect, useMemo, useState } from "react";
import type { Catalog, Kind, Layout, Placement } from "../api/types";
import { GEM_HEX } from "../lib/colors";
import { KINDS, anchorFor, fits, flipped, overlaps, pieceAt, pieceDef, rotated } from "../lib/pieces";

// Poser des gemmes à la main : on prend une gemme dans la réserve (ou sur la
// mine), on la tourne (R), on la retourne (F), on clique pour la poser.

export interface Held {
  kind: Kind;
  o: number;
}

export function useGemEditor(cat: Catalog | null, initial: Layout, storageKey?: string) {
  const [layout, setLayout] = useState<Layout>(() => {
    if (storageKey) {
      try {
        const saved = localStorage.getItem(storageKey);
        if (saved) return JSON.parse(saved).layout ?? initial;
      } catch {
        /* rien */
      }
    }
    return initial;
  });
  const [empties, setEmpties] = useState<Set<string>>(() => {
    if (storageKey) {
      try {
        const saved = localStorage.getItem(storageKey);
        if (saved) return new Set(JSON.parse(saved).empties ?? []);
      } catch {
        /* rien */
      }
    }
    return new Set();
  });
  const [held, setHeld] = useState<Held | null>(null);
  const [hover, setHover] = useState<{ x: number; y: number } | null>(null);
  const [markMode, setMarkMode] = useState(false);

  useEffect(() => {
    if (!storageKey) return;
    try {
      localStorage.setItem(storageKey, JSON.stringify({ layout, empties: [...empties] }));
    } catch {
      /* rien */
    }
  }, [storageKey, layout, empties]);

  const preview = useMemo(() => {
    if (!cat || !held || !hover) return null;
    const o = pieceDef(cat, held.kind).orientations[held.o];
    const a = anchorFor(o, hover.x, hover.y);
    const p: Placement = { kind: held.kind, o: held.o, ...a };
    return { p, ok: fits(cat, p) && !overlaps(cat, layout, p) };
  }, [cat, held, hover, layout]);

  const take = useCallback(
    (kind: Kind) => {
      setMarkMode(false);
      const placed = layout.find((p) => p.kind === kind);
      if (placed) setLayout((l) => l.filter((p) => p.kind !== kind));
      setHeld((h) => (h?.kind === kind && !placed ? null : { kind, o: placed?.o ?? 0 }));
    },
    [layout],
  );

  const rotate = useCallback(() => {
    if (cat) setHeld((h) => (h ? { ...h, o: rotated(pieceDef(cat, h.kind), h.o) } : h));
  }, [cat]);

  const flip = useCallback(() => {
    if (cat) setHeld((h) => (h ? { ...h, o: flipped(pieceDef(cat, h.kind), h.o) } : h));
  }, [cat]);

  const click = useCallback(
    (x: number, y: number) => {
      if (!cat) return;
      if (markMode) {
        const k = `${x},${y}`;
        setEmpties((s) => {
          const n = new Set(s);
          if (n.has(k)) n.delete(k);
          else n.add(k);
          return n;
        });
        return;
      }
      if (held) {
        const o = pieceDef(cat, held.kind).orientations[held.o];
        const p: Placement = { kind: held.kind, o: held.o, ...anchorFor(o, x, y) };
        if (fits(cat, p) && !overlaps(cat, layout, p)) {
          setLayout((l) => [...l.filter((q) => q.kind !== p.kind), p]);
          setHeld(null);
        }
        return;
      }
      const i = pieceAt(cat, layout, x, y);
      if (i >= 0) {
        const p = layout[i];
        setLayout((l) => l.filter((_, j) => j !== i));
        setHeld({ kind: p.kind, o: p.o });
      }
    },
    [cat, held, layout, markMode],
  );

  const clear = useCallback(() => {
    setLayout([]);
    setEmpties(new Set());
    setHeld(null);
  }, []);

  return { layout, setLayout, empties, held, setHeld, hover, setHover, preview, take, rotate, flip, click, clear, markMode, setMarkMode };
}

export type GemEditorState = ReturnType<typeof useGemEditor>;

/** Raccourcis clavier : R tourner, F retourner, Échap reposer. */
export function useEditorKeys(ed: GemEditorState | null, active: boolean) {
  useEffect(() => {
    if (!ed || !active) return;
    const onKey = (ev: KeyboardEvent) => {
      if (ev.target instanceof HTMLInputElement || ev.target instanceof HTMLTextAreaElement) return;
      if (ev.key === "r" || ev.key === "R") ed.rotate();
      else if (ev.key === "f" || ev.key === "F") ed.flip();
      else if (ev.key === "Escape") ed.setHeld(null);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [ed, active]);
}

function MiniGem({ cat, kind, o }: { cat: Catalog; kind: Kind; o: number }) {
  const def = pieceDef(cat, kind);
  const or = def.orientations[o];
  const pts = or.outline.map((q) => `${q.x},${q.y}`).join(" ");
  return (
    <svg viewBox={`-0.15 -0.15 ${4.3} ${4.3}`} className="mini-gem" aria-hidden>
      <g transform={`translate(${(4 - or.w) / 2} ${(4 - or.h) / 2})`}>
        <polygon points={pts} fill={GEM_HEX[def.color]} className="gem-body" />
      </g>
    </svg>
  );
}

export function GemPalette({ cat, ed, showMarks }: { cat: Catalog; ed: GemEditorState; showMarks?: boolean }) {
  return (
    <div className="palette">
      <div className="palette-gems" role="toolbar" aria-label="Gemmes">
        {KINDS.map((k) => {
          const def = pieceDef(cat, k);
          const placed = ed.layout.some((p) => p.kind === k);
          const isHeld = ed.held?.kind === k;
          return (
            <button
              key={k}
              type="button"
              className={`palette-gem ${placed ? "is-placed" : ""} ${isHeld ? "is-held" : ""}`}
              onClick={() => ed.take(k)}
              aria-pressed={isHeld}
              title={placed ? `Reprendre le ${def.name}` : `Prendre le ${def.name}`}
            >
              <MiniGem cat={cat} kind={k} o={isHeld ? ed.held!.o : (ed.layout.find((p) => p.kind === k)?.o ?? 0)} />
              <span>{def.name}</span>
            </button>
          );
        })}
      </div>
      <div className="palette-tools">
        <button type="button" className="tool" onClick={ed.rotate} disabled={!ed.held} title="Raccourci : R">
          Tourner
        </button>
        <button
          type="button"
          className="tool"
          onClick={ed.flip}
          disabled={!ed.held || ed.held.kind !== "red-parallelogram"}
          title="Raccourci : F"
        >
          Retourner
        </button>
        {showMarks && (
          <button
            type="button"
            className={`tool ${ed.markMode ? "is-on" : ""}`}
            aria-pressed={ed.markMode}
            onClick={() => {
              ed.setHeld(null);
              ed.setMarkMode(!ed.markMode);
            }}
          >
            Marquer vide
          </button>
        )}
        <button type="button" className="tool tool-quiet" onClick={ed.clear} disabled={ed.layout.length === 0 && ed.empties.size === 0}>
          Tout enlever
        </button>
      </div>
    </div>
  );
}

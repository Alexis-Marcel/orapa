import { useCallback, useEffect, useMemo, useRef, useState, type PointerEvent as ReactPointerEvent } from "react";
import type { Catalog, Kind, Layout, Orientation, Placement } from "../api/types";
import { GEM_HEX } from "../lib/colors";
import { H, KINDS, W, anchorFor, fits, flipped, overlaps, pieceAt, pieceDef, rotated } from "../lib/pieces";

// Poser des gemmes à la main. On glisse une gemme de la réserve vers la mine
// (ou d'une case à l'autre), ou on la prend d'un clic puis on clique pour la
// poser. R tourne, F retourne, même pendant un glissé. En mode « Marquer vide »,
// un glissé peint une ligne de cases (Maj : tracé libre).

export interface Held {
  kind: Kind;
  o: number;
}

type Cell = { x: number; y: number };

type Gesture =
  // Bouton enfoncé, pas encore bougé : un clic si on relâche sur place.
  | { t: "press"; id: number; sx: number; sy: number; kind: Kind; from: Placement | null; grip: Cell | null }
  | { t: "drag"; id: number; origin: Placement | null }
  | { t: "paint"; id: number; add: boolean; start: Cell; last: Cell; axis: "row" | "col" | "free" | null; base: Set<string>; done: Set<string> };

const THRESHOLD = 5; // px avant qu'un appui devienne un glissé

function clampAnchor(o: Orientation, x: number, y: number): Cell {
  return { x: Math.min(Math.max(x, 0), W - o.w), y: Math.min(Math.max(y, 0), H - o.h) };
}

function loadSaved(storageKey: string | undefined): { layout?: Layout; empties?: string[] } {
  if (!storageKey) return {};
  try {
    return JSON.parse(localStorage.getItem(storageKey) ?? "{}");
  } catch {
    return {};
  }
}

export function useGemEditor(cat: Catalog | null, initial: Layout, storageKey?: string) {
  const [layout, setLayout] = useState<Layout>(() => loadSaved(storageKey).layout ?? initial);
  const [empties, setEmpties] = useState<Set<string>>(() => new Set(loadSaved(storageKey).empties ?? []));
  const [held, setHeld] = useState<Held | null>(null);
  const [hover, setHover] = useState<Cell | null>(null);
  const [markMode, setMarkMode] = useState(false);
  // Case de la gemme tenue sous le pointeur, pour qu'elle ne saute pas quand on la déplace.
  const [grip, setGrip] = useState<Cell | null>(null);
  // Position du pointeur quand la gemme glissée est hors de la mine.
  const [float, setFloat] = useState<{ x: number; y: number } | null>(null);
  const [dragging, setDragging] = useState(false);
  const board = useRef<SVGSVGElement | null>(null);
  const gesture = useRef<Gesture | null>(null);

  useEffect(() => {
    if (!storageKey) return;
    try {
      localStorage.setItem(storageKey, JSON.stringify({ layout, empties: [...empties] }));
    } catch {
      /* rien */
    }
  }, [storageKey, layout, empties]);

  const placementAt = useCallback(
    (h: Held, c: Cell, g: Cell | null): { p: Placement; ok: boolean } | null => {
      if (!cat) return null;
      const o = pieceDef(cat, h.kind).orientations[h.o];
      const a = g ? clampAnchor(o, c.x - g.x, c.y - g.y) : anchorFor(o, c.x, c.y);
      const p: Placement = { kind: h.kind, o: h.o, ...a };
      return { p, ok: fits(cat, p) && !overlaps(cat, layout, p) };
    },
    [cat, layout],
  );

  const preview = useMemo(() => (held && hover ? placementAt(held, hover, grip) : null), [held, hover, grip, placementAt]);

  // Les gestes sont suivis sur la fenêtre : au doigt, le pointeur reste attaché
  // à l'élément touché en premier (la réserve), pas à la mine.
  const live = useRef({ layout, empties, held, grip, placementAt });
  live.current = { layout, empties, held, grip, placementAt };

  const cellAt = useCallback((cx: number, cy: number, clamp = false): Cell | null => {
    const ctm = board.current?.getScreenCTM();
    if (!ctm) return null;
    const pt = new DOMPoint(cx, cy).matrixTransform(ctm.inverse());
    let x = Math.floor(pt.x);
    let y = Math.floor(pt.y);
    if (clamp) {
      x = Math.min(Math.max(x, 0), W - 1);
      y = Math.min(Math.max(y, 0), H - 1);
    }
    return x >= 0 && x < W && y >= 0 && y < H ? { x, y } : null;
  }, []);

  const endGesture = useCallback(() => {
    gesture.current = null;
    setDragging(false);
    setFloat(null);
  }, []);

  const lift = useCallback((g: Extract<Gesture, { t: "press" }>) => {
    const { layout, held } = live.current;
    setMarkMode(false);
    let origin = g.from;
    let o = g.from?.o ?? 0;
    if (!origin) {
      // Depuis la réserve : la gemme quitte la mine si elle y était.
      origin = layout.find((p) => p.kind === g.kind) ?? null;
      o = origin?.o ?? (held?.kind === g.kind ? held.o : 0);
    }
    if (origin) setLayout((l) => l.filter((p) => p.kind !== g.kind));
    setHeld({ kind: g.kind, o });
    setGrip(g.grip);
    setDragging(true);
    gesture.current = { t: "drag", id: g.id, origin };
  }, []);

  const paintTo = useCallback((g: Extract<Gesture, { t: "paint" }>, c: Cell, free: boolean) => {
    if (c.x === g.last.x && c.y === g.last.y) return;
    if (free) g.axis = "free";
    else if (g.axis === null) g.axis = Math.abs(c.x - g.start.x) >= Math.abs(c.y - g.start.y) ? "row" : "col";
    if (g.axis === "free") {
      // Toutes les cases entre le dernier point et celui-ci, même en allant vite.
      const steps = Math.max(Math.abs(c.x - g.last.x), Math.abs(c.y - g.last.y));
      for (let i = 1; i <= steps; i++) {
        const x = Math.round(g.last.x + ((c.x - g.last.x) * i) / steps);
        const y = Math.round(g.last.y + ((c.y - g.last.y) * i) / steps);
        g.done.add(`${x},${y}`);
      }
    } else {
      // Sur une ligne, on peint du départ jusqu'au pointeur : revenir en arrière dépeint.
      g.done = new Set();
      const [a, b] = g.axis === "row" ? [g.start.x, c.x] : [g.start.y, c.y];
      for (let i = Math.min(a, b); i <= Math.max(a, b); i++) g.done.add(g.axis === "row" ? `${i},${g.start.y}` : `${g.start.x},${i}`);
    }
    g.last = c;
    const n = new Set(g.base);
    for (const k of g.done) {
      if (g.add) n.add(k);
      else n.delete(k);
    }
    setEmpties(n);
  }, []);

  useEffect(() => {
    const move = (ev: PointerEvent) => {
      const g = gesture.current;
      if (!g || ev.pointerId !== g.id) return;
      if (g.t === "press") {
        if (Math.hypot(ev.clientX - g.sx, ev.clientY - g.sy) < THRESHOLD) return;
        lift(g);
      }
      const cur = gesture.current!;
      if (cur.t === "drag") {
        const c = cellAt(ev.clientX, ev.clientY);
        setHover(c);
        setFloat(c ? null : { x: ev.clientX, y: ev.clientY });
      } else if (cur.t === "paint") {
        const c = cellAt(ev.clientX, ev.clientY, true);
        if (c) paintTo(cur, c, ev.shiftKey);
      }
    };
    const up = (ev: PointerEvent) => {
      const g = gesture.current;
      if (!g || ev.pointerId !== g.id) return;
      const { held, grip, placementAt } = live.current;
      if (g.t === "press") {
        // Un simple clic : la gemme reste en main, on la posera d'un autre clic.
        setMarkMode(false);
        if (g.from) {
          setLayout((l) => l.filter((p) => p.kind !== g.kind));
          setHeld({ kind: g.kind, o: g.from.o });
          setGrip(g.grip);
        } else {
          const placed = live.current.layout.find((p) => p.kind === g.kind);
          if (placed) setLayout((l) => l.filter((p) => p.kind !== g.kind));
          setHeld((h) => (h?.kind === g.kind && !placed ? null : { kind: g.kind, o: placed?.o ?? 0 }));
          setGrip(null);
        }
      } else if (g.t === "drag" && held) {
        const c = ev.type === "pointerup" ? cellAt(ev.clientX, ev.clientY) : null;
        const at = c ? placementAt(held, c, grip) : null;
        if (at?.ok) setLayout((l) => [...l.filter((q) => q.kind !== at.p.kind), at.p]);
        // Lâchée sur la mine à une place impossible : elle revient où elle était.
        // Lâchée hors de la mine : elle retourne dans la réserve.
        // Geste interrompu par le navigateur : pareil, on ne perd rien.
        else if ((c || ev.type === "pointercancel") && g.origin) setLayout((l) => [...l, g.origin!]);
        setHeld(null);
        setGrip(null);
        if (!c) setHover(null);
      }
      endGesture();
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
    window.addEventListener("pointercancel", up);
    return () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
      window.removeEventListener("pointercancel", up);
    };
  }, [cellAt, lift, paintTo, endGesture]);

  /** Appui sur une case de la mine. */
  const down = useCallback(
    (x: number, y: number, ev: ReactPointerEvent) => {
      if (!cat || (ev.pointerType === "mouse" && ev.button !== 0)) return;
      if (markMode) {
        const k = `${x},${y}`;
        const g: Gesture = { t: "paint", id: ev.pointerId, add: !empties.has(k), start: { x, y }, last: { x, y }, axis: null, base: new Set(empties), done: new Set([k]) };
        gesture.current = g;
        const n = new Set(empties);
        if (g.add) n.add(k);
        else n.delete(k);
        setEmpties(n);
        return;
      }
      if (held) {
        const at = placementAt(held, { x, y }, grip);
        if (at?.ok) {
          setLayout((l) => [...l.filter((q) => q.kind !== at.p.kind), at.p]);
          setHeld(null);
          setGrip(null);
        }
        return;
      }
      const i = pieceAt(cat, layout, x, y);
      if (i >= 0) {
        const p = layout[i];
        gesture.current = { t: "press", id: ev.pointerId, sx: ev.clientX, sy: ev.clientY, kind: p.kind, from: p, grip: { x: x - p.x, y: y - p.y } };
      }
    },
    [cat, markMode, empties, held, grip, layout, placementAt],
  );

  /** Appui sur une gemme de la réserve. */
  const pressPalette = useCallback((kind: Kind, ev: ReactPointerEvent) => {
    if (ev.pointerType === "mouse" && ev.button !== 0) return;
    ev.preventDefault();
    gesture.current = { t: "press", id: ev.pointerId, sx: ev.clientX, sy: ev.clientY, kind, from: null, grip: null };
  }, []);

  /** Prendre ou reposer une gemme de la réserve (clavier). */
  const take = useCallback(
    (kind: Kind) => {
      setMarkMode(false);
      setGrip(null);
      const placed = layout.find((p) => p.kind === kind);
      if (placed) setLayout((l) => l.filter((p) => p.kind !== kind));
      setHeld((h) => (h?.kind === kind && !placed ? null : { kind, o: placed?.o ?? 0 }));
    },
    [layout],
  );

  const rotate = useCallback(() => {
    if (!cat) return;
    setGrip(null);
    setHeld((h) => (h ? { ...h, o: rotated(pieceDef(cat, h.kind), h.o) } : h));
  }, [cat]);

  const flip = useCallback(() => {
    if (!cat) return;
    setGrip(null);
    setHeld((h) => (h ? { ...h, o: flipped(pieceDef(cat, h.kind), h.o) } : h));
  }, [cat]);

  const drop = useCallback(() => {
    setHeld(null);
    setGrip(null);
  }, []);

  /** Marque d'un coup des cases vides (le trajet d'une onde, par exemple). */
  const markEmpty = useCallback((cells: Cell[]) => {
    setEmpties((s) => {
      const n = new Set(s);
      for (const c of cells) n.add(`${c.x},${c.y}`);
      return n;
    });
  }, []);

  const clear = useCallback(() => {
    setLayout([]);
    setEmpties(new Set());
    setHeld(null);
    setGrip(null);
  }, []);

  const overGem = !held && !markMode && hover && cat ? pieceAt(cat, layout, hover.x, hover.y) >= 0 : false;
  const cursor = dragging ? "grabbing" : markMode ? "cell" : overGem ? "grab" : undefined;

  return {
    layout,
    setLayout,
    empties,
    held,
    hover,
    setHover,
    preview,
    board,
    down,
    pressPalette,
    take,
    rotate,
    flip,
    drop,
    markEmpty,
    clear,
    markMode,
    setMarkMode,
    float,
    cursor,
  };
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
      else if (ev.key === "Escape") ed.drop();
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
              onPointerDown={(ev) => ed.pressPalette(k, ev)}
              // Le clic à la souris ou au doigt passe par le geste ; ici seulement le clavier.
              onClick={(ev) => ev.detail === 0 && ed.take(k)}
              aria-pressed={isHeld}
              title={placed ? `Déplacer le ${def.name}` : `Glisser le ${def.name} sur la mine`}
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
            title="Glisse sur la mine pour marquer une ligne de cases (Maj : tracé libre)"
            onClick={() => {
              ed.drop();
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
      {ed.float && ed.held && (
        <div className="drag-float" style={{ left: ed.float.x, top: ed.float.y }} aria-hidden>
          <MiniGem cat={cat} kind={ed.held.kind} o={ed.held.o} />
        </div>
      )}
    </div>
  );
}

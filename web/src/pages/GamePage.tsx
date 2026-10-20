import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api, guestName, saveGuestName, saveSeatToken, seatToken } from "../api/client";
import type { ActionView, Catalog, GameView, Layout } from "../api/types";
import { useGame, type GameSocket } from "../api/useGame";
import { useApp } from "../auth/AuthProvider";
import { Clock, fmt, useNow } from "../components/Clock";
import { GemPalette, useEditorKeys, useGemEditor } from "../components/GemEditor";
import { ColorGuide } from "../components/ColorGuide";
import { Header } from "../components/Header";
import { MineBoard, type CellMark, type EdgeMark, type WavePath } from "../components/MineBoard";
import { GEM_HEX, gemName, waveHex, waveName, waveRecipe } from "../lib/colors";
import { H, W, cellName, entryLabel, entryPos } from "../lib/pieces";

// ---- lecture des actions

/** Ondes et sondages d'un joueur contre la mine de l'autre. */
function readActions(actions: ActionView[], seat: number) {
  const marks = new Map<number, EdgeMark>();
  const probes = new Map<string, CellMark>();
  const waves: (ActionView & { n: number })[] = [];
  let n = 0;
  for (const a of actions) {
    if (a.seat !== seat) continue;
    if (a.kind === "wave") {
      n++;
      waves.push({ ...a, n });
      const m: EdgeMark = { n, colors: a.colors, absorbed: a.absorbed, back: a.exit === a.entry };
      marks.set(a.entry, m);
      if (a.exit >= 0 && a.exit !== a.entry) marks.set(a.exit, m);
    } else if (a.kind === "probe") {
      probes.set(`${a.x},${a.y}`, { kind: "probe", color: a.found });
    }
  }
  return { marks, probes, waves };
}

/**
 * Cases d'une onde sans couleur ressortie juste en face : elle n'a rien touché,
 * toute sa ligne est vide. null pour les autres ondes.
 */
function clearLine(a: ActionView): { x: number; y: number }[] | null {
  if (a.kind !== "wave" || a.absorbed || a.colors !== 0 || a.exit < 0) return null;
  const p = entryPos(a.entry);
  const q = entryPos(a.exit);
  if (p.x === q.x && p.x > 0 && p.x < W) return Array.from({ length: H }, (_, y) => ({ x: Math.floor(p.x), y }));
  if (p.y === q.y && p.y > 0 && p.y < H) return Array.from({ length: W }, (_, x) => ({ x, y: Math.floor(p.y) }));
  return null;
}

const REASONS: Record<string, (w: string, l: string) => string> = {
  solved: (w) => `${w} a trouvé la disposition.`,
  wrong: (_, l) => `${l} a proposé une disposition fausse.`,
  timeout: (_, l) => `${l} n'a plus de temps.`,
  resign: (_, l) => `${l} a abandonné.`,
  "setup-timeout": (_, l) => `${l} n'a pas caché ses gemmes à temps.`,
  "both-solved": () => "Les deux ont trouvé la disposition.",
  "no-setup": () => "Personne n'a caché ses gemmes à temps.",
};

// ---- page

export function GamePage({ id }: { id: string }) {
  const { catalog } = useApp();
  const [token, setToken] = useState(() => seatToken(id));
  const sock = useGame(id, token);
  const navigate = useNavigate();
  const s = sock.state;

  // Revanche acceptée pendant qu'on regarde : la place suit dans la nouvelle
  // partie. (Revoir une vieille partie ne renvoie pas vers sa revanche.)
  const firstNext = useRef<string | null>(null);
  useEffect(() => {
    if (!s) return;
    firstNext.current ??= s.next ?? "";
    if (s.next && s.next !== firstNext.current && token) {
      saveSeatToken(s.next, token);
      navigate(`/g/${s.next}`);
    }
  }, [s, token, navigate]);

  if (sock.notFound) {
    return (
      <>
        <Header />
        <main className="page page-narrow">
          <h1>Partie introuvable</h1>
          <p className="muted">Le lien est peut-être incomplet.</p>
          <Link className="btn" to="/">
            Revenir à l'accueil
          </Link>
        </main>
      </>
    );
  }
  if (!s || !catalog) {
    return (
      <>
        <Header />
        <main className="page page-narrow">
          <p className="muted">{sock.conn === "closed" ? "Connexion au serveur…" : "Chargement de la partie…"}</p>
        </main>
      </>
    );
  }

  return (
    <>
      <Header>{sock.conn !== "open" && <span className="conn">Reconnexion…</span>}</Header>
      <main className="page game">
        {s.phase === "waiting" && <Waiting s={s} sock={sock} onJoined={setToken} />}
        {s.phase === "canceled" && (
          <div className="page-narrow">
            <h1>Partie annulée</h1>
            <Link className="btn" to="/">
              Revenir à l'accueil
            </Link>
          </div>
        )}
        {s.phase === "setup" && <Setup key={s.id} s={s} sock={sock} cat={catalog} />}
        {(s.phase === "playing" || s.phase === "finished") && <Play s={s} sock={sock} cat={catalog} />}
      </main>
    </>
  );
}

// ---- attente du deuxième joueur

function Waiting({ s, sock, onJoined }: { s: GameView; sock: GameSocket; onJoined: (t: string) => void }) {
  const { user, accounts, signIn } = useApp();
  const [name, setName] = useState(guestName);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const host = s.players[0]!;
  const url = window.location.href;

  if (s.you === 0) {
    return (
      <div className="page-narrow waiting">
        <h1>En attente d'un adversaire</h1>
        <p className="lede">Envoie ce lien à la personne qui jouera contre toi. La partie commence dès son arrivée.</p>
        <div className="share">
          <input readOnly value={url} onFocus={(e) => e.target.select()} aria-label="Lien de la partie" />
          <button
            className="btn btn-primary"
            onClick={() => {
              void navigator.clipboard?.writeText(url).then(() => setCopied(true));
            }}
          >
            {copied ? "Lien copié" : "Copier le lien"}
          </button>
        </div>
        <p className="muted">
          Pendule : {s.baseMs / 60000} min chacun, +{s.incMs / 1000} s par coup.{" "}
          {s.public ? "La partie est aussi visible dans les parties ouvertes." : ""}
        </p>
        <button className="btn btn-quiet" onClick={() => sock.send({ t: "cancel" })}>
          Annuler la partie
        </button>
      </div>
    );
  }

  const join = async (ev: FormEvent) => {
    ev.preventDefault();
    setError(null);
    try {
      if (!user) saveGuestName(name.trim());
      const r = await api.join(s.id, name.trim());
      saveSeatToken(s.id, r.token);
      onJoined(r.token);
    } catch (e) {
      setError((e as Error).message);
    }
  };

  return (
    <form className="page-narrow waiting" onSubmit={join}>
      <h1>{host.name} t'invite à jouer</h1>
      <p className="lede">
        Pendule : {s.baseMs / 60000} min chacun, +{s.incMs / 1000} s par coup.
      </p>
      {user ? (
        <p>
          Tu joues en tant que <strong>{user.name}</strong>.
        </p>
      ) : (
        <>
          <label className="field">
            <span>Ton nom</span>
            <input value={name} onChange={(e) => setName(e.target.value)} maxLength={24} placeholder="Invité" autoComplete="nickname" />
          </label>
          {accounts && (
            <p className="muted">
              Ou{" "}
              <button type="button" className="link" onClick={() => signIn()}>
                connecte-toi avec ton compte Eriphe
              </button>{" "}
              pour garder tes parties.
            </p>
          )}
        </>
      )}
      {error && <p className="error">{error}</p>}
      <button className="btn btn-primary">Rejoindre la partie</button>
    </form>
  );
}

// ---- pose des gemmes

function Setup({ s, sock, cat }: { s: GameView; sock: GameSocket; cat: Catalog }) {
  const me = s.you >= 0 ? s.players[s.you]! : null;
  const opp = s.you >= 0 ? s.players[1 - s.you]! : null;
  const ed = useGemEditor(cat, s.myLayout ?? [], `orapa:setup:${s.id}`);
  const ready = me?.ready ?? false;
  useEditorKeys(ed, !ready && s.you >= 0);
  const now = useNow(true, 500);
  const left = s.setupLeftMs - (now - sock.receivedAt);

  if (s.you < 0) {
    return (
      <div className="page-narrow">
        <h1>
          {s.players[0]?.name} contre {s.players[1]?.name}
        </h1>
        <p className="lede">Les deux cachent leurs gemmes ({fmt(left)}).</p>
      </div>
    );
  }

  return (
    <div className="setup">
      <div className="status-line">
        <h1>{ready ? "Tes gemmes sont cachées" : "Cache tes gemmes"}</h1>
        <span className={`countdown ${left < 30_000 ? "is-low" : ""}`}>{fmt(left)}</span>
      </div>
      <p className="lede">
        {ready
          ? opp!.ready
            ? "La chasse commence."
            : `${opp!.name} cache encore les siennes.`
          : "Pose les cinq gemmes : coins sur les points de la grille, jamais deux gemmes côte à côte par un côté, et chacune visible depuis un bord."}
      </p>
      <div className="setup-board">
        <MineBoard
          cat={cat}
          gems={ready ? s.myLayout : ed.layout}
          preview={ready ? null : ed.preview}
          onCell={ready ? undefined : ed.down}
          onHover={ed.setHover}
          svgRef={ed.board}
          cursor={ed.cursor}
          label="Ta mine"
        />
      </div>
      {!ready && (
        <div className="setup-tools">
          <GemPalette cat={cat} ed={ed} />
          {sock.error && <p className="error">{sock.error}</p>}
          <div className="actions">
            <button className="btn btn-primary" disabled={ed.layout.length < 5} onClick={() => sock.send({ t: "hide", layout: ed.layout })}>
              Cacher mes gemmes
            </button>
            <span className="muted">Une fois cachées, elles ne bougent plus.</span>
          </div>
        </div>
      )}
      <p className="muted small">
        {opp!.name} : {opp!.ready ? "prêt à jouer" : "pose en cours"}. Si le temps s'écoule avant ta pose, tu perds la partie.
      </p>
    </div>
  );
}

// ---- la chasse

type Pending = { kind: "wave"; entry: number } | { kind: "probe"; x: number; y: number } | { kind: "solve" } | { kind: "resign" } | null;

function Play({ s, sock, cat }: { s: GameView; sock: GameSocket; cat: Catalog }) {
  if (s.you < 0) return <Spectate s={s} sock={sock} cat={cat} />;
  return <Hunt s={s} sock={sock} cat={cat} />;
}

function PlayerStrip({ s, seat, receivedAt, you }: { s: GameView; seat: number; receivedAt: number; you?: boolean }) {
  const p = s.players[seat]!;
  const running = s.phase === "playing" && s.turn === seat;
  return (
    <div className={`strip ${running ? "is-turn" : ""}`}>
      <span className={`dot ${p.online ? "is-online" : ""}`} title={p.online ? "En ligne" : "Hors ligne"} />
      <span className="strip-name">
        {p.name}
        {you && <span className="muted"> (toi)</span>}
      </span>
      <Clock ms={p.remainingMs} running={running} receivedAt={receivedAt} />
    </div>
  );
}

function Hunt({ s, sock, cat }: { s: GameView; sock: GameSocket; cat: Catalog }) {
  const you = s.you;
  const opp = 1 - you;
  const oppName = s.players[opp]!.name;
  const done = s.phase === "finished";
  const myTurn = s.phase === "playing" && s.turn === you;
  const lastChance = s.solver >= 0 && !done;
  const ed = useGemEditor(cat, [], `orapa:hunt:${s.id}`);
  const [pending, setPending] = useState<Pending>(null);
  const [probeMode, setProbeMode] = useState(false);
  const [focus, setFocus] = useState<number | null>(null); // onde survolée dans le journal
  const [tab, setTab] = useState<"mine" | "theirs">("mine");
  useEditorKeys(ed, !done);

  const mine = useMemo(() => readActions(s.actions, you), [s.actions, you]);
  const theirs = useMemo(() => readActions(s.actions, opp), [s.actions, opp]);

  // Un coup joué ferme la confirmation en cours.
  useEffect(() => {
    setPending(null);
    setProbeMode(false);
  }, [s.actions.length, s.phase]);

  const cellMarks = useMemo(() => {
    const m = new Map<string, CellMark>();
    for (const k of ed.empties) m.set(k, { kind: "empty" });
    for (const [k, v] of mine.probes) m.set(k, v);
    return m;
  }, [ed.empties, mine.probes]);

  const canAct = myTurn && !lastChance;
  const hot = useMemo(() => {
    const w = mine.waves.find((x) => x.n === focus);
    return w ? new Set([w.entry, w.exit]) : undefined;
  }, [mine.waves, focus]);

  const theirPaths: WavePath[] = theirs.waves
    .filter((w) => w.path)
    .map((w) => ({ points: w.path!, colors: w.colors, strong: w.n === theirs.waves.length, dim: w.n !== theirs.waves.length }));
  const myPaths: WavePath[] =
    done && focus !== null
      ? mine.waves.filter((w) => w.n === focus && w.path).map((w) => ({ points: w.path!, colors: w.colors, strong: true }))
      : [];

  const confirm = () => {
    if (!pending) return;
    if (pending.kind === "wave") sock.send({ t: "wave", entry: pending.entry });
    else if (pending.kind === "probe") sock.send({ t: "probe", x: pending.x, y: pending.y });
    else if (pending.kind === "solve") sock.send({ t: "solve", layout: ed.layout });
    else sock.send({ t: "resign" });
    setPending(null);
  };

  let status: string;
  if (done) {
    const r = s.result!;
    const w = r.winner >= 0 ? s.players[r.winner]!.name : "";
    const l = r.winner >= 0 ? s.players[1 - r.winner]!.name : "";
    status = REASONS[r.reason]?.(w, l) ?? "";
  } else if (lastChance) {
    status =
      s.solver === you
        ? `Tu as trouvé ! ${oppName} a une dernière chance de trouver ta disposition pour égaliser.`
        : `${oppName} a trouvé ta disposition. Dernière chance : propose la sienne pour égaliser.`;
  } else if (myTurn) {
    status = "À toi : envoie une onde depuis un bord, sonde une case, ou propose la disposition.";
  } else {
    status = `${oppName} réfléchit.`;
  }

  const outcome = done ? (s.result!.winner === -1 ? "Égalité" : s.result!.winner === you ? "Victoire" : "Défaite") : null;
  const revealed: Layout | undefined = done ? s.layouts?.[opp] : undefined;

  return (
    <div className="hunt">
      <div className="players">
        <PlayerStrip s={s} seat={you} receivedAt={sock.receivedAt} you />
        <PlayerStrip s={s} seat={opp} receivedAt={sock.receivedAt} />
      </div>

      <div className={`status ${myTurn ? "is-mine" : ""} ${done ? "is-done" : ""}`} role="status">
        {outcome && <strong className={`outcome-big outcome-${s.result!.winner === -1 ? "draw" : s.result!.winner === you ? "win" : "loss"}`}>{outcome}</strong>}
        <span>{status}</span>
      </div>

      <div className="hunt-grid">
        <section className="hunt-main" aria-label={`Mine de ${oppName}`}>
          <h2>
            Mine de {oppName}
            {done && <span className="muted small"> : la vraie pose, ta dernière hypothèse en pointillé</span>}
          </h2>
          <MineBoard
            cat={cat}
            gems={revealed ?? ed.layout}
            ghost={revealed ? ed.layout : undefined}
            preview={done || probeMode ? null : ed.preview}
            marks={mine.marks}
            cellMarks={cellMarks}
            paths={myPaths}
            hotEntries={hot}
            selectedEntry={pending?.kind === "wave" ? pending.entry : null}
            selectedCell={pending?.kind === "probe" ? pending : null}
            entryDisabled={(e) => !canAct || mine.marks.has(e)}
            onEntry={canAct ? (e) => setPending({ kind: "wave", entry: e }) : undefined}
            onCell={
              done
                ? undefined
                : (x, y, ev) => {
                    if (probeMode) setPending({ kind: "probe", x, y });
                    else ed.down(x, y, ev);
                  }
            }
            onHover={ed.setHover}
            svgRef={ed.board}
            cursor={probeMode ? undefined : ed.cursor}
            label={`Mine de ${oppName}`}
          />

          {pending && (
            <div className="confirm" role="dialog" aria-live="polite">
              <span>
                {pending.kind === "wave" && `Envoyer une onde depuis ${entryLabel(pending.entry)} ?`}
                {pending.kind === "probe" && `Sonder la case ${cellName(pending.x, pending.y)} ?`}
                {pending.kind === "solve" && "Proposer cette disposition ? Si elle est fausse, tu perds la partie."}
                {pending.kind === "resign" && "Abandonner la partie ?"}
              </span>
              <button className="btn btn-primary" onClick={confirm}>
                {pending.kind === "wave" ? "Envoyer" : pending.kind === "probe" ? "Sonder" : pending.kind === "solve" ? "Proposer" : "Abandonner"}
              </button>
              <button className="btn btn-quiet" onClick={() => setPending(null)}>
                Annuler
              </button>
            </div>
          )}
          {sock.error && <p className="error">{sock.error}</p>}

        </section>

        <aside className="hunt-side">
        {!done && (
          <>
            <GemPalette cat={cat} ed={ed} showMarks />
            <div className="actions">
              <button
                className={`btn ${probeMode ? "btn-on" : ""}`}
                disabled={!canAct}
                aria-pressed={probeMode}
                onClick={() => {
                  ed.drop();
                  ed.setMarkMode(false);
                  setProbeMode(!probeMode);
                  setPending(null);
                }}
              >
                {probeMode ? "Choisis une case…" : "Sonder une case"}
              </button>
              <button className="btn btn-primary" disabled={!myTurn || ed.layout.length < 5} onClick={() => setPending({ kind: "solve" })}>
                Proposer la disposition
              </button>
              <button className="btn btn-quiet" onClick={() => setPending({ kind: "resign" })}>
                Abandonner
              </button>
            </div>
          </>
        )}
        {done && <EndActions s={s} sock={sock} />}
          <div className="tabs" role="tablist">
            <button role="tab" aria-selected={tab === "mine"} className={tab === "mine" ? "is-on" : ""} onClick={() => setTab("mine")}>
              Ton journal
            </button>
            <button role="tab" aria-selected={tab === "theirs"} className={tab === "theirs" ? "is-on" : ""} onClick={() => setTab("theirs")}>
              Ta mine
            </button>
          </div>
          {tab === "mine" ? (
            <Journal
              actions={s.actions}
              seat={you}
              focus={focus}
              setFocus={setFocus}
              onMarkEmpty={done ? undefined : (a) => ed.markEmpty(clearLine(a) ?? [])}
              isMarked={(a) => (clearLine(a) ?? []).every((c) => ed.empties.has(`${c.x},${c.y}`))}
              empty="Pas encore d'onde. Clique un point du bord pour en envoyer une."
            />
          ) : (
            <>
              <MineBoard
                cat={cat}
                gems={s.myLayout}
                marks={theirs.marks}
                cellMarks={theirs.probes}
                paths={theirPaths}
                label="Ta mine, avec les ondes de ton adversaire"
                compact
              />
              <Journal actions={s.actions} seat={opp} empty={`${oppName} n'a encore rien envoyé.`} />
            </>
          )}
          <ColorDetails />
        </aside>
      </div>
    </div>
  );
}

function Journal({
  actions,
  seat,
  focus,
  setFocus,
  onMarkEmpty,
  isMarked,
  empty,
}: {
  actions: ActionView[];
  seat: number;
  focus?: number | null;
  setFocus?: (n: number | null) => void;
  onMarkEmpty?: (a: ActionView) => void;
  isMarked?: (a: ActionView) => boolean;
  empty: string;
}) {
  const list = useRef<HTMLOListElement>(null);
  let n = 0;
  const rows = actions
    .filter((a) => a.seat === seat)
    .map((a, i) => ({ a, i, wn: a.kind === "wave" ? ++n : null }));

  // Dans l'ordre des coups, le dernier toujours en vue.
  useEffect(() => {
    const el = list.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [rows.length]);

  if (rows.length === 0) return <p className="muted small journal-empty">{empty}</p>;
  return (
    <ol className="journal" ref={list}>
      {rows.map(({ a, i, wn }) => (
        <li
          key={i}
          className={wn !== null && focus === wn ? "is-focus" : ""}
          onMouseEnter={() => wn !== null && setFocus?.(wn)}
          onMouseLeave={() => setFocus?.(null)}
        >
          <span className="journal-n">{wn ?? (a.kind === "probe" ? "?" : "!")}</span>
          {a.kind === "wave" ? (
            <>
              <span className="journal-in">{entryLabel(a.entry)}</span>
              <span className="journal-arrow" aria-label="vers">→</span>
              <span className="journal-out">
                {a.absorbed ? (
                  <span title="absorbée">×</span>
                ) : (
                  <>
                    {entryLabel(a.exit)}
                    {a.exit === a.entry && (
                      <span className="journal-back" title="revenue à son point d'entrée">
                        ↺
                      </span>
                    )}
                  </>
                )}
              </span>
              <span className="journal-what" title={a.absorbed ? undefined : waveRecipe(a.colors)}>
                {a.absorbed ? (
                  "absorbée"
                ) : (
                  <>
                    <span className={`swatch ${a.colors ? "" : "swatch-clear"}`} style={{ background: a.colors ? waveHex(a.colors) : undefined }} />
                    <span>{waveName(a.colors)}</span>
                  </>
                )}
              </span>
              {onMarkEmpty && clearLine(a) && !isMarked?.(a) && (
                <button type="button" className="journal-act" onClick={() => onMarkEmpty(a)} title="Elle n'a rien touché : toute sa ligne est vide">
                  Marquer vide
                </button>
              )}
            </>
          ) : a.kind === "probe" ? (
            <>
              <span className="journal-cell">{cellName(a.x, a.y)}</span>
              <span className="journal-what">
                {a.found ? (
                  <>
                    <span className="swatch" style={{ background: GEM_HEX[a.found] }} />
                    <span>gemme {gemName(a.found)}</span>
                  </>
                ) : (
                  "rien"
                )}
              </span>
            </>
          ) : (
            <>
              <span className="journal-cell">disposition</span>
              <span className="journal-what">{a.correct ? "juste" : "fausse"}</span>
            </>
          )}
        </li>
      ))}
    </ol>
  );
}

// Guide des couleurs repliable sous le journal ; ouvert par défaut, l'état est retenu.
function ColorDetails() {
  const [open, setOpen] = useState(() => {
    try {
      return localStorage.getItem("orapa:guide") !== "closed";
    } catch {
      return true;
    }
  });
  return (
    <details
      className="guide"
      open={open}
      onToggle={(e) => {
        const now = e.currentTarget.open;
        setOpen(now);
        try {
          localStorage.setItem("orapa:guide", now ? "open" : "closed");
        } catch {
          // stockage indisponible : l'état n'est pas retenu
        }
      }}
    >
      <summary>Guide des couleurs</summary>
      <p className="muted small">L'onde prend la couleur de chaque gemme touchée.</p>
      <ColorGuide />
    </details>
  );
}

function EndActions({ s, sock }: { s: GameView; sock: GameSocket }) {
  const you = s.you;
  const opp = 1 - you;
  const asked = s.rematch[you];
  const offered = s.rematch[opp];
  return (
    <div className="actions end-actions">
      {you >= 0 && (
        <button className="btn btn-primary" disabled={asked || Boolean(s.next)} onClick={() => sock.send({ t: "rematch" })}>
          {asked ? "Revanche demandée" : offered ? "Accepter la revanche" : "Proposer une revanche"}
        </button>
      )}
      {offered && !asked && !s.next && <span className="muted">{s.players[opp]!.name} propose une revanche.</span>}
      {s.next && (
        <Link className="btn" to={`/g/${s.next}`}>
          Voir la revanche
        </Link>
      )}
      <Link className="btn btn-quiet" to="/">
        Accueil
      </Link>
    </div>
  );
}

// ---- spectateur : les deux mines, sans rien de caché avant la fin

function Spectate({ s, sock, cat }: { s: GameView; sock: GameSocket; cat: Catalog }) {
  const done = s.phase === "finished";
  const a0 = useMemo(() => readActions(s.actions, 0), [s.actions]);
  const a1 = useMemo(() => readActions(s.actions, 1), [s.actions]);
  return (
    <div className="hunt">
      <div className="players">
        <PlayerStrip s={s} seat={0} receivedAt={sock.receivedAt} />
        <PlayerStrip s={s} seat={1} receivedAt={sock.receivedAt} />
      </div>
      {done && (
        <div className="status is-done">
          {REASONS[s.result!.reason]?.(
            s.result!.winner >= 0 ? s.players[s.result!.winner]!.name : "",
            s.result!.winner >= 0 ? s.players[1 - s.result!.winner]!.name : "",
          )}
        </div>
      )}
      <div className="spectate">
        {[0, 1].map((seat) => {
          const hunter = seat === 0 ? a1 : a0;
          return (
            <section key={seat}>
              <h2>Mine de {s.players[seat]!.name}</h2>
              <MineBoard cat={cat} gems={s.layouts?.[seat]} marks={hunter.marks} cellMarks={hunter.probes} label={`Mine de ${s.players[seat]!.name}`} />
              <Journal actions={s.actions} seat={1 - seat} empty="Rien pour l'instant." />
            </section>
          );
        })}
      </div>
      {done && (
        <div className="actions">
          <Link className="btn" to="/">
            Accueil
          </Link>
        </div>
      )}
    </div>
  );
}

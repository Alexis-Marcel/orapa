import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api, guestName, saveGuestName, saveSeatToken, seatTokens } from "../api/client";
import type { Catalog, CurrentGame, Layout, Me, OpenGame } from "../api/types";
import { useApp } from "../auth/AuthProvider";
import { ColorGuide } from "../components/ColorGuide";
import { Header } from "../components/Header";
import { MineBoard, type EdgeMark, type WavePath } from "../components/MineBoard";
import { coloredSegments } from "../lib/waves";

// Une mine de démonstration et une onde qui la traverse : elle part du point 9,
// rebondit sur les quatre couleurs et ressort grise en C.
const DEMO: Layout = [
  { kind: "white-triangle", o: 1, x: 0, y: 2 },
  { kind: "blue-triangle", o: 3, x: 1, y: 4 },
  { kind: "yellow-triangle", o: 1, x: 0, y: 6 },
  { kind: "white-diamond", o: 0, x: 8, y: 2 },
  { kind: "red-parallelogram", o: 3, x: 2, y: 0 },
];
const DEMO_PATH = [
  [8.5, 0], [8.5, 2.5], [2.5, 2.5], [2.5, 4.5], [1.5, 4.5], [1.5, 5.5],
  [0.5, 5.5], [0.5, 6.5], [1.5, 6.5], [1.5, 7.5], [2.5, 7.5], [2.5, 8],
].map(([x, y]) => ({ x, y }));

function DemoMine({ cat }: { cat: Catalog }) {
  const reduced = typeof window !== "undefined" && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  const { paths, marks } = useMemo(() => {
    const segs = coloredSegments(cat, DEMO, DEMO_PATH);
    const speed = 110; // ms par case
    let t = 600;
    const paths: WavePath[] = segs.map((s) => {
      const p: WavePath = { points: s.points, colors: s.colors, strong: true, draw: reduced ? undefined : { delay: t, duration: s.length * speed } };
      t += s.length * speed;
      return p;
    });
    const marks = new Map<number, EdgeMark>([
      [8, { n: 1, colors: 15 }],
      [20, { n: 1, colors: 15 }],
    ]);
    return { paths, marks };
  }, [cat, reduced]);
  return <MineBoard cat={cat} gems={DEMO} paths={paths} marks={marks} label="Exemple : une onde entre en 9, touche les quatre couleurs et ressort grise en C" decorative />;
}

const CADENCES = [
  { id: "5+3", label: "5 min", hint: "+3 s par coup" },
  { id: "10+5", label: "10 min", hint: "+5 s par coup" },
  { id: "15+10", label: "15 min", hint: "+10 s par coup" },
  { id: "25+15", label: "25 min", hint: "+15 s par coup" },
];

function cadenceOf(baseMs: number, incMs: number) {
  return `${Math.round(baseMs / 60000)} min +${Math.round(incMs / 1000)} s`;
}

function CreateForm() {
  const { user, loading } = useApp();
  const navigate = useNavigate();
  const [name, setName] = useState(guestName);
  const [cadence, setCadence] = useState("10+5");
  const [isPublic, setPublic] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async (ev: FormEvent) => {
    ev.preventDefault();
    setBusy(true);
    setError(null);
    try {
      if (!user) saveGuestName(name.trim());
      const r = await api.create(name.trim(), cadence, isPublic);
      saveSeatToken(r.id, r.token);
      navigate(`/g/${r.id}`);
    } catch (e) {
      setError((e as Error).message);
      setBusy(false);
    }
  };

  return (
    <form className="panel create" onSubmit={submit}>
      <h2>Nouvelle partie</h2>
      {/* Le temps de savoir si l'on est connecté, pas de champ « invité ». */}
      {!user && !loading && (
        <label className="field">
          <span>Ton nom</span>
          <input value={name} onChange={(e) => setName(e.target.value)} maxLength={24} placeholder="Invité" autoComplete="nickname" />
        </label>
      )}
      <fieldset className="field">
        <legend>Pendule</legend>
        <div className="segmented">
          {CADENCES.map((c) => (
            <label key={c.id} className={cadence === c.id ? "is-on" : ""}>
              <input type="radio" name="cadence" value={c.id} checked={cadence === c.id} onChange={() => setCadence(c.id)} />
              <strong>{c.label}</strong>
              <small>{c.hint}</small>
            </label>
          ))}
        </div>
      </fieldset>
      <label className="check">
        <input type="checkbox" checked={isPublic} onChange={(e) => setPublic(e.target.checked)} />
        <span>Visible dans les parties ouvertes (sinon, seulement par le lien)</span>
      </label>
      {error && <p className="error">{error}</p>}
      <button className="btn btn-primary" disabled={busy || loading}>
        Créer la partie
      </button>
    </form>
  );
}

function OpenGames() {
  const [games, setGames] = useState<OpenGame[] | null>(null);
  useEffect(() => {
    let stop = false;
    const load = () => api.open().then((g) => !stop && setGames(g), () => {});
    void load();
    const t = setInterval(load, 5000);
    return () => {
      stop = true;
      clearInterval(t);
    };
  }, []);
  return (
    <section className="panel">
      <h2>Parties ouvertes</h2>
      {games === null ? (
        <p className="muted">Chargement…</p>
      ) : games.length === 0 ? (
        <p className="muted">Aucune partie n'attend d'adversaire. Crée la tienne et envoie le lien.</p>
      ) : (
        <ul className="rows">
          {games.map((g) => (
            <li key={g.id}>
              <span className="row-main">{g.host}</span>
              <span className="muted">{cadenceOf(g.baseMs, g.incMs)}</span>
              <Link className="btn btn-small" to={`/g/${g.id}`}>
                Rejoindre
              </Link>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

// Parties pas finies de ce navigateur (ou du compte), pour y revenir après avoir fermé l'onglet.
function CurrentGames() {
  const { user } = useApp();
  const [games, setGames] = useState<CurrentGame[]>([]);
  useEffect(() => {
    let stop = false;
    const load = () => api.current(seatTokens()).then((g) => !stop && setGames(g), () => {});
    void load();
    const t = setInterval(load, 10000);
    return () => {
      stop = true;
      clearInterval(t);
    };
  }, [user]);
  if (games.length === 0) return null;
  return (
    <section className="panel current">
      <h2>Tes parties en cours</h2>
      <ul className="rows">
        {games.map((g) => (
          <li key={g.id}>
            <span className="row-main">{g.opponent ? `contre ${g.opponent}` : "en attente d'un adversaire"}</span>
            <span className={g.yourMove ? "turn-mine" : "muted"}>{currentStatus(g)}</span>
            <Link className={`btn btn-small ${g.yourMove ? "btn-primary" : ""}`} to={`/g/${g.id}`}>
              Reprendre
            </Link>
          </li>
        ))}
      </ul>
    </section>
  );
}

function currentStatus(g: CurrentGame): string {
  if (g.phase === "waiting") return "lien à partager";
  if (g.phase === "setup") return g.yourMove ? "à toi de cacher tes gemmes" : "pose en cours";
  return g.yourMove ? "à toi de jouer" : `à ${g.opponent}`;
}

const OUTCOME = { win: "Victoire", loss: "Défaite", draw: "Égalité" };

function MyGames() {
  const { user } = useApp();
  const [me, setMe] = useState<Me | null>(null);
  useEffect(() => {
    if (user) api.me().then(setMe, () => setMe(null));
  }, [user]);
  if (!user || !me) return null;
  return (
    <section className="panel">
      <h2>Tes parties</h2>
      {me.stats.played === 0 ? (
        <p className="muted">Pas encore de partie terminée.</p>
      ) : (
        <>
          <p className="stats">
            {me.stats.won} victoire{me.stats.won > 1 ? "s" : ""}, {me.stats.lost} défaite{me.stats.lost > 1 ? "s" : ""}
            {me.stats.drawn > 0 && `, ${me.stats.drawn} égalité${me.stats.drawn > 1 ? "s" : ""}`}
          </p>
          <ul className="rows">
            {me.games.slice(0, 8).map((g) => (
              <li key={g.id}>
                <span className={`outcome outcome-${g.outcome}`}>{OUTCOME[g.outcome]}</span>
                <span className="row-main">contre {g.opponent}</span>
                <Link className="btn btn-small btn-quiet" to={`/g/${g.id}`}>
                  Revoir
                </Link>
              </li>
            ))}
          </ul>
        </>
      )}
    </section>
  );
}

function Rules() {
  return (
    <section className="panel rules">
      <h2>Comment on joue</h2>
      <ol>
        <li>Chacun cache ses cinq gemmes dans sa mine de 10 × 8, sans que deux gemmes se touchent par un côté.</li>
        <li>
          À ton tour, envoie une onde depuis un point du bord. Elle va tout droit, tourne de 90° sur une face en diagonale et repart d'où
          elle vient sur une face droite. Tu apprends où elle ressort et de quelle couleur.
        </li>
        <li>Tu peux aussi sonder une case : tu sais alors quelle couleur de gemme s'y trouve, mais pas sa forme.</li>
        <li>
          Quand tu penses avoir tout trouvé, propose la disposition. Juste, tu gagnes ; fausse, tu perds. Si tu as commencé, l'autre a une
          dernière chance d'égaliser.
        </li>
      </ol>
      <h3>Couleur de l'onde</h3>
      <p className="muted">Elle prend la couleur de chaque gemme touchée. Toucher deux fois la même couleur ne change rien.</p>
      <ColorGuide />
    </section>
  );
}

export function HomePage() {
  const { catalog } = useApp();
  return (
    <>
      <Header />
      <main className="page home">
        <section className="hero">
          <div className="hero-text">
            <h1>Orapa</h1>
            <p className="lede">Cache cinq gemmes, sonde la mine d'en face avec des ondes, et trouve sa disposition avant que la tienne soit trouvée.</p>
          </div>
          <div className="hero-mine">{catalog && <DemoMine cat={catalog} />}</div>
        </section>
        <div className="home-grid">
          <div className="home-col">
            <CurrentGames />
            <CreateForm />
            <MyGames />
          </div>
          <div className="home-col">
            <OpenGames />
            <Rules />
          </div>
        </div>
      </main>
    </>
  );
}

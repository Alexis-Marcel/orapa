// Types du protocole, en miroir de internal/game et internal/server.

export type Kind = "white-triangle" | "blue-triangle" | "yellow-triangle" | "white-diamond" | "red-parallelogram";

export interface Placement {
  kind: Kind;
  o: number;
  x: number;
  y: number;
}

export type Layout = Placement[];

/** 0 vide, 1 pleine, 2 ◥, 3 ◢, 4 ◣, 5 ◤ (le coin de l'angle droit). */
export type Fill = 0 | 1 | 2 | 3 | 4 | 5;

export interface Point {
  x: number;
  y: number;
}

export interface Orientation {
  w: number;
  h: number;
  cells: { x: number; y: number; fill: Fill }[];
  outline: Point[];
}

export interface PieceDef {
  kind: Kind;
  name: string;
  color: number;
  orientations: Orientation[];
}

export interface Catalog {
  width: number;
  height: number;
  pieces: PieceDef[];
}

export type Phase = "waiting" | "setup" | "playing" | "finished" | "canceled";

export type Reason = "solved" | "wrong" | "timeout" | "resign" | "setup-timeout" | "both-solved" | "no-setup";

export interface PlayerView {
  name: string;
  account: boolean;
  ready: boolean;
  online: boolean;
  turns: number;
  remainingMs: number;
}

export interface ActionView {
  seat: number;
  kind: "wave" | "probe" | "solve";
  entry: number;
  exit: number;
  colors: number;
  absorbed?: boolean;
  path?: Point[];
  x: number;
  y: number;
  found: number;
  correct: boolean;
  layout?: Layout;
  at: string;
}

export interface GameView {
  id: string;
  phase: Phase;
  you: number;
  first: number;
  turn: number;
  solver: number;
  baseMs: number;
  incMs: number;
  setupMs: number;
  setupLeftMs: number;
  players: [PlayerView | null, PlayerView | null];
  myLayout?: Layout;
  actions: ActionView[];
  result?: { winner: number; reason: Reason };
  layouts?: [Layout, Layout];
  /** À la fin : où chacun en était de la mine d'en face, et les gemmes à leur place. */
  drafts?: [Draft | null, Draft | null];
  rematch: [boolean, boolean];
  next?: string;
  public: boolean;
  canceled?: boolean;
}

export interface Draft {
  layout: Layout;
  found: boolean[];
}

export interface CurrentGame {
  id: string;
  phase: "waiting" | "setup" | "playing";
  seat: number;
  /** Absent : en attente d'un adversaire. */
  opponent?: string;
  yourMove: boolean;
  created: string;
}

export interface OpenGame {
  id: string;
  host: string;
  account: boolean;
  baseMs: number;
  incMs: number;
  created: string;
}

export interface HistoryItem {
  id: string;
  opponent: string;
  outcome: "win" | "loss" | "draw";
  reason: Reason;
  actions: number;
  at: string;
}

export interface Me {
  name: string;
  stats: { played: number; won: number; lost: number; drawn: number };
  games: HistoryItem[];
}

/** Méthode de connexion choisie par l'hébergeur (gamekit/auth). */
export interface AuthInfo {
  mode: string;
  /** « Se connecter avec … » ; absent : « Se connecter ». */
  accountName?: string;
  /** Jouer sans compte est permis. */
  guestPlay: boolean;
  oidc?: { issuer: string; clientId: string };
}

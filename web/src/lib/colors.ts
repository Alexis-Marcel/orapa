// Couleurs des gemmes et des ondes. Masque : rouge 1, jaune 2, bleu 4, blanc 8.

export const RED = 1;
export const YELLOW = 2;
export const BLUE = 4;
export const WHITE = 8;

export const GEM_HEX: Record<number, string> = {
  [RED]: "#e5484d",
  [YELLOW]: "#f2c94c",
  [BLUE]: "#3e7bfa",
  [WHITE]: "#eeeef6",
};

const WAVES: Record<number, [string, string]> = {
  0: ["transparente", "transparent"],
  [RED]: ["rouge", "#e5484d"],
  [YELLOW]: ["jaune", "#f2c94c"],
  [BLUE]: ["bleue", "#3e7bfa"],
  [WHITE]: ["blanche", "#eeeef6"],
  [RED | BLUE]: ["violette", "#8e4ec6"],
  [BLUE | WHITE]: ["bleu ciel", "#8ec8f6"],
  [BLUE | YELLOW]: ["verte", "#30a46c"],
  [RED | WHITE]: ["rose", "#f49cb4"],
  [RED | YELLOW]: ["orange", "#f2994a"],
  [YELLOW | WHITE]: ["citron", "#f5e98a"],
  [RED | BLUE | WHITE]: ["violet clair", "#c3a6e8"],
  [RED | BLUE | YELLOW]: ["noire", "#101118"],
  [BLUE | YELLOW | WHITE]: ["vert clair", "#9fddb0"],
  [RED | YELLOW | WHITE]: ["orange clair", "#f8c291"],
  [RED | YELLOW | BLUE | WHITE]: ["grise", "#8b8d98"],
};

export function waveName(colors: number): string {
  return WAVES[colors & 15]?.[0] ?? "inconnue";
}

export function waveHex(colors: number): string {
  return WAVES[colors & 15]?.[1] ?? "#888";
}

export function gemName(color: number): string {
  switch (color) {
    case RED:
      return "rouge";
    case YELLOW:
      return "jaune";
    case BLUE:
      return "bleue";
    case WHITE:
      return "blanche";
  }
  return "";
}

/** Composantes d'un mélange, dans l'ordre rouge, jaune, bleu, blanc. */
export function components(colors: number): number[] {
  return [RED, YELLOW, BLUE, WHITE].filter((c) => colors & c);
}

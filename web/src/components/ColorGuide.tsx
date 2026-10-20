import { WAVE_GUIDE, waveHex, waveName, waveRecipe } from "../lib/colors";

/** Correspondance entre la couleur d'une onde et les gemmes qu'elle a touchées. */
export function ColorGuide() {
  return (
    <ul className="mixes">
      {WAVE_GUIDE.map((c) => (
        <li key={c}>
          <span className={`swatch ${c ? "" : "swatch-clear"}`} style={{ background: c ? waveHex(c) : undefined }} />
          <strong>{waveName(c)}</strong>
          <span className="muted">{waveRecipe(c)}</span>
        </li>
      ))}
    </ul>
  );
}

import { useEffect, useState } from "react";

export function useNow(active: boolean, every = 250) {
  const [now, setNow] = useState(() => performance.now());
  useEffect(() => {
    if (!active) return;
    const t = setInterval(() => setNow(performance.now()), every);
    return () => clearInterval(t);
  }, [active, every]);
  return now;
}

export function fmt(ms: number) {
  const s = Math.max(0, Math.ceil(ms / 1000));
  const m = Math.floor(s / 60);
  return `${m}:${String(s % 60).padStart(2, "0")}`;
}

/** Temps restant, décompté depuis la réception de l'état quand la pendule tourne. */
export function Clock({ ms, running, receivedAt }: { ms: number; running: boolean; receivedAt: number }) {
  const now = useNow(running);
  const left = running ? ms - (now - receivedAt) : ms;
  return (
    <span className={`clock ${running ? "is-running" : ""} ${left < 30_000 ? "is-low" : ""}`} aria-label={`${fmt(left)} restantes`}>
      {fmt(left)}
    </span>
  );
}

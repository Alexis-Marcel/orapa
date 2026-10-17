package server

import (
	"time"

	"github.com/alexis-marcel/orapa/internal/game"
)

// La vue d'une partie dépend de qui regarde : chacun voit sa propre pose et
// le trajet des ondes envoyées dans sa mine, jamais ceux de l'adversaire. Tout
// est dévoilé à la fin.

type playerView struct {
	Name        string `json:"name"`
	Account     bool   `json:"account"`
	Ready       bool   `json:"ready"`
	Online      bool   `json:"online"`
	Turns       int    `json:"turns"`
	RemainingMs int64  `json:"remainingMs"`
}

type actionView struct {
	Seat     int          `json:"seat"`
	Kind     string       `json:"kind"`
	Entry    int          `json:"entry"`
	Exit     int          `json:"exit"`
	Colors   game.Color   `json:"colors"`
	Absorbed bool         `json:"absorbed,omitempty"`
	Path     []game.Point `json:"path,omitempty"`
	X        int          `json:"x"`
	Y        int          `json:"y"`
	Found    game.Color   `json:"found"`
	Correct  bool         `json:"correct"`
	Layout   game.Layout  `json:"layout,omitempty"`
	At       time.Time    `json:"at"`
}

type gameView struct {
	ID          string          `json:"id"`
	Phase       string          `json:"phase"`
	You         int             `json:"you"`
	First       int             `json:"first"`
	Turn        int             `json:"turn"`
	Solver      int             `json:"solver"`
	BaseMs      int64           `json:"baseMs"`
	IncMs       int64           `json:"incMs"`
	SetupMs     int64           `json:"setupMs"`
	SetupLeftMs int64           `json:"setupLeftMs"`
	Players     [2]*playerView  `json:"players"`
	MyLayout    game.Layout     `json:"myLayout,omitempty"`
	Actions     []actionView    `json:"actions"`
	Result      *game.Result    `json:"result,omitempty"`
	Layouts     *[2]game.Layout `json:"layouts,omitempty"`
	Rematch     [2]bool         `json:"rematch"`
	Next        string          `json:"next,omitempty"`
	Public      bool            `json:"public"`
	Canceled    bool            `json:"canceled,omitempty"`
}

func (g *live) view(seat int, now time.Time) gameView {
	r, m := g.rec, g.rec.Match
	phase := string(m.Phase)
	if r.Canceled {
		phase = "canceled"
	}
	v := gameView{
		ID: r.ID, Phase: phase, You: seat, First: m.First, Turn: m.Turn, Solver: m.Solver,
		BaseMs: m.Config.Base.Milliseconds(), IncMs: m.Config.Increment.Milliseconds(), SetupMs: m.Config.SetupTime.Milliseconds(),
		Result: m.Result, Rematch: r.Rematch, Next: r.Next, Public: r.Public, Canceled: r.Canceled,
		Actions: []actionView{},
	}
	if m.Phase == game.Setup {
		v.SetupLeftMs = max(m.PhaseStart.Add(m.Config.SetupTime).Sub(now).Milliseconds(), 0)
	}
	for i, p := range m.Players {
		if p == nil {
			continue
		}
		rem := m.Remaining(i, now)
		if m.Phase == game.Waiting || m.Phase == game.Setup {
			rem = m.Config.Base
		}
		v.Players[i] = &playerView{Name: p.Name, Account: p.UserID != "", Ready: p.Ready, Online: g.online(i), Turns: p.Turns, RemainingMs: rem.Milliseconds()}
	}
	if seat >= 0 && m.Players[seat] != nil {
		v.MyLayout = m.Players[seat].Layout
	}
	done := m.Phase == game.Finished
	if done {
		v.Layouts = &[2]game.Layout{m.Players[0].Layout, m.Players[1].Layout}
	}
	for _, a := range m.Actions {
		// Une action vise la mine de l'autre : sa cible en voit le détail.
		reveal := done || (seat >= 0 && a.Seat != seat)
		av := actionView{Seat: a.Seat, Kind: string(a.Kind), X: a.X, Y: a.Y, Found: a.Found, Correct: a.Correct, At: a.At}
		if a.Shot != nil {
			av.Entry, av.Exit, av.Colors, av.Absorbed = a.Shot.Entry, a.Shot.Exit, a.Shot.Colors, a.Shot.Absorbed
			if reveal {
				av.Path = a.Shot.Path
			}
		}
		if a.Kind == game.ActSolve && (reveal || a.Seat == seat) {
			av.Layout = a.Layout
		}
		v.Actions = append(v.Actions, av)
	}
	return v
}

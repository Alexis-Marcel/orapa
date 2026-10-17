package game

import (
	"errors"
	"time"
)

// Une partie à deux : chacun cache ses gemmes pour l'autre (pose simultanée,
// limitée en temps), puis à tour de rôle chacun envoie une onde, sonde une case
// ou propose la disposition de l'adversaire. Chaque joueur a sa pendule.

type Phase string

const (
	Waiting  Phase = "waiting"  // en attente du deuxième joueur
	Setup    Phase = "setup"    // les deux cachent leurs gemmes
	Playing  Phase = "playing"  // la chasse
	Finished Phase = "finished" // terminée
)

type Config struct {
	Base      time.Duration `json:"base"`
	Increment time.Duration `json:"increment"`
	SetupTime time.Duration `json:"setupTime"`
}

var DefaultConfig = Config{Base: 10 * time.Minute, Increment: 5 * time.Second, SetupTime: 3 * time.Minute}

type Player struct {
	Name      string        `json:"name"`
	UserID    string        `json:"userId,omitempty"`
	Layout    Layout        `json:"layout,omitempty"`
	Ready     bool          `json:"ready"`
	Remaining time.Duration `json:"remaining"`
	Turns     int           `json:"turns"`

	board *Board
}

type ActionKind string

const (
	ActWave  ActionKind = "wave"
	ActProbe ActionKind = "probe"
	ActSolve ActionKind = "solve"
)

// Action est un coup joué par Seat contre la mine de l'adversaire.
type Action struct {
	Seat    int        `json:"seat"`
	Kind    ActionKind `json:"kind"`
	Shot    *Shot      `json:"shot,omitempty"`
	X       int        `json:"x,omitempty"`
	Y       int        `json:"y,omitempty"`
	Found   Color      `json:"found,omitempty"`
	Layout  Layout     `json:"layout,omitempty"`
	Correct bool       `json:"correct,omitempty"`
	At      time.Time  `json:"at"`
}

type Reason string

const (
	BySolve       Reason = "solved"        // a trouvé la disposition
	ByWrong       Reason = "wrong"         // l'adversaire s'est trompé
	ByTimeout     Reason = "timeout"       // l'adversaire est tombé
	ByResign      Reason = "resign"        // l'adversaire a abandonné
	BySetupTime   Reason = "setup-timeout" // l'adversaire n'a pas caché ses gemmes à temps
	ByBothSolved  Reason = "both-solved"   // nul : les deux ont trouvé
	ByBothNoSetup Reason = "no-setup"      // nul : aucun n'a caché ses gemmes
)

type Result struct {
	Winner int    `json:"winner"` // -1 : nul
	Reason Reason `json:"reason"`
}

type Match struct {
	Config  Config     `json:"config"`
	Phase   Phase      `json:"phase"`
	Players [2]*Player `json:"players"`
	First   int        `json:"first"`
	Turn    int        `json:"turn"`
	// Solver est le joueur qui a trouvé en premier alors que l'autre avait un
	// tour de moins : l'autre a une dernière chance (seulement proposer). -1 sinon.
	Solver     int       `json:"solver"`
	Actions    []Action  `json:"actions"`
	Result     *Result   `json:"result,omitempty"`
	PhaseStart time.Time `json:"phaseStart"`
	TurnStart  time.Time `json:"turnStart"`
}

var (
	ErrSeatTaken   = errors.New("place déjà prise")
	ErrBadSeat     = errors.New("place inconnue")
	ErrPhase       = errors.New("impossible à ce moment de la partie")
	ErrNotYourTurn = errors.New("ce n'est pas ton tour")
	ErrAlreadySet  = errors.New("tes gemmes sont déjà cachées")
	ErrBadEntry    = errors.New("point d'entrée inconnu")
	ErrBadCell     = errors.New("case hors de la mine")
	ErrLastChance  = errors.New("dernière chance : il ne reste qu'à proposer une solution")
	ErrKnownWave   = errors.New("cette onde est déjà connue")
	ErrOver        = errors.New("la partie est terminée")
)

func NewMatch(cfg Config, first int) *Match {
	return &Match{Config: cfg, Phase: Waiting, First: first, Turn: first, Solver: -1}
}

// Sit installe un joueur. Quand les deux places sont prises, la pose commence.
func (m *Match) Sit(seat int, name, userID string, now time.Time) error {
	if m.Phase != Waiting {
		return ErrPhase
	}
	if seat < 0 || seat > 1 {
		return ErrBadSeat
	}
	if m.Players[seat] != nil {
		return ErrSeatTaken
	}
	m.Players[seat] = &Player{Name: name, UserID: userID}
	if m.Players[0] != nil && m.Players[1] != nil {
		m.startSetup(now)
	}
	return nil
}

func (m *Match) startSetup(now time.Time) {
	m.Phase = Setup
	m.PhaseStart = now
}

// Hide cache les gemmes d'un joueur. Définitif : la pose est vérifiée puis verrouillée.
func (m *Match) Hide(seat int, l Layout, now time.Time) error {
	if err := m.Tick(now); err != nil {
		return err
	}
	if m.Phase != Setup {
		return ErrPhase
	}
	p, err := m.player(seat)
	if err != nil {
		return err
	}
	if p.Ready {
		return ErrAlreadySet
	}
	b, err := l.Validate()
	if err != nil {
		return err
	}
	p.Layout, p.board, p.Ready = l, b, true
	if m.Players[0].Ready && m.Players[1].Ready {
		m.Phase = Playing
		m.PhaseStart, m.TurnStart = now, now
		m.Turn = m.First
		for _, pl := range m.Players {
			pl.Remaining = m.Config.Base
		}
	}
	return nil
}

func (m *Match) player(seat int) (*Player, error) {
	if seat < 0 || seat > 1 || m.Players[seat] == nil {
		return nil, ErrBadSeat
	}
	return m.Players[seat], nil
}

// Board renvoie la mine cachée d'un joueur (nil avant sa pose).
func (m *Match) Board(seat int) *Board {
	p := m.Players[seat]
	if p == nil || !p.Ready {
		return nil
	}
	if p.board == nil {
		// Partie rechargée : la mine se reconstruit depuis la pose.
		p.board, _ = p.Layout.Build()
	}
	return p.board
}

// Remaining donne le temps qu'il reste à un joueur à l'instant now.
func (m *Match) Remaining(seat int, now time.Time) time.Duration {
	p := m.Players[seat]
	if p == nil {
		return 0
	}
	r := p.Remaining
	if m.Phase == Playing && m.Turn == seat {
		r -= now.Sub(m.TurnStart)
	}
	return max(r, 0)
}

// Deadline est le prochain instant où la partie peut se terminer d'elle-même
// (drapeau ou fin du temps de pose), zéro s'il n'y en a pas.
func (m *Match) Deadline() time.Time {
	switch m.Phase {
	case Setup:
		return m.PhaseStart.Add(m.Config.SetupTime)
	case Playing:
		return m.TurnStart.Add(m.Players[m.Turn].Remaining)
	}
	return time.Time{}
}

// Tick applique le temps : drapeau tombé, ou temps de pose écoulé. Ne renvoie
// une erreur que pour signaler que la partie vient de finir ainsi.
func (m *Match) Tick(now time.Time) error {
	switch m.Phase {
	case Setup:
		if now.Before(m.PhaseStart.Add(m.Config.SetupTime)) {
			return nil
		}
		r0, r1 := m.Players[0].Ready, m.Players[1].Ready
		switch {
		case !r0 && !r1:
			m.finish(-1, ByBothNoSetup, now)
		case !r0:
			m.finish(1, BySetupTime, now)
		default:
			m.finish(0, BySetupTime, now)
		}
		return ErrOver
	case Playing:
		if m.Remaining(m.Turn, now) > 0 {
			return nil
		}
		m.Players[m.Turn].Remaining = 0
		winner := 1 - m.Turn
		if m.Solver >= 0 {
			// Tomber pendant sa dernière chance laisse la victoire à celui qui a trouvé.
			winner = m.Solver
		}
		m.finish(winner, ByTimeout, now)
		return ErrOver
	}
	return nil
}

func (m *Match) finish(winner int, reason Reason, now time.Time) {
	if m.Phase == Playing {
		p := m.Players[m.Turn]
		p.Remaining = max(p.Remaining-now.Sub(m.TurnStart), 0)
	}
	m.Phase = Finished
	m.PhaseStart = now
	m.Result = &Result{Winner: winner, Reason: reason}
}

func (m *Match) startTurn(seat int, now time.Time) error {
	if err := m.Tick(now); err != nil {
		return err
	}
	if m.Phase != Playing {
		return ErrPhase
	}
	if seat != m.Turn {
		return ErrNotYourTurn
	}
	return nil
}

func (m *Match) endTurn(now time.Time) {
	p := m.Players[m.Turn]
	p.Remaining = p.Remaining - now.Sub(m.TurnStart) + m.Config.Increment
	p.Turns++
	m.Turn = 1 - m.Turn
	m.TurnStart = now
}

// Known dit si l'onde partie de e est déjà connue de seat : déjà envoyée, ou
// point de sortie d'une onde déjà envoyée (le trajet inverse est le même).
func (m *Match) Known(seat, e int) bool {
	for _, a := range m.Actions {
		if a.Seat == seat && a.Kind == ActWave && (a.Shot.Entry == e || a.Shot.Exit == e) {
			return true
		}
	}
	return false
}

// Wave envoie une onde dans la mine de l'adversaire.
func (m *Match) Wave(seat, e int, now time.Time) (*Shot, error) {
	if err := m.startTurn(seat, now); err != nil {
		return nil, err
	}
	if m.Solver >= 0 {
		return nil, ErrLastChance
	}
	if e < 0 || e >= Entries {
		return nil, ErrBadEntry
	}
	if m.Known(seat, e) {
		return nil, ErrKnownWave
	}
	shot := m.Board(1 - seat).Fire(e)
	m.Actions = append(m.Actions, Action{Seat: seat, Kind: ActWave, Shot: &shot, At: now})
	m.endTurn(now)
	return &shot, nil
}

// Probe demande ce qu'il y a sur une case de la mine de l'adversaire.
func (m *Match) Probe(seat, x, y int, now time.Time) (Color, error) {
	if err := m.startTurn(seat, now); err != nil {
		return 0, err
	}
	if m.Solver >= 0 {
		return 0, ErrLastChance
	}
	if !inside(x, y) {
		return 0, ErrBadCell
	}
	c := m.Board(1-seat).Probe(x, y)
	m.Actions = append(m.Actions, Action{Seat: seat, Kind: ActProbe, X: x, Y: y, Found: c, At: now})
	m.endTurn(now)
	return c, nil
}

// Solve propose la disposition de l'adversaire. Une proposition qui n'est pas
// une pose valide est refusée sans coûter le tour ; une proposition fausse fait
// perdre aussitôt. Si le premier joueur trouve, l'autre a une dernière chance
// de trouver aussi (nul).
func (m *Match) Solve(seat int, l Layout, now time.Time) (bool, error) {
	if err := m.startTurn(seat, now); err != nil {
		return false, err
	}
	b, err := l.Validate()
	if err != nil {
		return false, err
	}
	correct := b.Equal(m.Board(1 - seat))
	m.Actions = append(m.Actions, Action{Seat: seat, Kind: ActSolve, Layout: l, Correct: correct, At: now})
	switch {
	case m.Solver >= 0 && correct:
		m.finish(-1, ByBothSolved, now)
	case m.Solver >= 0:
		m.finish(m.Solver, BySolve, now)
	case !correct:
		m.finish(1-seat, ByWrong, now)
	case m.Players[1-seat].Turns <= m.Players[seat].Turns:
		// L'adversaire a joué un tour de moins : il a droit à sa dernière chance.
		m.Solver = seat
		m.endTurn(now)
	default:
		m.finish(seat, BySolve, now)
	}
	return correct, nil
}

// Resign abandonne (à tout moment de la pose ou de la chasse).
func (m *Match) Resign(seat int, now time.Time) error {
	if err := m.Tick(now); err != nil {
		return err
	}
	if m.Phase != Setup && m.Phase != Playing {
		return ErrPhase
	}
	if _, err := m.player(seat); err != nil {
		return err
	}
	m.finish(1-seat, ByResign, now)
	return nil
}

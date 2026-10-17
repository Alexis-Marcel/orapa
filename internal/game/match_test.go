package game

import (
	"errors"
	"math/rand/v2"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)

var other = Layout{
	{Kind: WhiteTriangle, O: 2, X: 6, Y: 0},
	{Kind: BlueTriangle, O: 1, X: 0, Y: 2},
	{Kind: YellowTri, O: 3, X: 4, Y: 5},
	{Kind: WhiteDiamond, O: 0, X: 7, Y: 5},
	{Kind: RedParallelo, O: 2, X: 3, Y: 2},
}

// playing renvoie une partie lancée : joueur 0 = sample, joueur 1 = other, 0 commence.
func playing(t *testing.T) *Match {
	t.Helper()
	if _, err := other.Validate(); err != nil {
		t.Fatalf("pose other : %v", err)
	}
	m := NewMatch(DefaultConfig, 0)
	must(t, m.Sit(0, "Alice", "u-a", t0))
	if err := m.Sit(0, "X", "", t0); !errors.Is(err, ErrSeatTaken) {
		t.Fatal("place prise acceptée")
	}
	must(t, m.Sit(1, "Bob", "", t0))
	if m.Phase != Setup {
		t.Fatal("pas en pose")
	}
	must(t, m.Hide(0, sample, t0.Add(time.Second)))
	if err := m.Hide(0, sample, t0.Add(time.Second)); !errors.Is(err, ErrAlreadySet) {
		t.Fatal("pose deux fois")
	}
	must(t, m.Hide(1, other, t0.Add(2*time.Second)))
	if m.Phase != Playing || m.Turn != 0 {
		t.Fatalf("phase %s tour %d", m.Phase, m.Turn)
	}
	return m
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func at(s int) time.Time { return t0.Add(2*time.Second + time.Duration(s)*time.Second) }

func TestSitErrors(t *testing.T) {
	m := NewMatch(DefaultConfig, 0)
	if err := m.Sit(2, "x", "", t0); !errors.Is(err, ErrBadSeat) {
		t.Error("place 2")
	}
	m.Phase = Playing
	if err := m.Sit(0, "x", "", t0); !errors.Is(err, ErrPhase) {
		t.Error("assis en cours de partie")
	}
	m.Phase = Waiting
	if m.Deadline() != (time.Time{}) || m.Remaining(0, t0) != 0 {
		t.Error("sans joueurs")
	}
}

func TestHideErrors(t *testing.T) {
	m := NewMatch(DefaultConfig, 0)
	if err := m.Hide(0, sample, t0); !errors.Is(err, ErrPhase) {
		t.Error("pose en attente")
	}
	must(t, m.Sit(0, "A", "", t0))
	must(t, m.Sit(1, "B", "", t0))
	if err := m.Hide(0, sample[:3], t0); !errors.Is(err, ErrIncomplete) {
		t.Error("pose incomplète acceptée")
	}
	if err := m.Hide(3, sample, t0); !errors.Is(err, ErrBadSeat) {
		t.Error("place inconnue")
	}
	if m.Board(0) != nil {
		t.Error("mine avant la pose")
	}
}

func TestWaveProbeAndClock(t *testing.T) {
	m := playing(t)
	if _, err := m.Wave(1, 0, at(1)); !errors.Is(err, ErrNotYourTurn) {
		t.Error("hors tour")
	}
	s, err := m.Wave(0, 9, at(10))
	must(t, err)
	if s.Entry != 9 || m.Turn != 1 {
		t.Errorf("onde %+v tour %d", s, m.Turn)
	}
	// 10 s consommées, 5 s rendues.
	if got := m.Players[0].Remaining; got != DefaultConfig.Base-5*time.Second {
		t.Errorf("pendule %v", got)
	}
	if got := m.Remaining(1, at(13)); got != DefaultConfig.Base-3*time.Second {
		t.Errorf("pendule qui tourne %v", got)
	}
	c, err := m.Probe(1, 1, 1, at(20))
	must(t, err)
	if c != White {
		t.Errorf("sondage %b", c)
	}
	if _, err := m.Probe(0, 10, 0, at(21)); !errors.Is(err, ErrBadCell) {
		t.Error("case hors mine")
	}
	if _, err := m.Wave(0, 36, at(21)); !errors.Is(err, ErrBadEntry) {
		t.Error("entrée inconnue")
	}
	// Onde déjà envoyée, ou depuis sa sortie : déjà connue.
	if _, err := m.Wave(0, 9, at(21)); !errors.Is(err, ErrKnownWave) {
		t.Error("onde répétée")
	}
	if _, err := m.Wave(0, s.Exit, at(21)); !errors.Is(err, ErrKnownWave) {
		t.Error("onde inverse")
	}
	if len(m.Actions) != 2 || m.Actions[1].Kind != ActProbe {
		t.Errorf("journal %+v", m.Actions)
	}
	if m.Deadline() != at(20).Add(m.Players[0].Remaining) {
		t.Error("échéance")
	}
}

func TestSecondSolvesWins(t *testing.T) {
	m := playing(t)
	_, _ = m.Wave(0, 0, at(1))
	ok, err := m.Solve(1, sample, at(2))
	must(t, err)
	if !ok || m.Phase != Finished || m.Result.Winner != 1 || m.Result.Reason != BySolve {
		t.Fatalf("%+v", m.Result)
	}
	if _, err := m.Wave(0, 3, at(3)); !errors.Is(err, ErrPhase) {
		t.Error("onde après la fin")
	}
}

func TestFirstSolvesLastChance(t *testing.T) {
	// Le premier trouve : l'autre n'a plus que sa proposition.
	m := playing(t)
	ok, err := m.Solve(0, other, at(1))
	must(t, err)
	if !ok || m.Phase != Playing || m.Solver != 0 || m.Turn != 1 {
		t.Fatalf("phase %s solver %d", m.Phase, m.Solver)
	}
	if _, err := m.Wave(1, 0, at(2)); !errors.Is(err, ErrLastChance) {
		t.Error("onde en dernière chance")
	}
	if _, err := m.Probe(1, 0, 0, at(2)); !errors.Is(err, ErrLastChance) {
		t.Error("sondage en dernière chance")
	}

	t.Run("les deux trouvent : nul", func(t *testing.T) {
		m := *m
		m.Players = [2]*Player{ptr(*m.Players[0]), ptr(*m.Players[1])}
		ok, err := m.Solve(1, sample, at(3))
		must(t, err)
		if !ok || m.Result.Winner != -1 || m.Result.Reason != ByBothSolved {
			t.Fatalf("%+v", m.Result)
		}
	})
	t.Run("le second se trompe", func(t *testing.T) {
		m := *m
		m.Players = [2]*Player{ptr(*m.Players[0]), ptr(*m.Players[1])}
		ok, err := m.Solve(1, other, at(3))
		must(t, err)
		if ok || m.Result.Winner != 0 || m.Result.Reason != BySolve {
			t.Fatalf("%+v", m.Result)
		}
	})
	t.Run("le second tombe", func(t *testing.T) {
		m := *m
		m.Players = [2]*Player{ptr(*m.Players[0]), ptr(*m.Players[1])}
		if err := m.Tick(at(2).Add(time.Hour)); !errors.Is(err, ErrOver) {
			t.Fatal("pas de drapeau")
		}
		if m.Result.Winner != 0 || m.Result.Reason != ByTimeout {
			t.Fatalf("%+v", m.Result)
		}
	})
}

func ptr[T any](v T) *T { return &v }

func TestWrongSolutionLoses(t *testing.T) {
	m := playing(t)
	ok, err := m.Solve(0, sample, at(1)) // sa propre pose, pas celle de Bob
	must(t, err)
	if ok || m.Result.Winner != 1 || m.Result.Reason != ByWrong {
		t.Fatalf("%+v", m.Result)
	}
}

func TestInvalidSolutionCostsNothing(t *testing.T) {
	m := playing(t)
	if _, err := m.Solve(0, sample[:2], at(1)); err == nil {
		t.Fatal("proposition incomplète acceptée")
	}
	if m.Phase != Playing || m.Turn != 0 || len(m.Actions) != 0 {
		t.Error("proposition invalide comptée")
	}
}

func TestTimeout(t *testing.T) {
	m := playing(t)
	must(t, m.Tick(at(60)))
	if _, err := m.Wave(0, 0, at(601)); !errors.Is(err, ErrOver) {
		t.Fatal("pas de drapeau")
	}
	if m.Result.Winner != 1 || m.Result.Reason != ByTimeout || m.Players[0].Remaining != 0 {
		t.Fatalf("%+v", m.Result)
	}
	must(t, m.Tick(at(700))) // déjà finie : rien
}

func TestSetupTimeout(t *testing.T) {
	for _, c := range []struct {
		ready  []int
		winner int
		reason Reason
	}{
		{nil, -1, ByBothNoSetup},
		{[]int{0}, 0, BySetupTime},
		{[]int{1}, 1, BySetupTime},
	} {
		m := NewMatch(DefaultConfig, 0)
		must(t, m.Sit(0, "A", "", t0))
		must(t, m.Sit(1, "B", "", t0))
		l := [2]Layout{sample, other}
		for _, s := range c.ready {
			must(t, m.Hide(s, l[s], t0))
		}
		if m.Deadline() != t0.Add(DefaultConfig.SetupTime) {
			t.Error("échéance de pose")
		}
		must(t, m.Tick(t0.Add(time.Minute)))
		if err := m.Hide(1, other, t0.Add(4*time.Minute)); !errors.Is(err, ErrOver) {
			t.Error("pose après le temps")
		}
		if m.Result.Winner != c.winner || m.Result.Reason != c.reason {
			t.Errorf("%v : %+v", c.ready, m.Result)
		}
	}
}

func TestResign(t *testing.T) {
	m := playing(t)
	if err := m.Resign(5, at(1)); !errors.Is(err, ErrBadSeat) {
		t.Error("place inconnue")
	}
	must(t, m.Resign(1, at(1)))
	if m.Result.Winner != 0 || m.Result.Reason != ByResign {
		t.Fatalf("%+v", m.Result)
	}
	if err := m.Resign(0, at(2)); !errors.Is(err, ErrPhase) {
		t.Error("abandon après la fin")
	}
	m2 := NewMatch(DefaultConfig, 0)
	if err := m2.Resign(0, t0); !errors.Is(err, ErrPhase) {
		t.Error("abandon en attente")
	}
	must(t, m2.Sit(0, "A", "", t0))
	must(t, m2.Sit(1, "B", "", t0))
	if err := m2.Tick(t0.Add(time.Hour)); !errors.Is(err, ErrOver) {
		t.Error("temps de pose")
	}
	if err := m2.Resign(0, t0.Add(time.Hour)); !errors.Is(err, ErrPhase) {
		t.Error("abandon après la fin de la pose")
	}
}

// Une partie rechargée (sans mines en mémoire) reconstruit les mines depuis les poses.
func TestBoardRebuilt(t *testing.T) {
	m := playing(t)
	m.Players[1].board = nil
	if m.Board(1) == nil || m.Board(1).Probe(0, 2) == 0 {
		t.Fatal("mine non reconstruite")
	}
	if m.Known(0, 99) {
		t.Error("onde inconnue")
	}
}

func TestRandomLayoutValid(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	for range 50 {
		if _, err := RandomLayout(rng).Validate(); err != nil {
			t.Fatal(err)
		}
	}
}

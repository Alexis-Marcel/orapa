package game

import (
	"errors"
	"math/rand/v2"
	"strings"
	"testing"
)

// sample est une pose valide de référence :
//
//	   0 1 2 3 4 5 6 7 8 9
//	0  . ◢ ◣ . . . . . . .
//	1  ◢ ■ ■ ◣ . . . . ◢ .
//	2  . . . . . . . ◢ ■ .
//	3  . . . . . . . . . .
//	4  . . . ◢ ◣ . . . . .
//	5  ◢ ■ ◤ ◥ ◤ . . . . .
//	6  . . . . . . ◢ ◣ . .
//	7  . . . . . ◢ ■ ■ ◣ .
var sample = Layout{
	{Kind: WhiteTriangle, O: 0, X: 0, Y: 0},
	{Kind: YellowTri, O: 0, X: 7, Y: 1},
	{Kind: WhiteDiamond, O: 0, X: 3, Y: 4},
	{Kind: RedParallelo, O: 0, X: 0, Y: 5},
	{Kind: BlueTriangle, O: 0, X: 5, Y: 6},
}

func mustBoard(t *testing.T, l Layout) *Board {
	t.Helper()
	b, err := l.Validate()
	if err != nil {
		t.Fatalf("pose refusée : %v", err)
	}
	return b
}

func TestOrientationCounts(t *testing.T) {
	want := map[Kind]int{WhiteTriangle: 4, BlueTriangle: 4, YellowTri: 4, WhiteDiamond: 1, RedParallelo: 4}
	for k, n := range want {
		if got := len(Pieces[k].Orientations); got != n {
			t.Errorf("%s : %d orientations, attendu %d", k, got, n)
		}
	}
	if len(PieceList()) != len(BaseSet) {
		t.Fatal("PieceList incomplète")
	}
}

// L'aire de chaque orientation se conserve et toutes les cases restent dans la boîte.
func TestOrientationsWellFormed(t *testing.T) {
	area := map[Kind]float64{WhiteTriangle: 4, BlueTriangle: 4, YellowTri: 2, WhiteDiamond: 2, RedParallelo: 2}
	for k, def := range Pieces {
		for i, o := range def.Orientations {
			a := 0.0
			for _, c := range o.Cells {
				if c.X < 0 || c.Y < 0 || c.X >= o.W || c.Y >= o.H {
					t.Errorf("%s/%d : case (%d,%d) hors de la boîte", k, i, c.X, c.Y)
				}
				if c.Fill == Full {
					a++
				} else {
					a += 0.5
				}
			}
			if a != area[k] {
				t.Errorf("%s/%d : aire %v, attendu %v", k, i, a, area[k])
			}
			for _, p := range o.Outline {
				if p.X < 0 || p.Y < 0 || p.X > float64(o.W) || p.Y > float64(o.H) {
					t.Errorf("%s/%d : contour hors de la boîte", k, i)
				}
			}
		}
	}
}

func TestFire(t *testing.T) {
	b := mustBoard(t, sample)
	cases := []struct {
		name   string
		entry  int
		exit   int
		colors Color
	}{
		{"colonne vide", 9, 27, 0},
		{"diagonale vers la gauche", 0, 29, White},
		{"trajet inverse", 29, 0, White},
		{"face droite : demi-tour", 24, 24, Blue},
		{"diagonale vers le haut", 30, 7, Yellow},
		{"pointe du triangle", 1, 28, White},
	}
	for _, c := range cases {
		s := b.Fire(c.entry)
		if s.Exit != c.exit || s.Colors != c.colors {
			t.Errorf("%s : entrée %s → sortie %d couleurs %b, attendu %d %b", c.name, EntryLabel(c.entry), s.Exit, s.Colors, c.exit, c.colors)
		}
		if len(s.Path) < 2 {
			t.Errorf("%s : tracé trop court", c.name)
		}
	}
}

// Un trajet est réversible : partir de la sortie ramène à l'entrée avec les
// mêmes couleurs. Vérifié sur des poses tirées au hasard.
func TestFireReversible(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for range 200 {
		l := RandomLayout(rng)
		b := mustBoard(t, l)
		for e := range Entries {
			s := b.Fire(e)
			back := b.Fire(s.Exit)
			if back.Exit != e || back.Colors != s.Colors {
				t.Fatalf("%v : %d→%d mais %d→%d", l, e, s.Exit, s.Exit, back.Exit)
			}
		}
	}
}

func TestExtensionsMaterials(t *testing.T) {
	b := newBoard()
	b.cells[0][0] = Cell{Fill: Full, Color: Transparent, Piece: 0}
	b.cells[0][5] = Cell{Fill: Full, Color: Black, Piece: 1}
	if s := b.Fire(0); s.Exit != 0 || s.Colors != 0 {
		t.Errorf("transparent : %+v", s)
	}
	if s := b.Fire(5); !s.Absorbed || s.Exit != -1 {
		t.Errorf("noir : %+v", s)
	}
}

func TestProbeAndEqual(t *testing.T) {
	b := mustBoard(t, sample)
	if b.Probe(1, 1) != White || b.Probe(2, 0) != White || b.Probe(0, 0) != 0 || b.Probe(8, 7) != Blue {
		t.Error("sondage faux")
	}
	if b.Probe(-1, 0) != 0 {
		t.Error("hors mine")
	}
	if !b.Equal(mustBoard(t, sample)) {
		t.Error("même pose différente")
	}
	moved := append(Layout{}, sample...)
	moved[4].X = 6
	if b.Equal(mustBoard(t, moved)) {
		t.Error("poses différentes égales")
	}
}

func TestValidate(t *testing.T) {
	with := func(i int, p Placement) Layout {
		l := append(Layout{}, sample...)
		l[i] = p
		return l
	}
	cases := []struct {
		name string
		l    Layout
	}{
		{"incomplète", sample[:4]},
		{"en double", with(1, Placement{Kind: WhiteTriangle, X: 5, Y: 2})},
		{"inconnue", with(1, Placement{Kind: "black", X: 5, Y: 2})},
		{"orientation", with(2, Placement{Kind: WhiteDiamond, O: 3, X: 3, Y: 4})},
		{"dépasse", with(4, Placement{Kind: BlueTriangle, X: 7, Y: 6})},
		{"chevauche", with(2, Placement{Kind: WhiteDiamond, X: 6, Y: 6})},
		// Le losange posé en (2,4) : sa case (2,5) ◥ … chevauche le parallélogramme.
		{"chevauche 2", with(2, Placement{Kind: WhiteDiamond, X: 2, Y: 4})},
		// Jaune retourné sous le triangle blanc : (1,2) plein sous (1,1) plein.
		{"côté commun", with(1, Placement{Kind: YellowTri, O: 2, X: 1, Y: 2})},
	}
	for _, c := range cases {
		if _, err := c.l.Validate(); err == nil {
			t.Errorf("%s : pose acceptée", c.name)
		}
	}
	// Une diagonale contre un côté ne fait pas un côté commun : le jaune (1,2) ◢
	// sous le triangle blanc ne le touche qu'en des points.
	if _, err := with(1, Placement{Kind: YellowTri, O: 0, X: 1, Y: 2}).Validate(); err != nil {
		t.Errorf("pose voisine refusée : %v", err)
	}
}

func TestHiddenPiece(t *testing.T) {
	// Le losange enfermé au milieu de quatre gemmes qui le masquent depuis
	// les quatre bords n'est pas visible.
	l := Layout{
		{Kind: WhiteDiamond, O: 0, X: 4, Y: 3},
		{Kind: WhiteTriangle, O: 0, X: 3, Y: 0}, // au-dessus : colonnes 3–6
		{Kind: BlueTriangle, O: 2, X: 3, Y: 6},  // au-dessous
		{Kind: YellowTri, O: 0, X: 0, Y: 3},     // à gauche, lignes 3–4
		{Kind: RedParallelo, O: 1, X: 8, Y: 2},  // à droite, debout
	}
	_, err := l.Validate()
	var le *LayoutError
	if !errors.As(err, &le) || !strings.Contains(le.Msg, "losange") {
		t.Fatalf("attendu une gemme cachée, obtenu %v", err)
	}
}

func TestEntryLabel(t *testing.T) {
	if EntryLabel(0) != "1" || EntryLabel(17) != "18" || EntryLabel(18) != "A" || EntryLabel(35) != "R" {
		t.Error("libellés")
	}
}

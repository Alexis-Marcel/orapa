package game

import (
	"fmt"
	"slices"
	"strings"
)

// Les gemmes sont les pièces d'un tangram. Une forme est décrite par ses
// demi-cases et par son contour (pour l'affichage), dans une orientation de
// référence ; les autres orientations s'obtiennent par rotation (et miroir pour
// le parallélogramme).

type Kind string

const (
	WhiteTriangle Kind = "white-triangle"
	BlueTriangle  Kind = "blue-triangle"
	YellowTri     Kind = "yellow-triangle"
	WhiteDiamond  Kind = "white-diamond"
	RedParallelo  Kind = "red-parallelogram"
)

// BaseSet est la mine du jeu de base : 5 gemmes, chacune une fois.
var BaseSet = []Kind{WhiteTriangle, BlueTriangle, YellowTri, WhiteDiamond, RedParallelo}

type ShapeCell struct {
	X    int  `json:"x"`
	Y    int  `json:"y"`
	Fill Fill `json:"fill"`
}

type Orientation struct {
	W       int         `json:"w"`
	H       int         `json:"h"`
	Cells   []ShapeCell `json:"cells"`
	Outline []Point     `json:"outline"`
}

type PieceDef struct {
	Kind         Kind          `json:"kind"`
	Name         string        `json:"name"`
	Color        Color         `json:"color"`
	Orientations []Orientation `json:"orientations"`
}

var (
	bigTriangle = Orientation{W: 4, H: 2,
		Cells:   []ShapeCell{{1, 0, HalfSE}, {2, 0, HalfSW}, {0, 1, HalfSE}, {1, 1, Full}, {2, 1, Full}, {3, 1, HalfSW}},
		Outline: []Point{{0, 2}, {2, 0}, {4, 2}},
	}
	midTriangle = Orientation{W: 2, H: 2,
		Cells:   []ShapeCell{{1, 0, HalfSE}, {0, 1, HalfSE}, {1, 1, Full}},
		Outline: []Point{{0, 2}, {2, 0}, {2, 2}},
	}
	diamond = Orientation{W: 2, H: 2,
		Cells:   []ShapeCell{{0, 0, HalfSE}, {1, 0, HalfSW}, {0, 1, HalfNE}, {1, 1, HalfNW}},
		Outline: []Point{{1, 0}, {2, 1}, {1, 2}, {0, 1}},
	}
	parallelogram = Orientation{W: 3, H: 1,
		Cells:   []ShapeCell{{0, 0, HalfSE}, {1, 0, Full}, {2, 0, HalfNW}},
		Outline: []Point{{0, 1}, {1, 0}, {3, 0}, {2, 1}},
	}
)

var Pieces = map[Kind]*PieceDef{
	WhiteTriangle: {Kind: WhiteTriangle, Name: "grand triangle blanc", Color: White, Orientations: orientations(bigTriangle, false)},
	BlueTriangle:  {Kind: BlueTriangle, Name: "grand triangle bleu", Color: Blue, Orientations: orientations(bigTriangle, false)},
	YellowTri:     {Kind: YellowTri, Name: "triangle jaune", Color: Yellow, Orientations: orientations(midTriangle, false)},
	WhiteDiamond:  {Kind: WhiteDiamond, Name: "losange blanc", Color: White, Orientations: orientations(diamond, false)},
	RedParallelo:  {Kind: RedParallelo, Name: "parallélogramme rouge", Color: Red, Orientations: orientations(parallelogram, true)},
}

// PieceList renvoie les gemmes du jeu de base dans l'ordre de BaseSet.
func PieceList() []*PieceDef {
	out := make([]*PieceDef, len(BaseSet))
	for i, k := range BaseSet {
		out[i] = Pieces[k]
	}
	return out
}

var rotateFill = map[Fill]Fill{Full: Full, HalfNE: HalfSE, HalfSE: HalfSW, HalfSW: HalfNW, HalfNW: HalfNE}
var mirrorFill = map[Fill]Fill{Full: Full, HalfNE: HalfNW, HalfNW: HalfNE, HalfSE: HalfSW, HalfSW: HalfSE}

// rotate tourne une forme d'un quart de tour dans le sens des aiguilles d'une
// montre (y vers le bas).
func rotate(o Orientation) Orientation {
	r := Orientation{W: o.H, H: o.W}
	for _, c := range o.Cells {
		r.Cells = append(r.Cells, ShapeCell{o.H - 1 - c.Y, c.X, rotateFill[c.Fill]})
	}
	for _, p := range o.Outline {
		r.Outline = append(r.Outline, Point{float64(o.H) - p.Y, p.X})
	}
	return r
}

func mirror(o Orientation) Orientation {
	r := Orientation{W: o.W, H: o.H}
	for _, c := range o.Cells {
		r.Cells = append(r.Cells, ShapeCell{o.W - 1 - c.X, c.Y, mirrorFill[c.Fill]})
	}
	for _, p := range o.Outline {
		r.Outline = append(r.Outline, Point{float64(o.W) - p.X, p.Y})
	}
	return r
}

func (o Orientation) key() string {
	cells := slices.Clone(o.Cells)
	slices.SortFunc(cells, func(a, b ShapeCell) int {
		if a.Y != b.Y {
			return a.Y - b.Y
		}
		return a.X - b.X
	})
	var sb strings.Builder
	for _, c := range cells {
		fmt.Fprintf(&sb, "%d,%d,%d;", c.X, c.Y, c.Fill)
	}
	return sb.String()
}

// orientations énumère les orientations distinctes d'une forme, dans un ordre
// fixe : les rotations successives, puis celles du miroir.
func orientations(base Orientation, flip bool) []Orientation {
	var out []Orientation
	seen := map[string]bool{}
	add := func(o Orientation) {
		for range 4 {
			if k := o.key(); !seen[k] {
				seen[k] = true
				out = append(out, o)
			}
			o = rotate(o)
		}
	}
	add(base)
	if flip {
		add(mirror(base))
	}
	return out
}

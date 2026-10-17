// Package game est le moteur de règles d'Orapa : grille, gemmes, trajet des
// ondes, validation des poses et déroulé d'une partie à deux. Aucune I/O, aucune
// concurrence : l'heure est toujours passée en paramètre.
package game

// La mine fait 10 cases de large sur 8 de haut. Une case est vide, pleine, ou
// coupée en deux par une diagonale : une demi-case est le triangle rectangle
// dont l'angle droit est dans le coin indiqué.
const (
	Width  = 10
	Height = 8
	// Entries est le nombre de points d'entrée autour de la mine :
	// haut 0–9, droite 10–17, bas 18–27, gauche 28–35.
	Entries = 2 * (Width + Height)
)

type Fill uint8

const (
	Empty Fill = iota
	Full
	HalfNE // ◥
	HalfSE // ◢
	HalfSW // ◣
	HalfNW // ◤
)

// Côtés d'une case.
const (
	north = iota
	east
	south
	west
)

// sideFilled dit si la matière touche le côté sur toute sa longueur (une
// demi-case ne touche ses deux autres côtés qu'en un point).
func sideFilled(f Fill, side int) bool {
	switch f {
	case Full:
		return true
	case HalfNE:
		return side == north || side == east
	case HalfSE:
		return side == south || side == east
	case HalfSW:
		return side == south || side == west
	case HalfNW:
		return side == north || side == west
	}
	return false
}

// Color est un ensemble de couleurs de gemmes (masque de bits). La couleur
// d'une onde est l'union des couleurs des gemmes sur lesquelles elle a rebondi.
type Color uint8

const (
	Red Color = 1 << iota
	Yellow
	Blue
	White
	// Transparent et Black ne sont pas des couleurs d'onde : ce sont les
	// matières des extensions (Diamant renvoie sans colorer, Corps noir absorbe).
	Transparent
	Black
)

type Cell struct {
	Fill  Fill
	Color Color
	Piece int // indice de la pièce dans la pose, -1 si vide
}

type Board struct {
	cells [Height][Width]Cell
}

func newBoard() *Board {
	b := &Board{}
	for y := range Height {
		for x := range Width {
			b.cells[y][x].Piece = -1
		}
	}
	return b
}

func inside(x, y int) bool { return x >= 0 && x < Width && y >= 0 && y < Height }

func (b *Board) At(x, y int) Cell {
	if !inside(x, y) {
		return Cell{Piece: -1}
	}
	return b.cells[y][x]
}

// Probe répond à « qu'y a-t-il en (x, y) ? » : la couleur de la gemme, 0 si
// la case est vide.
func (b *Board) Probe(x, y int) Color { return b.At(x, y).Color }

// Equal compare deux mines case par case (forme et couleur).
func (b *Board) Equal(o *Board) bool {
	for y := range Height {
		for x := range Width {
			if b.cells[y][x].Fill != o.cells[y][x].Fill || b.cells[y][x].Color != o.cells[y][x].Color {
				return false
			}
		}
	}
	return true
}

// Point est un point du plan de la mine : (0,0) est le coin en haut à gauche,
// une case mesure 1. Les ondes passent toujours au milieu des cases.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Shot struct {
	Entry    int     `json:"entry"`
	Exit     int     `json:"exit"` // -1 si l'onde a été absorbée
	Colors   Color   `json:"colors"`
	Absorbed bool    `json:"absorbed,omitempty"`
	Path     []Point `json:"path"`
}

// entryStart donne la case (hors de la mine) d'où part l'onde et sa direction.
func entryStart(e int) (x, y, dx, dy int) {
	switch {
	case e < Width:
		return e, -1, 0, 1
	case e < Width+Height:
		return Width, e - Width, -1, 0
	case e < 2*Width+Height:
		return e - Width - Height, Height, 0, -1
	default:
		return -1, e - 2*Width - Height, 1, 0
	}
}

// exitAt donne le point d'entrée correspondant à une case juste hors de la mine.
func exitAt(x, y int) int {
	switch {
	case y < 0:
		return x
	case x >= Width:
		return Width + y
	case y >= Height:
		return Width + Height + x
	default:
		return 2*Width + Height + y
	}
}

func center(x, y int) Point { return Point{float64(x) + 0.5, float64(y) + 0.5} }

// sideOf donne le côté d'une case par lequel on sort en allant dans (dx, dy).
func sideOf(dx, dy int) int {
	switch {
	case dy < 0:
		return north
	case dx > 0:
		return east
	case dy > 0:
		return south
	}
	return west
}

var sideDir = [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}

// Fire envoie une onde depuis le point d'entrée e. L'onde va tout droit ; sur
// une face droite elle repart d'où elle vient, sur une face en diagonale elle
// tourne de 90°. Elle prend la couleur de chaque gemme touchée.
func (b *Board) Fire(e int) Shot {
	x, y, dx, dy := entryStart(e)
	shot := Shot{Entry: e, Exit: -1}
	// (x, y) est la case où l'onde va entrer ; edge est le point où elle y entre.
	x, y = x+dx, y+dy
	edge := func() Point {
		p := center(x, y)
		p.X -= float64(dx) * 0.5
		p.Y -= float64(dy) * 0.5
		return p
	}
	shot.Path = append(shot.Path, edge())

	for range 4 * Width * Height * 4 {
		if !inside(x, y) {
			shot.Path = append(shot.Path, edge())
			shot.Exit = exitAt(x, y)
			return shot
		}
		c := b.cells[y][x]
		if c.Fill == Empty {
			x, y = x+dx, y+dy
			continue
		}
		if c.Color&Black != 0 {
			shot.Path = append(shot.Path, edge())
			shot.Absorbed = true
			return shot
		}
		shot.Colors |= c.Color &^ Transparent
		in := sideOf(-dx, -dy) // côté de la case par lequel l'onde arrive
		if sideFilled(c.Fill, in) {
			// Face droite : demi-tour sur le bord, l'onde rentre dans la case
			// d'où elle venait (et peut y retoucher une diagonale).
			shot.Path = append(shot.Path, edge())
			dx, dy = -dx, -dy
			x, y = x+dx, y+dy
			continue
		}
		// Face en diagonale : l'onde la touche au centre de la case et ressort
		// par l'autre côté libre.
		shot.Path = append(shot.Path, center(x, y))
		for side := range 4 {
			if side != in && !sideFilled(c.Fill, side) {
				dx, dy = sideDir[side][0], sideDir[side][1]
				break
			}
		}
		x, y = x+dx, y+dy
	}
	panic("orapa: onde sans fin") // impossible : un trajet est réversible
}

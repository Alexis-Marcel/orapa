package game

import (
	"errors"
	"fmt"
)

// Placement pose une gemme : orientation O, coin haut gauche de sa boîte en (X, Y).
type Placement struct {
	Kind Kind `json:"kind"`
	O    int  `json:"o"`
	X    int  `json:"x"`
	Y    int  `json:"y"`
}

type Layout []Placement

// LayoutError explique pourquoi une pose est refusée, en français, pour l'interface.
type LayoutError struct{ Msg string }

func (e *LayoutError) Error() string { return e.Msg }

func layoutErr(format string, a ...any) error { return &LayoutError{fmt.Sprintf(format, a...)} }

var ErrIncomplete = errors.New("pose incomplète")

// Build construit la mine d'une pose sans vérifier les règles de pose : chaque
// gemme connue, dans la mine, sans chevauchement.
func (l Layout) Build() (*Board, error) {
	b := newBoard()
	for i, p := range l {
		def, ok := Pieces[p.Kind]
		if !ok {
			return nil, layoutErr("gemme inconnue : %q", p.Kind)
		}
		if p.O < 0 || p.O >= len(def.Orientations) {
			return nil, layoutErr("orientation impossible pour le %s", def.Name)
		}
		o := def.Orientations[p.O]
		if p.X < 0 || p.Y < 0 || p.X+o.W > Width || p.Y+o.H > Height {
			return nil, layoutErr("le %s dépasse de la mine", def.Name)
		}
		for _, c := range o.Cells {
			cell := &b.cells[p.Y+c.Y][p.X+c.X]
			if cell.Piece >= 0 {
				return nil, layoutErr("le %s chevauche le %s", def.Name, Pieces[l[cell.Piece].Kind].Name)
			}
			*cell = Cell{Fill: c.Fill, Color: def.Color, Piece: i}
		}
	}
	return b, nil
}

// Validate vérifie une pose complète du jeu de base :
//   - chaque gemme une fois, entièrement dans la mine, sans chevauchement ;
//   - deux gemmes ne se touchent jamais par un côté (un coin, oui) ;
//   - aucune gemme n'est entièrement cachée par les autres vue des bords.
func (l Layout) Validate() (*Board, error) {
	if len(l) != len(BaseSet) {
		return nil, ErrIncomplete
	}
	seen := map[Kind]bool{}
	for _, p := range l {
		if seen[p.Kind] {
			return nil, layoutErr("le %s est posé deux fois", Pieces[p.Kind].Name)
		}
		seen[p.Kind] = true
	}
	for _, k := range BaseSet {
		if !seen[k] {
			return nil, ErrIncomplete
		}
	}
	b, err := l.Build()
	if err != nil {
		return nil, err
	}
	// Côtés partagés : deux cases voisines dont la matière touche le côté
	// commun sur toute sa longueur. Les diagonales ne peuvent se toucher que
	// dans une même case, déjà exclu.
	for y := range Height {
		for x := range Width {
			c := b.cells[y][x]
			if c.Piece < 0 {
				continue
			}
			for _, n := range [2]struct{ dx, dy, side, opp int }{{1, 0, east, west}, {0, 1, south, north}} {
				o := b.At(x+n.dx, y+n.dy)
				if o.Piece >= 0 && o.Piece != c.Piece && sideFilled(c.Fill, n.side) && sideFilled(o.Fill, n.opp) {
					return nil, layoutErr("le %s touche le %s par un côté", Pieces[l[c.Piece].Kind].Name, Pieces[l[o.Piece].Kind].Name)
				}
			}
		}
	}
	// Visibilité : chaque gemme doit être la première touchée par au moins une
	// ligne droite partie d'un bord.
	visible := make([]bool, len(l))
	for e := range Entries {
		x, y, dx, dy := entryStart(e)
		for {
			x, y = x+dx, y+dy
			if !inside(x, y) {
				break
			}
			if p := b.cells[y][x].Piece; p >= 0 {
				visible[p] = true
				break
			}
		}
	}
	for i, v := range visible {
		if !v {
			return nil, layoutErr("le %s est caché par les autres gemmes vu des bords", Pieces[l[i].Kind].Name)
		}
	}
	return b, nil
}

// EntryLabel donne le nom d'un point d'entrée : 1–18 en haut puis à droite,
// A–R en bas puis à gauche.
func EntryLabel(e int) string {
	if e < Width+Height {
		return fmt.Sprint(e + 1)
	}
	return string(rune('A' + e - Width - Height))
}

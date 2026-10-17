package game

import "math/rand/v2"

// RandomLayout tire une pose valide au hasard (tests, démonstrations).
func RandomLayout(rng *rand.Rand) Layout {
	for {
		l := make(Layout, 0, len(BaseSet))
		for _, k := range BaseSet {
			def := Pieces[k]
			o := rng.IntN(len(def.Orientations))
			or := def.Orientations[o]
			l = append(l, Placement{Kind: k, O: o, X: rng.IntN(Width - or.W + 1), Y: rng.IntN(Height - or.H + 1)})
		}
		if _, err := l.Validate(); err == nil {
			return l
		}
	}
}

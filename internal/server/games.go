package server

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"math/big"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/alexis-marcel/orapa/internal/game"
)

// live est une partie chargée en mémoire : un seul verrou par partie, des
// abonnés WebSocket, et une minuterie pour le drapeau et la fin de la pose.
type live struct {
	mu    sync.Mutex
	rec   *record
	subs  map[*client]struct{}
	timer *time.Timer
}

var (
	errNotFound = errors.New("partie introuvable")
	errNotSeat  = errors.New("tu ne joues pas dans cette partie")
	errSameUser = errors.New("tu es déjà assis dans cette partie")
	errFull     = errors.New("la partie est complète")
)

const idAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"

func newID() string {
	var sb strings.Builder
	for range 8 {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(idAlphabet))))
		sb.WriteByte(idAlphabet[n.Int64()])
	}
	return sb.String()
}

func newToken() (token, hash string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, hashToken(token)
}

func hashToken(t string) string {
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:])
}

func cleanName(name string, u *User) string {
	if u != nil && u.Name != "" {
		name = u.Name
	}
	name = strings.Join(strings.Fields(name), " ")
	if utf8.RuneCountInString(name) > 24 {
		name = string([]rune(name)[:24])
	}
	if name == "" {
		name = "Invité"
	}
	return name
}

func userID(u *User) string {
	if u == nil {
		return ""
	}
	return u.ID
}

func randomSeat() int {
	n, _ := rand.Int(rand.Reader, big.NewInt(2))
	return int(n.Int64())
}

// get renvoie une partie, chargée depuis la base si besoin.
func (s *Server) get(id string) (*live, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if g, ok := s.games[id]; ok {
		return g, nil
	}
	rec, err := s.store.load(id)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, errNotFound
	}
	g := s.adopt(rec)
	return g, nil
}

// adopt met une partie en mémoire (verrou du serveur tenu).
func (s *Server) adopt(rec *record) *live {
	g := &live{rec: rec, subs: map[*client]struct{}{}}
	s.games[rec.ID] = g
	return g
}

func (s *Server) create(cfg game.Config, public bool, name string, u *User) (*record, string, error) {
	tok, hash := newToken()
	now := s.now()
	rec := &record{ID: newID(), Match: game.NewMatch(cfg, randomSeat()), Public: public, Created: now}
	rec.Tokens[0] = hash
	if err := rec.Match.Sit(0, cleanName(name, u), userID(u), now); err != nil {
		return nil, "", err
	}
	if err := s.store.save(rec); err != nil {
		return nil, "", err
	}
	s.mu.Lock()
	s.adopt(rec)
	s.mu.Unlock()
	return rec, tok, nil
}

func (s *Server) join(id, name string, u *User) (string, int, error) {
	g, err := s.get(id)
	if err != nil {
		return "", 0, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	m := g.rec.Match
	if g.rec.Canceled || m.Phase != game.Waiting || m.Players[1] != nil {
		return "", 0, errFull
	}
	if u != nil && m.Players[0].UserID == u.ID {
		return "", 0, errSameUser
	}
	tok, hash := newToken()
	if err := m.Sit(1, cleanName(name, u), userID(u), s.now()); err != nil {
		return "", 0, err
	}
	g.rec.Tokens[1] = hash
	s.commit(g)
	return tok, 1, nil
}

// seatOf retrouve la place d'un client : par son jeton de place, sinon par son compte.
func (g *live) seatOf(token string, u *User) int {
	if token != "" {
		h := hashToken(token)
		for i, t := range g.rec.Tokens {
			if t != "" && subtle.ConstantTimeCompare([]byte(t), []byte(h)) == 1 {
				return i
			}
		}
	}
	if u != nil {
		for i, p := range g.rec.Match.Players {
			if p != nil && p.UserID == u.ID {
				return i
			}
		}
	}
	return -1
}

// commit enregistre la partie, réarme sa minuterie et prévient les abonnés
// (verrou de la partie tenu).
func (s *Server) commit(g *live) {
	if err := s.store.save(g.rec); err != nil {
		s.log.Error("enregistrement", "game", g.rec.ID, "err", err)
	}
	s.arm(g)
	s.broadcast(g)
}

func (s *Server) arm(g *live) {
	if g.timer != nil {
		g.timer.Stop()
		g.timer = nil
	}
	d := g.rec.Match.Deadline()
	if d.IsZero() || g.rec.Canceled {
		return
	}
	g.timer = time.AfterFunc(time.Until(d)+20*time.Millisecond, func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		before := g.rec.Match.Phase
		_ = g.rec.Match.Tick(s.now())
		if g.rec.Match.Phase != before {
			s.commit(g)
		} else {
			s.arm(g)
		}
	})
}

type command struct {
	T      string      `json:"t"`
	Layout game.Layout `json:"layout,omitempty"`
	Entry  int         `json:"entry"`
	X      int         `json:"x"`
	Y      int         `json:"y"`
}

// apply joue une commande d'un joueur.
func (s *Server) apply(g *live, seat int, c command) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if seat < 0 {
		return errNotSeat
	}
	m, now := g.rec.Match, s.now()
	var err error
	switch c.T {
	case "hide":
		err = m.Hide(seat, c.Layout, now)
	case "wave":
		_, err = m.Wave(seat, c.Entry, now)
	case "probe":
		_, err = m.Probe(seat, c.X, c.Y, now)
	case "solve":
		_, err = m.Solve(seat, c.Layout, now)
	case "resign":
		err = m.Resign(seat, now)
	case "cancel":
		if m.Phase != game.Waiting || g.rec.Canceled {
			return game.ErrPhase
		}
		g.rec.Canceled = true
	case "rematch":
		err = s.rematch(g, seat)
	case "draft":
		// Enregistré sans prévenir personne : rien n'en est visible avant la fin.
		// Hors partie ou incohérent, le brouillon est ignoré sans erreur.
		if m.Phase == game.Playing && validDraft(c.Layout) {
			g.rec.Drafts[seat] = c.Layout
			if err := s.store.save(g.rec); err != nil {
				s.log.Error("enregistrement", "game", g.rec.ID, "err", err)
			}
		}
		return nil
	default:
		return errors.New("commande inconnue")
	}
	if errors.Is(err, game.ErrOver) {
		// La partie vient de finir au temps : à enregistrer quand même.
		s.commit(g)
		return err
	}
	if err != nil {
		return err
	}
	s.commit(g)
	return nil
}

// validDraft accepte une pose partielle : au plus les gemmes du jeu de base,
// chacune une fois, dans la mine et sans chevauchement.
func validDraft(l game.Layout) bool {
	if len(l) > len(game.BaseSet) {
		return false
	}
	seen := map[game.Kind]bool{}
	for _, p := range l {
		if seen[p.Kind] {
			return false
		}
		seen[p.Kind] = true
	}
	_, err := l.Build()
	return err == nil
}

// rematch : quand les deux la demandent, une nouvelle partie démarre avec les
// mêmes joueurs (mêmes jetons), l'autre joueur commençant.
func (s *Server) rematch(g *live, seat int) error {
	rec := g.rec
	if rec.Match.Phase != game.Finished || rec.Next != "" {
		return game.ErrPhase
	}
	rec.Rematch[seat] = true
	if !rec.Rematch[0] || !rec.Rematch[1] {
		return nil
	}
	now := s.now()
	next := &record{ID: newID(), Match: game.NewMatch(rec.Match.Config, 1-rec.Match.First), Tokens: rec.Tokens, Created: now}
	for i, p := range rec.Match.Players {
		if err := next.Match.Sit(i, p.Name, p.UserID, now); err != nil {
			return err
		}
	}
	if err := s.store.save(next); err != nil {
		return err
	}
	s.mu.Lock()
	ng := s.adopt(next)
	s.mu.Unlock()
	ng.mu.Lock()
	s.arm(ng)
	ng.mu.Unlock()
	rec.Next = next.ID
	return nil
}

// openGames liste les parties publiques en attente dont le créateur est là.
func (s *Server) openGames() []openGame {
	s.mu.Lock()
	games := make([]*live, 0, len(s.games))
	for _, g := range s.games {
		games = append(games, g)
	}
	s.mu.Unlock()
	out := []openGame{}
	for _, g := range games {
		g.mu.Lock()
		r := g.rec
		if r.Public && !r.Canceled && r.Match.Phase == game.Waiting && g.online(0) {
			out = append(out, openGame{ID: r.ID, Host: r.Match.Players[0].Name, Account: r.Match.Players[0].UserID != "",
				BaseMs: r.Match.Config.Base.Milliseconds(), IncMs: r.Match.Config.Increment.Milliseconds(), Created: r.Created})
		}
		g.mu.Unlock()
	}
	return out
}

// currentGames liste les parties pas encore finies où l'on est assis : par les
// jetons de place gardés dans le navigateur, ou par le compte. Toutes ces
// parties sont en mémoire (reprises au démarrage, retirées seulement une fois finies).
func (s *Server) currentGames(tokens map[string]string, u *User) []currentGame {
	s.mu.Lock()
	games := make([]*live, 0, len(s.games))
	for _, g := range s.games {
		games = append(games, g)
	}
	s.mu.Unlock()
	out := []currentGame{}
	for _, g := range games {
		g.mu.Lock()
		r, m := g.rec, g.rec.Match
		if !r.Canceled && m.Phase != game.Finished {
			if tok, ok := tokens[r.ID]; ok || u != nil {
				if seat := g.seatOf(tok, u); seat >= 0 {
					cg := currentGame{ID: r.ID, Phase: string(m.Phase), Seat: seat, Created: r.Created}
					if o := m.Players[1-seat]; o != nil {
						cg.Opponent = o.Name
					}
					switch m.Phase {
					case game.Setup:
						cg.YourMove = !m.Players[seat].Ready
					case game.Playing:
						cg.YourMove = m.Turn == seat
					}
					out = append(out, cg)
				}
			}
		}
		g.mu.Unlock()
	}
	slices.SortFunc(out, func(a, b currentGame) int { return b.Created.Compare(a.Created) })
	return out
}

type currentGame struct {
	ID       string    `json:"id"`
	Phase    string    `json:"phase"`
	Seat     int       `json:"seat"`
	Opponent string    `json:"opponent,omitempty"` // vide : en attente d'un adversaire
	YourMove bool      `json:"yourMove"`           // à toi de poser ou de jouer
	Created  time.Time `json:"created"`
}

type openGame struct {
	ID      string    `json:"id"`
	Host    string    `json:"host"`
	Account bool      `json:"account"`
	BaseMs  int64     `json:"baseMs"`
	IncMs   int64     `json:"incMs"`
	Created time.Time `json:"created"`
}

func (g *live) online(seat int) bool {
	for c := range g.subs {
		if c.seat == seat {
			return true
		}
	}
	return false
}

// pauseAll marque l'heure d'arrêt de toutes les parties en cours.
func (s *Server) pauseAll() {
	s.mu.Lock()
	games := make([]*live, 0, len(s.games))
	for _, g := range s.games {
		games = append(games, g)
	}
	s.mu.Unlock()
	now := s.now()
	for _, g := range games {
		g.mu.Lock()
		if g.timer != nil {
			g.timer.Stop()
		}
		if p := g.rec.Match.Phase; !g.rec.Canceled && (p == game.Setup || p == game.Playing) {
			g.rec.PausedAt = now
			if err := s.store.save(g.rec); err != nil {
				s.log.Error("pause", "game", g.rec.ID, "err", err)
			}
		}
		g.mu.Unlock()
	}
}

// resume recharge les parties en cours au démarrage. Le temps passé serveur
// arrêté n'est décompté à personne.
func (s *Server) resume() error {
	recs, err := s.store.unfinished()
	if err != nil {
		return err
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, rec := range recs {
		if rec.Canceled {
			continue
		}
		if !rec.PausedAt.IsZero() {
			gap := now.Sub(rec.PausedAt)
			rec.Match.PhaseStart = rec.Match.PhaseStart.Add(gap)
			rec.Match.TurnStart = rec.Match.TurnStart.Add(gap)
			rec.PausedAt = time.Time{}
		}
		g := s.adopt(rec)
		g.mu.Lock()
		s.arm(g)
		g.mu.Unlock()
	}
	s.log.Info("parties reprises", "count", len(recs))
	return nil
}

// sweep retire de la mémoire les parties finies (ou en attente depuis longtemps)
// que plus personne ne regarde.
func (s *Server) sweep() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for id, g := range s.games {
		// Ordre des verrous : partie puis serveur (revanche). Ici l'inverse, donc
		// on passe une partie occupée.
		if !g.mu.TryLock() {
			continue
		}
		r := g.rec
		idle := len(g.subs) == 0
		done := r.Canceled || r.Match.Phase == game.Finished
		stale := r.Match.Phase == game.Waiting && now.Sub(r.Created) > 24*time.Hour
		if idle && (done || stale) {
			if g.timer != nil {
				g.timer.Stop()
			}
			delete(s.games, id)
		}
		g.mu.Unlock()
	}
}

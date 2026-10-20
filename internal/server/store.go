package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/alexis-marcel/orapa/internal/game"
	_ "modernc.org/sqlite"
)

// Une partie est enregistrée en entier (JSON) à chaque coup : c'est petit, et
// une partie en cours survit à un redémarrage.

type record struct {
	ID       string      `json:"id"`
	Match    *game.Match `json:"match"`
	Tokens   [2]string   `json:"tokens"` // SHA-256 des jetons de place
	Public   bool        `json:"public"`
	Rematch  [2]bool     `json:"rematch"`
	Next     string      `json:"next,omitempty"`
	Canceled bool        `json:"canceled,omitempty"`
	Created  time.Time   `json:"created"`
	// Drafts : où chacun en est de la mine d'en face (gemmes posées pendant la
	// recherche). Secret jusqu'à la fin, montré au débrief.
	Drafts [2]game.Layout `json:"drafts,omitzero"`
	// PausedAt est posé à l'arrêt du serveur : au redémarrage, les pendules
	// reprennent là où elles étaient.
	PausedAt time.Time `json:"pausedAt,omitzero"`
}

type Store struct{ db *sql.DB }

func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS games (
	id         TEXT PRIMARY KEY,
	data       TEXT NOT NULL,
	phase      TEXT NOT NULL,
	public     INTEGER NOT NULL DEFAULT 0,
	user0      TEXT,
	user1      TEXT,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS games_phase ON games(phase);
CREATE INDEX IF NOT EXISTS games_user0 ON games(user0, updated_at);
CREATE INDEX IF NOT EXISTS games_user1 ON games(user1, updated_at);`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("schéma : %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func (s *Store) save(rec *record) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	phase := string(rec.Match.Phase)
	if rec.Canceled {
		phase = "canceled"
	}
	var users [2]string
	for i, p := range rec.Match.Players {
		if p != nil {
			users[i] = p.UserID
		}
	}
	_, err = s.db.Exec(`INSERT INTO games (id, data, phase, public, user0, user1, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET data = excluded.data, phase = excluded.phase, public = excluded.public,
			user0 = excluded.user0, user1 = excluded.user1, updated_at = excluded.updated_at`,
		rec.ID, data, phase, rec.Public, nullable(users[0]), nullable(users[1]), rec.Created.UnixMilli(), time.Now().UnixMilli())
	return err
}

func scanRecords(rows *sql.Rows) ([]*record, error) {
	defer rows.Close()
	var out []*record
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		rec := &record{}
		if err := json.Unmarshal(data, rec); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *Store) load(id string) (*record, error) {
	rows, err := s.db.Query(`SELECT data FROM games WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	recs, err := scanRecords(rows)
	if err != nil || len(recs) == 0 {
		return nil, err
	}
	return recs[0], nil
}

// unfinished renvoie les parties à reprendre au démarrage.
func (s *Store) unfinished() ([]*record, error) {
	rows, err := s.db.Query(`SELECT data FROM games WHERE phase IN ('waiting', 'setup', 'playing')`)
	if err != nil {
		return nil, err
	}
	return scanRecords(rows)
}

// history renvoie les dernières parties terminées d'un compte.
func (s *Store) history(userID string, limit int) ([]*record, error) {
	rows, err := s.db.Query(`SELECT data FROM games WHERE phase = 'finished' AND (user0 = ? OR user1 = ?)
		ORDER BY updated_at DESC LIMIT ?`, userID, userID, limit)
	if err != nil {
		return nil, err
	}
	return scanRecords(rows)
}

type stats struct {
	Played int `json:"played"`
	Won    int `json:"won"`
	Lost   int `json:"lost"`
	Drawn  int `json:"drawn"`
}

func (s *Store) stats(userID string) (stats, error) {
	var st stats
	rows, err := s.db.Query(`SELECT
		CASE WHEN user0 = ? THEN 0 ELSE 1 END,
		json_extract(data, '$.match.result.winner')
		FROM games WHERE phase = 'finished' AND (user0 = ? OR user1 = ?)`, userID, userID, userID)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	for rows.Next() {
		var seat int
		var winner sql.NullInt64
		if err := rows.Scan(&seat, &winner); err != nil {
			return st, err
		}
		st.Played++
		switch {
		case !winner.Valid || winner.Int64 < 0:
			st.Drawn++
		case int(winner.Int64) == seat:
			st.Won++
		default:
			st.Lost++
		}
	}
	return st, rows.Err()
}

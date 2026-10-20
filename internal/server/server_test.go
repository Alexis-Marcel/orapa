package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/alexis-marcel/orapa/internal/game"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

var layoutA = game.Layout{
	{Kind: game.WhiteTriangle, O: 0, X: 0, Y: 0},
	{Kind: game.YellowTri, O: 0, X: 7, Y: 1},
	{Kind: game.WhiteDiamond, O: 0, X: 3, Y: 4},
	{Kind: game.RedParallelo, O: 0, X: 0, Y: 5},
	{Kind: game.BlueTriangle, O: 0, X: 5, Y: 6},
}

var layoutB = game.Layout{
	{Kind: game.WhiteTriangle, O: 2, X: 6, Y: 0},
	{Kind: game.BlueTriangle, O: 1, X: 0, Y: 2},
	{Kind: game.YellowTri, O: 3, X: 4, Y: 5},
	{Kind: game.WhiteDiamond, O: 0, X: 7, Y: 5},
	{Kind: game.RedParallelo, O: 2, X: 3, Y: 2},
}

type harness struct {
	t     *testing.T
	srv   *Server
	http  *httptest.Server
	store *Store
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return startHarness(t, store)
}

func startHarness(t *testing.T, store *Store) *harness {
	t.Helper()
	srv, err := New(Config{Store: store, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	// Compte de test : « Authorization: Bearer user:<id> ».
	srv.testUser = func(r *http.Request) *User {
		if b := bearer(r); strings.HasPrefix(b, "user:") {
			id := strings.TrimPrefix(b, "user:")
			return &User{ID: id, Name: "Compte " + id}
		}
		return nil
	}
	h := &harness{t: t, srv: srv, http: httptest.NewServer(srv.Handler()), store: store}
	t.Cleanup(h.http.Close)
	return h
}

func (h *harness) post(path, auth string, body any, out any) int {
	h.t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, h.http.URL+path, bytes.NewReader(b))
	if auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	if out != nil {
		_ = json.NewDecoder(res.Body).Decode(out)
	}
	return res.StatusCode
}

func (h *harness) get(path, auth string, out any) int {
	h.t.Helper()
	req, _ := http.NewRequest(http.MethodGet, h.http.URL+path, nil)
	if auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	if out != nil {
		_ = json.NewDecoder(res.Body).Decode(out)
	}
	return res.StatusCode
}

type wsClient struct {
	t    *testing.T
	conn *websocket.Conn
}

type msg struct {
	T       string    `json:"t"`
	Message string    `json:"message"`
	State   *gameView `json:"state"`
}

func (h *harness) dial(id, query string) *wsClient {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(h.http.URL, "http") + "/ws/games/" + id + "?" + query
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		h.t.Fatal(err)
	}
	h.t.Cleanup(func() { conn.CloseNow() })
	return &wsClient{t: h.t, conn: conn}
}

func (c *wsClient) send(cmd any) {
	c.t.Helper()
	if err := wsjson.Write(context.Background(), c.conn, cmd); err != nil {
		c.t.Fatal(err)
	}
}

// until lit jusqu'à un message qui satisfait ok.
func (c *wsClient) until(ok func(msg) bool) msg {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		var m msg
		if err := wsjson.Read(ctx, c.conn, &m); err != nil {
			c.t.Fatalf("lecture : %v", err)
		}
		if ok(m) {
			return m
		}
	}
}

func phase(p string) func(msg) bool {
	return func(m msg) bool { return m.T == "state" && m.State.Phase == p }
}

func TestFullGame(t *testing.T) {
	h := newHarness(t)

	var cfg map[string]string
	h.get("/api/config", "", &cfg)
	var pieces struct {
		Pieces []game.PieceDef `json:"pieces"`
	}
	h.get("/api/pieces", "", &pieces)
	if len(pieces.Pieces) != 5 || len(pieces.Pieces[4].Orientations) != 4 {
		t.Fatalf("catalogue %+v", pieces)
	}

	var created struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	if st := h.post("/api/games", "user:alice", map[string]any{"name": "ignoré", "cadence": "5+3", "public": true}, &created); st != http.StatusCreated {
		t.Fatalf("création %d", st)
	}
	a := h.dial(created.ID, "token="+created.Token)
	a.until(phase("waiting"))

	var open []openGame
	h.get("/api/games/open", "", &open)
	if len(open) != 1 || open[0].Host != "Compte alice" || open[0].BaseMs != 300_000 {
		t.Fatalf("parties ouvertes %+v", open)
	}
	// Le même compte ne joue pas contre lui-même.
	if st := h.post("/api/games/"+created.ID+"/join", "user:alice", map[string]string{}, nil); st != http.StatusConflict {
		t.Fatalf("auto-partie %d", st)
	}
	var joined struct {
		Token string `json:"token"`
		Seat  int    `json:"seat"`
	}
	if st := h.post("/api/games/"+created.ID+"/join", "", map[string]string{"name": "  Bob   le  chasseur  "}, &joined); st != http.StatusOK || joined.Seat != 1 {
		t.Fatalf("arrivée %d", st)
	}
	if st := h.post("/api/games/"+created.ID+"/join", "", map[string]string{}, nil); st != http.StatusConflict {
		t.Fatalf("troisième joueur %d", st)
	}
	if st := h.post("/api/games/nope/join", "", map[string]string{}, nil); st != http.StatusNotFound {
		t.Fatalf("partie inconnue %d", st)
	}
	// Parties en cours : par le jeton de place, ou par le compte.
	var cur []currentGame
	h.post("/api/games/current", "", map[string]any{"tokens": map[string]string{created.ID: joined.Token, "nope": "x"}}, &cur)
	if len(cur) != 1 || cur[0].Seat != 1 || cur[0].Opponent != "Compte alice" || cur[0].Phase != "setup" || !cur[0].YourMove {
		t.Fatalf("en cours (jeton) %+v", cur)
	}
	h.post("/api/games/current", "user:alice", map[string]any{}, &cur)
	if len(cur) != 1 || cur[0].Seat != 0 || cur[0].Opponent != "Bob le chasseur" {
		t.Fatalf("en cours (compte) %+v", cur)
	}
	h.post("/api/games/current", "", map[string]any{"tokens": map[string]string{created.ID: "faux"}}, &cur)
	if len(cur) != 0 {
		t.Fatalf("en cours (mauvais jeton) %+v", cur)
	}
	b := h.dial(created.ID, "token="+joined.Token)
	sb := b.until(phase("setup")).State
	if sb.You != 1 || sb.Players[1].Name != "Bob le chasseur" || !sb.Players[0].Account {
		t.Fatalf("vue de Bob %+v", sb)
	}
	// Spectateur : par le compte d'Alice sans jeton, on retrouve sa place.
	a2 := h.dial(created.ID, "access_token=user:alice")
	if s := a2.until(phase("setup")).State; s.You != 0 {
		t.Fatalf("place par le compte : %d", s.You)
	}
	spec := h.dial(created.ID, "")
	if s := spec.until(phase("setup")).State; s.You != -1 {
		t.Fatal("spectateur assis")
	}
	spec.send(command{T: "resign"})
	if m := spec.until(func(m msg) bool { return m.T == "error" }); m.Message != errNotSeat.Error() {
		t.Fatalf("spectateur : %q", m.Message)
	}

	// Pose : une pose invalide est expliquée.
	a.send(command{T: "hide", Layout: layoutA[:4]})
	if m := a.until(func(m msg) bool { return m.T == "error" }); m.Message != game.ErrIncomplete.Error() {
		t.Fatalf("pose incomplète : %q", m.Message)
	}
	bad := append(game.Layout{}, layoutA...)
	bad[1] = game.Placement{Kind: game.YellowTri, O: 2, X: 1, Y: 2}
	a.send(command{T: "hide", Layout: bad})
	if m := a.until(func(m msg) bool { return m.T == "error" }); !strings.Contains(m.Message, "par un côté") {
		t.Fatalf("pose invalide : %q", m.Message)
	}
	a.send(command{T: "hide", Layout: layoutA})
	b.send(command{T: "hide", Layout: layoutB})
	s := a.until(phase("playing")).State
	if len(s.MyLayout) != 5 || s.Layouts != nil {
		t.Fatal("poses mal montrées")
	}
	first, second := a, b
	firstLayout, secondTarget := layoutB, layoutA
	if s.Turn == 1 {
		first, second = b, a
		firstLayout, secondTarget = layoutA, layoutB
	}

	// Brouillons : enregistrés en secret, montrés à la fin avec les gemmes trouvées.
	moved := game.Placement{Kind: game.WhiteDiamond, X: 4, Y: 5} // ni chez A (3,4) ni chez B (7,5)
	first.send(command{T: "draft", Layout: game.Layout{firstLayout[0], moved}})
	second.send(command{T: "draft", Layout: game.Layout{secondTarget[0], secondTarget[0]}}) // invalide : ignoré

	// Le premier envoie une onde : il voit la réponse sans le trajet, l'autre voit le trajet.
	first.send(command{T: "wave", Entry: 9})
	fs := first.until(func(m msg) bool { return m.T == "state" && len(m.State.Actions) == 1 }).State
	ss := second.until(func(m msg) bool { return m.T == "state" && len(m.State.Actions) == 1 }).State
	if fs.Actions[0].Path != nil || ss.Actions[0].Path == nil {
		t.Fatal("trajet mal caché")
	}
	if fs.Drafts != nil || ss.Drafts != nil {
		t.Fatal("brouillon visible en cours de partie")
	}
	first.send(command{T: "wave", Entry: 0})
	if m := first.until(func(m msg) bool { return m.T == "error" }); m.Message != game.ErrNotYourTurn.Error() {
		t.Fatalf("hors tour : %q", m.Message)
	}
	// Le second trouve : il gagne.
	second.send(command{T: "solve", Layout: secondTarget})
	end := first.until(phase("finished")).State
	if end.Result == nil || end.Result.Reason != game.BySolve || end.Layouts == nil || end.Actions[0].Path == nil {
		t.Fatalf("fin %+v", end)
	}
	if d := end.Drafts; d == nil || d[1-s.Turn] != nil || d[s.Turn] == nil || !slices.Equal(d[s.Turn].Found, []bool{true, false}) {
		t.Fatalf("débrief %+v", end.Drafts)
	}

	// Revanche : les deux la demandent, une nouvelle partie démarre.
	a.send(command{T: "rematch"})
	b.send(command{T: "rematch"})
	next := a.until(func(m msg) bool { return m.T == "state" && m.State.Next != "" }).State.Next
	a3 := h.dial(next, "token="+created.Token)
	ns := a3.until(phase("setup")).State
	if ns.You != 0 || ns.First == end.First {
		t.Fatalf("revanche %+v", ns)
	}

	// Historique du compte.
	var me struct {
		Stats stats         `json:"stats"`
		Games []historyItem `json:"games"`
	}
	if st := h.get("/api/me", "user:alice", &me); st != http.StatusOK || me.Stats.Played != 1 || len(me.Games) != 1 || me.Games[0].Opponent != "Bob le chasseur" {
		t.Fatalf("historique %d %+v", st, me)
	}
	if st := h.get("/api/me", "", nil); st != http.StatusUnauthorized {
		t.Fatalf("historique invité %d", st)
	}
}

func TestCancelAndResume(t *testing.T) {
	h := newHarness(t)
	var c1, c2 struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	h.post("/api/games", "", map[string]any{"name": "A"}, &c1)
	a := h.dial(c1.ID, "token="+c1.Token)
	a.until(phase("waiting"))
	a.send(command{T: "cancel"})
	a.until(phase("canceled"))
	if st := h.post("/api/games/"+c1.ID+"/join", "", map[string]string{}, nil); st != http.StatusConflict {
		t.Fatalf("partie annulée rejointe %d", st)
	}

	// Une partie en cours survit à un redémarrage, pendules en pause.
	h.post("/api/games", "", map[string]any{"name": "A"}, &c2)
	var j struct {
		Token string `json:"token"`
	}
	h.post("/api/games/"+c2.ID+"/join", "", map[string]string{"name": "B"}, &j)
	g, _ := h.srv.get(c2.ID)
	if err := h.srv.apply(g, 0, command{T: "hide", Layout: layoutA}); err != nil {
		t.Fatal(err)
	}
	if err := h.srv.apply(g, 1, command{T: "hide", Layout: layoutB}); err != nil {
		t.Fatal(err)
	}
	h.srv.pauseAll()

	h2 := startHarness(t, h.store)
	h2.srv.now = func() time.Time { return time.Now().Add(time.Hour) }
	if err := h2.srv.resume(); err != nil {
		t.Fatal(err)
	}
	b := h2.dial(c2.ID, "token="+j.Token)
	s := b.until(phase("playing")).State
	if s.Players[0].RemainingMs < 590_000 {
		t.Fatalf("pendule après redémarrage : %d", s.Players[0].RemainingMs)
	}
}

func TestStaticAndHealth(t *testing.T) {
	dir := t.TempDir()
	_ = writeFile(filepath.Join(dir, "index.html"), "<html>orapa</html>")
	store, _ := OpenStore(filepath.Join(t.TempDir(), "t.db"))
	defer store.Close()
	srv, _ := New(Config{Store: store, StaticDir: dir, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	for path, want := range map[string]int{"/": 200, "/g/abc": 200, "/healthz": 200, "/api/nope": 404, "/api/games/nope": 404} {
		res, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != want {
			t.Errorf("%s : %d", path, res.StatusCode)
		}
	}
}

func writeFile(path, content string) error { return os.WriteFile(path, []byte(content), 0o644) }

// Package ui serves the app's window: a small page on 127.0.0.1 that the exe opens in the default browser.
// There is no extra runtime to install. Every request needs the random per-run token and a loopback Host,
// so other web pages (or DNS rebinding) cannot drive the updater.
package ui

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/auzring1/librepatchdesktop/internal/updater"
)

//go:embed index.html
var page string

// Backend does the real work (scan, catalog lookup, install).
type Backend interface {
	Check(ctx context.Context) ([]updater.Item, error)
	Install(ctx context.Context, it updater.Item) error
}

// Server is the window's HTTP handler.
type Server struct {
	b     Backend
	Token string
	// Quit, if set, is called when the page's Quit button is pressed.
	Quit func()

	mu      sync.Mutex
	items   []updater.Item
	state   map[string]string // key -> "queued" | "installing" | "done" | "failed: reason"
	busy    bool
	loadErr string
	loading bool
}

func New(b Backend) *Server {
	t := make([]byte, 16)
	rand.Read(t)
	return &Server{b: b, Token: hex.EncodeToString(t), state: map[string]string{}}
}

func key(it updater.Item) string { return it.App.Scope + "|" + it.App.UninstallKey + "|" + it.App.Name }

// Row is one line of the table.
type Row struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	Installed string `json:"installed"`
	Latest    string `json:"latest,omitempty"`
	Status    string `json:"status"`
	State     string `json:"state,omitempty"`
}

type apps struct {
	Rows    []Row  `json:"rows"`
	Busy    bool   `json:"busy"`
	Loading bool   `json:"loading"`
	Error   string `json:"error,omitempty"`
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'self'")
		w.Write([]byte(strings.Replace(page, "__TOKEN__", s.Token, 1)))
	})
	mux.HandleFunc("GET /api/apps", s.handleApps)
	mux.HandleFunc("POST /api/refresh", s.handleRefresh)
	mux.HandleFunc("POST /api/update", s.handleUpdate)
	mux.HandleFunc("POST /api/quit", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]bool{"ok": true})
		if s.Quit != nil {
			go s.Quit()
		}
	})
	return s.guard(mux)
}

// guard enforces a loopback Host and the token on every API call.
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil || (host != "127.0.0.1" && host != "localhost") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") && r.Header.Get("X-Token") != s.Token {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleApps(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	out := apps{Busy: s.busy, Loading: s.loading, Error: s.loadErr, Rows: []Row{}}
	for _, it := range s.items {
		if it.Status == updater.Unknown {
			continue
		}
		r := Row{Key: key(it), Name: it.App.Name, Installed: it.App.Version, Status: string(it.Status), State: s.state[key(it)]}
		if it.Entry != nil {
			r.Latest = it.Entry.Version
		}
		out.Rows = append(out.Rows, r)
	}
	s.mu.Unlock()
	writeJSON(w, out)
}

// Refresh rescans in the background.
func (s *Server) Refresh() {
	s.mu.Lock()
	if s.loading || s.busy {
		s.mu.Unlock()
		return
	}
	s.loading, s.loadErr = true, ""
	s.mu.Unlock()
	go func() {
		items, err := s.b.Check(context.Background())
		s.mu.Lock()
		defer s.mu.Unlock()
		s.loading = false
		if err != nil {
			s.loadErr = err.Error()
			return
		}
		s.items, s.state = items, map[string]string{}
	}()
}

func (s *Server) handleRefresh(w http.ResponseWriter, _ *http.Request) {
	s.Refresh()
	writeJSON(w, map[string]bool{"ok": true})
}

// handleUpdate installs the listed keys one after another; an empty list means every outdated app.
func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Keys []string `json:"keys"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	want := map[string]bool{}
	for _, k := range req.Keys {
		want[k] = true
	}
	s.mu.Lock()
	if s.busy || s.loading {
		s.mu.Unlock()
		http.Error(w, "busy", http.StatusConflict)
		return
	}
	var todo []updater.Item
	for _, it := range s.items {
		if it.Status == updater.Outdated && (len(want) == 0 || want[key(it)]) {
			todo = append(todo, it)
			s.state[key(it)] = "queued"
		}
	}
	if len(todo) > 0 {
		s.busy = true
	}
	s.mu.Unlock()
	go func() {
		for _, it := range todo {
			s.set(key(it), "installing")
			if err := s.b.Install(context.Background(), it); err != nil {
				s.set(key(it), "failed: "+err.Error())
			} else {
				s.set(key(it), "done")
			}
		}
		s.mu.Lock()
		s.busy = false
		s.mu.Unlock()
	}()
	writeJSON(w, map[string]int{"queued": len(todo)})
}

func (s *Server) set(k, v string) {
	s.mu.Lock()
	s.state[k] = v
	s.mu.Unlock()
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(v)
}

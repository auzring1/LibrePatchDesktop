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
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/auzring1/librepatchdesktop/internal/inventory"
	"github.com/auzring1/librepatchdesktop/internal/schedule"
	"github.com/auzring1/librepatchdesktop/internal/settings"
	"github.com/auzring1/librepatchdesktop/internal/sysinfo"
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
	Quit     func()
	Settings *settings.Store
	// SetStartup creates or removes the "start with Windows" entry (nil where unsupported).
	SetStartup func(enabled bool) error
	Log        *log.Logger // nil discards
	LogDir     string
	Info       sysinfo.Info
	Version    string

	mu      sync.Mutex
	items   []updater.Item
	state   map[string]string // key -> "queued" | "installing" | "done" | "failed: reason"
	busy    bool
	loadErr string
	loading bool
	batch   int // apps queued in the current update run
	done    int // of which finished
}

func New(b Backend) *Server {
	t := make([]byte, 16)
	rand.Read(t)
	return &Server{b: b, Token: hex.EncodeToString(t), state: map[string]string{}, Settings: settings.Open(""), Info: sysinfo.Get(), Version: "dev"}
}

func key(it updater.Item) string { return it.App.Scope + "|" + it.App.UninstallKey + "|" + it.App.Name }

// Row is one application in the window.
type Row struct {
	Key       string `json:"key"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	FullName  string `json:"fullName"`
	Installed string `json:"installed"`
	Latest    string `json:"latest,omitempty"`
	Status    string `json:"status"` // outdated, current, older, skipped
	State     string `json:"state,omitempty"`
}

type apps struct {
	Rows    []Row  `json:"rows"`
	Busy    bool   `json:"busy"`
	Loading bool   `json:"loading"`
	Error   string `json:"error,omitempty"`
	Batch   int    `json:"batch"`
	Done    int    `json:"done"`
}

// skipKey identifies an app for the skip list: the winget id when known, else the program name.
func skipKey(it updater.Item) string {
	if it.Entry != nil && it.Entry.ID != "" {
		return it.Entry.ID
	}
	return it.App.Name
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
	mux.HandleFunc("GET /api/info", func(w http.ResponseWriter, _ *http.Request) {
		next := ""
		if d := s.Settings.Get(); d.Schedule.Enabled {
			if t, ok := d.Schedule.Next(time.Now()); ok {
				next = t.Format(time.RFC3339)
			}
		}
		writeJSON(w, map[string]any{"info": s.Info, "version": s.Version, "settings": s.Settings.Get(), "nextRun": next, "canStartup": s.SetStartup != nil})
	})
	mux.HandleFunc("POST /api/settings", s.handleSettings)
	mux.HandleFunc("POST /api/skip", s.handleSkip)
	mux.HandleFunc("POST /api/startup", s.handleStartup)
	s.routesMore(mux)
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
	out := apps{Busy: s.busy, Loading: s.loading, Error: s.loadErr, Batch: s.batch, Done: s.done, Rows: []Row{}}
	for _, it := range s.items {
		if it.Status == updater.Unknown {
			continue
		}
		r := Row{Key: key(it), ID: skipKey(it), Name: inventory.FriendlyName(it.App.Name, it.App.Version), FullName: it.App.Name,
			Installed: it.App.Version, State: s.state[key(it)]}
		if it.Entry != nil {
			r.Latest = it.Entry.Version
		}
		switch it.Status {
		case updater.Outdated:
			r.Status = "outdated"
			if s.Settings.IsSkipped(skipKey(it)) {
				r.Status = "skipped"
			}
		case updater.Superseded:
			r.Status = "older"
		default:
			r.Status = "current"
		}
		out.Rows = append(out.Rows, r)
	}
	s.mu.Unlock()
	// Outdated first, then the rest, each by name.
	rank := map[string]int{"outdated": 0, "skipped": 1, "older": 2, "current": 3}
	sort.SliceStable(out.Rows, func(i, j int) bool {
		a, b := out.Rows[i], out.Rows[j]
		if rank[a.Status] != rank[b.Status] {
			return rank[a.Status] < rank[b.Status]
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	writeJSON(w, out)
}

// handleSettings changes the saved choices. Only the fields present in the request change.
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Theme         *string            `json:"theme"`
		View          *string            `json:"view"`
		CloseToTray   *bool              `json:"closeToTray"`
		AskedSchedule *bool              `json:"askedSchedule"`
		Schedule      *schedule.Schedule `json:"schedule"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.Schedule != nil {
		if err := req.Schedule.Valid(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	err := s.Settings.Update(func(d *settings.Data) {
		if req.Theme != nil {
			d.Theme = *req.Theme
		}
		if req.View != nil {
			d.View = *req.View
		}
		if req.CloseToTray != nil {
			d.CloseToTray = *req.CloseToTray
		}
		if req.AskedSchedule != nil {
			d.AskedSchedule = *req.AskedSchedule
		}
		if req.Schedule != nil {
			d.Schedule = *req.Schedule
		}
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, s.Settings.Get())
}

// handleStartup turns "start with Windows" on or off through the injected SetStartup.
func (s *Server) handleStartup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if s.SetStartup == nil {
		http.Error(w, "not available here", http.StatusNotImplemented)
		return
	}
	if err := s.SetStartup(req.Enabled); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.Settings.Update(func(d *settings.Data) { d.StartWithWindows = req.Enabled })
	writeJSON(w, s.Settings.Get())
}

// handleSkip adds or removes an app (by row key) from the skip list.
func (s *Server) handleSkip(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key  string `json:"key"`
		Skip bool   `json:"skip"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	id := ""
	for _, it := range s.items {
		if key(it) == req.Key {
			id = skipKey(it)
		}
	}
	s.mu.Unlock()
	if id == "" {
		http.Error(w, "unknown app", http.StatusNotFound)
		return
	}
	if err := s.Settings.SetSkipped(id, req.Skip); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
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

// handleUpdate installs the listed keys one after another; an empty list means every outdated app that is not skipped.
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
	n, err := s.startUpdate(want, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSON(w, map[string]int{"queued": n})
}

// startUpdate queues the wanted apps (all non-skipped outdated ones when want is empty) and installs them in the
// background. auto marks a scheduled run, whose result is written to the settings for the Scheduler page.
func (s *Server) startUpdate(want map[string]bool, auto bool) (int, error) {
	s.mu.Lock()
	if s.busy || s.loading {
		s.mu.Unlock()
		return 0, errors.New("busy")
	}
	var todo []updater.Item
	for _, it := range s.items {
		if it.Status == updater.Outdated && ((len(want) == 0 && !s.Settings.IsSkipped(skipKey(it))) || want[key(it)]) {
			todo = append(todo, it)
			s.state[key(it)] = "queued"
		}
	}
	if len(todo) > 0 {
		s.busy, s.batch, s.done = true, len(todo), 0
	}
	s.mu.Unlock()
	if len(todo) == 0 {
		return 0, nil
	}
	go func() {
		failed := 0
		for _, it := range todo {
			s.set(key(it), "installing")
			if err := s.b.Install(context.Background(), it); err != nil {
				s.set(key(it), "failed: "+err.Error())
				s.logf("update %s failed: %v", it.App.Name, err)
				failed++
			} else {
				s.set(key(it), "done")
				s.logf("updated %s to %s", it.App.Name, it.Entry.Version)
			}
			s.mu.Lock()
			s.done++
			s.mu.Unlock()
		}
		s.mu.Lock()
		s.busy = false
		s.mu.Unlock()
		if auto {
			s.setResult(fmt.Sprintf("Updated %d of %d apps", len(todo)-failed, len(todo)))
			s.logf("scheduled run: updated %d of %d apps", len(todo)-failed, len(todo))
		}
	}()
	return len(todo), nil
}

func (s *Server) setResult(msg string) {
	s.Settings.Update(func(d *settings.Data) { d.LastResult = msg })
}

// RunScheduler checks every 30 seconds whether the scheduled update is due, until ctx ends.
func (s *Server) RunScheduler(ctx context.Context) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	s.tick(time.Now())
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			s.tick(now)
		}
	}
}

// tick starts the scheduled run if it is due and nothing else is running.
func (s *Server) tick(now time.Time) {
	d := s.Settings.Get()
	if !d.Schedule.Due(now, d.LastRun) {
		return
	}
	s.mu.Lock()
	idle := !s.busy && !s.loading
	if idle {
		s.loading, s.loadErr = true, ""
	}
	s.mu.Unlock()
	if !idle {
		return // try again at the next tick
	}
	s.Settings.Update(func(d *settings.Data) { d.LastRun, d.LastResult = now, "Running…" })
	go func() {
		items, err := s.b.Check(context.Background())
		s.mu.Lock()
		s.loading = false
		if err == nil {
			s.items, s.state = items, map[string]string{}
		} else {
			s.loadErr = err.Error()
		}
		s.mu.Unlock()
		if err != nil {
			s.setResult("Could not check for updates: " + err.Error())
			return
		}
		n, err := s.startUpdate(map[string]bool{}, true)
		switch {
		case err != nil:
			s.setResult("Could not start: " + err.Error())
		case n == 0:
			s.setResult("Everything was up to date")
		}
	}()
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

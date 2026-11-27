package ui

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/auzring1/librepatchdesktop/internal/inventory"
	"github.com/auzring1/librepatchdesktop/internal/selfupdate"
	"github.com/auzring1/librepatchdesktop/internal/startup"
	"github.com/auzring1/librepatchdesktop/internal/sysdir"
	"github.com/auzring1/librepatchdesktop/internal/updater"
)

// Optional abilities of a Backend. A page whose ability is missing answers "not available".

// LibraryItem is an app that can be installed from the library.
type LibraryItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Installed bool   `json:"installed"`
	State     string `json:"state,omitempty"`
}

// Library searches the app catalog and installs a new app.
type Library interface {
	Library(q string, limit int) []LibraryItem
	InstallPackage(ctx context.Context, id string) error
}

// Uninstaller removes an installed program.
type Uninstaller interface {
	Uninstall(ctx context.Context, app inventory.App) error
}

// Startups lists and switches Windows startup entries.
type Startups interface {
	Startups() ([]startup.Entry, error)
	SetStartupEnabled(id string, on bool) error
}

// Releaser looks for and installs a newer LibrePatchDesktop.
type Releaser interface {
	LatestRelease(ctx context.Context) (selfupdate.Release, error)
	InstallRelease(ctx context.Context, rel selfupdate.Release) error
}

func (s *Server) logf(format string, a ...any) {
	if s.Log != nil {
		s.Log.Printf(format, a...)
	}
}

func (s *Server) routesMore(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/library", s.handleLibrary)
	mux.HandleFunc("POST /api/library/install", s.handleLibraryInstall)
	mux.HandleFunc("GET /api/uninstall", s.handleUninstallList)
	mux.HandleFunc("POST /api/uninstall", s.handleUninstall)
	mux.HandleFunc("GET /api/startups", s.handleStartups)
	mux.HandleFunc("POST /api/startups", s.handleStartupsSet)
	mux.HandleFunc("GET /api/release", s.handleRelease)
	mux.HandleFunc("POST /api/release/install", s.handleReleaseInstall)
	mux.HandleFunc("GET /api/diagnostics", s.handleDiagnostics)
	mux.HandleFunc("POST /api/openlogs", s.handleOpenLogs)
}

func notAvailable(w http.ResponseWriter) {
	http.Error(w, "not available here", http.StatusNotImplemented)
}

// begin marks the app busy for one background job; false if something else is running.
func (s *Server) begin() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy || s.loading {
		return false
	}
	s.busy, s.batch, s.done = true, 1, 0
	return true
}

func (s *Server) end() {
	s.mu.Lock()
	s.busy, s.done = false, 1
	s.mu.Unlock()
}

// ---- App Library ----

func (s *Server) handleLibrary(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.b.(Library)
	if !ok {
		notAvailable(w)
		return
	}
	items := lib.Library(strings.TrimSpace(r.URL.Query().Get("q")), 60)
	s.mu.Lock()
	have := map[string]bool{}
	for _, it := range s.items {
		if it.Entry != nil {
			have[it.Entry.ID] = true
		}
	}
	for i := range items {
		items[i].Installed = have[items[i].ID]
		items[i].State = s.state["lib|"+items[i].ID]
	}
	s.mu.Unlock()
	writeJSON(w, map[string]any{"items": items, "busy": s.busyNow()})
}

func (s *Server) busyNow() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.busy || s.loading
}

func (s *Server) handleLibraryInstall(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.b.(Library)
	if !ok {
		notAvailable(w)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !s.begin() {
		http.Error(w, "busy", http.StatusConflict)
		return
	}
	k := "lib|" + req.ID
	s.set(k, "installing")
	go func() {
		err := lib.InstallPackage(context.Background(), req.ID)
		if err != nil {
			s.set(k, "failed: "+err.Error())
			s.logf("install %s failed: %v", req.ID, err)
		} else {
			s.set(k, "done")
			s.logf("installed %s", req.ID)
		}
		s.end()
		if err == nil {
			s.Refresh()
		}
	}()
	writeJSON(w, map[string]bool{"ok": true})
}

// ---- Uninstaller ----

type uninstallRow struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Publisher string `json:"publisher"`
	Scope     string `json:"scope"`
	State     string `json:"state,omitempty"`
}

func (s *Server) handleUninstallList(w http.ResponseWriter, _ *http.Request) {
	if _, ok := s.b.(Uninstaller); !ok {
		notAvailable(w)
		return
	}
	s.mu.Lock()
	rows := []uninstallRow{}
	for _, it := range s.items {
		a := it.App
		rows = append(rows, uninstallRow{Key: key(it), Name: a.Name, Version: a.Version, Publisher: a.Publisher, Scope: a.Scope, State: s.state["un|"+key(it)]})
	}
	s.mu.Unlock()
	sort.Slice(rows, func(i, j int) bool { return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name) })
	writeJSON(w, map[string]any{"rows": rows, "busy": s.busyNow(), "loading": s.isLoading()})
}

func (s *Server) isLoading() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loading
}

func (s *Server) handleUninstall(w http.ResponseWriter, r *http.Request) {
	un, ok := s.b.(Uninstaller)
	if !ok {
		notAvailable(w)
		return
	}
	var req struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var item *updater.Item
	s.mu.Lock()
	for i := range s.items {
		if key(s.items[i]) == req.Key {
			item = &s.items[i]
		}
	}
	s.mu.Unlock()
	if item == nil {
		http.Error(w, "unknown app", http.StatusNotFound)
		return
	}
	if !s.begin() {
		http.Error(w, "busy", http.StatusConflict)
		return
	}
	app := item.App
	k := "un|" + req.Key
	s.set(k, "removing")
	go func() {
		err := un.Uninstall(context.Background(), app)
		if err != nil {
			s.set(k, "failed: "+err.Error())
			s.logf("uninstall %s failed: %v", app.Name, err)
		} else {
			s.set(k, "done")
			s.logf("uninstalled %s", app.Name)
		}
		s.end()
		if err == nil {
			s.Refresh()
		}
	}()
	writeJSON(w, map[string]bool{"ok": true})
}

// ---- Startup manager ----

func (s *Server) handleStartups(w http.ResponseWriter, _ *http.Request) {
	st, ok := s.b.(Startups)
	if !ok {
		notAvailable(w)
		return
	}
	list, err := st.Startups()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []startup.Entry{}
	}
	writeJSON(w, map[string]any{"entries": list})
}

func (s *Server) handleStartupsSet(w http.ResponseWriter, r *http.Request) {
	st, ok := s.b.(Startups)
	if !ok {
		notAvailable(w)
		return
	}
	var req struct {
		ID string `json:"id"`
		On bool   `json:"on"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := st.SetStartupEnabled(req.ID, req.On); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.logf("startup entry %s set to %v", req.ID, req.On)
	writeJSON(w, map[string]bool{"ok": true})
}

// ---- About: updates, logs, diagnostics ----

func (s *Server) handleRelease(w http.ResponseWriter, r *http.Request) {
	rl, ok := s.b.(Releaser)
	if !ok {
		notAvailable(w)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	rel, err := rl.LatestRelease(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{"current": s.Version, "latest": rel.Version, "pageUrl": rel.PageURL, "newer": selfupdate.Newer(rel, s.Version)})
}

func (s *Server) handleReleaseInstall(w http.ResponseWriter, r *http.Request) {
	rl, ok := s.b.(Releaser)
	if !ok {
		notAvailable(w)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
	defer cancel()
	rel, err := rl.LatestRelease(ctx)
	if err != nil || !selfupdate.Newer(rel, s.Version) {
		http.Error(w, "no newer version", http.StatusConflict)
		return
	}
	s.logf("installing LibrePatchDesktop %s", rel.Version)
	if err := rl.InstallRelease(ctx, rel); err != nil {
		s.logf("self-update failed: %v", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]bool{"ok": true}) // the installer now closes this app and starts the new one
}

// handleDiagnostics sends a zip with the log, the settings and basic system information.
func (s *Server) handleDiagnostics(w http.ResponseWriter, _ *http.Request) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name string, data []byte) {
		if f, err := zw.Create(name); err == nil {
			f.Write(data)
		}
	}
	info := fmt.Sprintf("LibrePatchDesktop %s\nWindows: %s\nComputer: %s\nTime: %s\n", s.Version, s.Info.OS, s.Info.Hostname, time.Now().Format(time.RFC3339))
	add("info.txt", []byte(info))
	st, _ := json.MarshalIndent(s.Settings.Get(), "", "  ")
	add("settings.json", st)
	if s.LogDir != "" {
		for _, n := range []string{"app.log", "app.log.old"} {
			if b, err := os.ReadFile(filepath.Join(s.LogDir, n)); err == nil {
				add(n, b)
			}
		}
	}
	zw.Close()
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="LibrePatchDesktop-diagnostics.zip"`)
	io.Copy(w, &buf)
}

func (s *Server) handleOpenLogs(w http.ResponseWriter, _ *http.Request) {
	if s.LogDir == "" {
		notAvailable(w)
		return
	}
	os.MkdirAll(s.LogDir, 0o700)
	if err := exec.Command(sysdir.Windows("explorer.exe"), s.LogDir).Start(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"dir": s.LogDir})
}

// OpenLog opens (and rotates when big) the log file in dir and returns a logger for it.
func OpenLog(dir string) (*log.Logger, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	p := filepath.Join(dir, "app.log")
	if st, err := os.Stat(p); err == nil && st.Size() > 1<<20 {
		os.Rename(p, p+".old")
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	return log.New(f, "", log.LstdFlags), nil
}

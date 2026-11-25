package ui

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/auzring1/librepatchdesktop/internal/catalog"
	"github.com/auzring1/librepatchdesktop/internal/inventory"
	"github.com/auzring1/librepatchdesktop/internal/updater"
)

type fake struct{ installed []string }

func (f *fake) Check(context.Context) ([]updater.Item, error) {
	e := catalog.Entry{ID: "x", Version: "2"}
	return []updater.Item{
		{App: inventory.App{Name: "Old", Version: "1", Scope: "machine"}, Entry: &e, Status: updater.Outdated},
		{App: inventory.App{Name: "Fine", Version: "2", Scope: "machine"}, Entry: &e, Status: updater.UpToDate},
		{App: inventory.App{Name: "Unknown", Version: "1"}, Status: updater.Unknown},
	}, nil
}
func (f *fake) Install(_ context.Context, it updater.Item) error {
	f.installed = append(f.installed, it.App.Name)
	if it.App.Name == "Bad" {
		return errors.New("boom")
	}
	return nil
}

func do(h http.Handler, method, path, host, token, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Host = host
	if token != "" {
		r.Header.Set("X-Token", token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestGuard(t *testing.T) {
	s := New(&fake{})
	h := s.Handler()
	if w := do(h, "GET", "/api/apps", "127.0.0.1:1", "", ""); w.Code != 403 {
		t.Fatalf("no token: %d", w.Code)
	}
	if w := do(h, "GET", "/api/apps", "evil.example:80", s.Token, ""); w.Code != 403 {
		t.Fatalf("bad host: %d", w.Code)
	}
	if w := do(h, "GET", "/", "127.0.0.1:1", "", ""); w.Code != 200 || !strings.Contains(w.Body.String(), s.Token) {
		t.Fatalf("page: %d", w.Code)
	}
}

func TestUpdateAll(t *testing.T) {
	f := &fake{}
	s := New(f)
	h := s.Handler()
	do(h, "POST", "/api/refresh", "127.0.0.1:1", s.Token, "")
	wait(t, func() bool {
		return strings.Contains(do(h, "GET", "/api/apps", "localhost:1", s.Token, "").Body.String(), `"Old"`)
	})
	if w := do(h, "POST", "/api/update", "127.0.0.1:1", s.Token, `{"keys":[]}`); w.Code != 200 || !strings.Contains(w.Body.String(), `"queued":1`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	wait(t, func() bool {
		return strings.Contains(do(h, "GET", "/api/apps", "localhost:1", s.Token, "").Body.String(), `"state":"done"`)
	})
	if len(f.installed) != 1 || f.installed[0] != "Old" {
		t.Fatalf("installed %v", f.installed)
	}
	if body := do(h, "GET", "/api/apps", "localhost:1", s.Token, "").Body.String(); strings.Contains(body, "Unknown") {
		t.Fatal("apps without a catalog match are not listed")
	}
}

func wait(t *testing.T, cond func() bool) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out")
}

package selfupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLatestPicksHighestWithInstaller(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[
		 {"tag_name":"v0.3.0","html_url":"p3","assets":[{"name":"LibrePatchDesktop-Setup.exe","browser_download_url":"s3"},{"name":"SHA256SUMS.txt","browser_download_url":"h3"}]},
		 {"tag_name":"v0.10.0","html_url":"p10","assets":[{"name":"LibrePatchDesktop-Setup.exe","browser_download_url":"s10"},{"name":"SHA256SUMS.txt","browser_download_url":"h10"}]},
		 {"tag_name":"v0.11.0","html_url":"p11","assets":[]},
		 {"tag_name":"v9.0.0","draft":true,"html_url":"pd","assets":[{"name":"LibrePatchDesktop-Setup.exe","browser_download_url":"sd"},{"name":"SHA256SUMS.txt","browser_download_url":"hd"}]}
		]`))
	}))
	defer srv.Close()
	rel, err := Latest(context.Background(), srv.Client(), srv.URL)
	if err != nil || rel.Version != "0.10.0" || rel.SetupURL != "s10" {
		t.Fatalf("%+v %v", rel, err)
	}
	if !Newer(rel, "0.3.0") || Newer(rel, "0.10.0") || Newer(rel, "dev") {
		t.Fatal("Newer")
	}
}

func TestInstallChecksHash(t *testing.T) {
	setup := []byte("installer bytes")
	sum := sha256.Sum256(setup)
	good := hex.EncodeToString(sum[:]) + "  LibrePatchDesktop-Setup.exe\n"
	serve := func(sums string) (*httptest.Server, Release) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/sums" {
				w.Write([]byte(sums))
			} else {
				w.Write(setup)
			}
		}))
		return srv, Release{SetupURL: assetPrefix + "x", SumsURL: assetPrefix + "y"}
	}
	srv, rel := serve(good)
	defer srv.Close()
	c := srv.Client()
	c.Transport = rewrite{srv.URL, c.Transport}
	ran := ""
	if err := Install(context.Background(), c, rel, func(p string) error { ran = p; return nil }); err != nil || ran == "" {
		t.Fatalf("good: %v ran=%q", err, ran)
	}
	srv2, rel2 := serve("0000000000000000000000000000000000000000000000000000000000000000  LibrePatchDesktop-Setup.exe\n")
	defer srv2.Close()
	c2 := srv2.Client()
	c2.Transport = rewrite{srv2.URL, c2.Transport}
	ran = ""
	if err := Install(context.Background(), c2, rel2, func(p string) error { ran = p; return nil }); err == nil || ran != "" {
		t.Fatalf("bad hash must fail and not run: %v %q", err, ran)
	}
	if err := Install(context.Background(), c, Release{SetupURL: "https://evil.example/a", SumsURL: "https://evil.example/b"}, nil); err == nil {
		t.Fatal("unexpected host must be refused")
	}
}

// rewrite sends every request to the test server, keeping the path (the real release host is not reachable in tests).
type rewrite struct {
	base string
	next http.RoundTripper
}

func (r rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Host = r.base[len("https://"):]
	if req.URL.Path != "" && len(req.URL.Path) > 0 {
		if req.URL.Path[len(req.URL.Path)-1] == 'y' {
			req2.URL.Path = "/sums"
		} else {
			req2.URL.Path = "/setup"
		}
	}
	return r.next.RoundTrip(req2)
}

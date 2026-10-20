package server

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// upstream records the last request it received and answers 204.
type upstream struct {
	*httptest.Server
	last *http.Request
}

func newUpstream(t *testing.T) *upstream {
	u := &upstream{}
	u.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u.last = r
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(u.Close)
	return u
}

func newTestSite() fs.FS {
	return fstest.MapFS{
		"index.html":       {Data: []byte("<p>https://session." + CanonicalHost + "</p>")},
		"zh-cn/index.html": {Data: []byte("<p>zh</p>")},
		"favicon.svg":      {Data: []byte("<svg>" + CanonicalHost + "</svg>")},
		".gitkeep":         {Data: []byte("")},
	}
}

// newTestApp enables every module unless cfg.Modules is set (pass an empty
// map to disable them all).
func newTestApp(t *testing.T, cfg Config, up *upstream) *Server {
	if cfg.Modules == nil {
		cfg.Modules = map[Module]bool{}
		for _, m := range Modules {
			cfg.Modules[m] = true
		}
	}
	if up != nil {
		cfg.Upstreams = map[Service]string{}
		for _, s := range Services {
			cfg.Upstreams[s] = up.URL
		}
	}
	if cfg.Site == nil {
		cfg.Site = newTestSite()
	}
	app, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func do(app http.Handler, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, nil)
	for k, v := range headers {
		if k == "Host" {
			r.Host = v
		} else {
			r.Header.Set(k, v)
		}
	}
	w := httptest.NewRecorder()
	app.ServeHTTP(w, r)
	return w
}

func TestProxyForwardsPathAndStripsClientHeaders(t *testing.T) {
	up := newUpstream(t)
	app := newTestApp(t, Config{}, up)

	w := do(app, http.MethodGet, "/session/minecraft/hasJoined?username=a%20b&serverId=x", map[string]string{
		"Host":             "session.yggdrasil.localhost:8787",
		"CF-Connecting-IP": "1.2.3.4",
		"X-Forwarded-For":  "1.2.3.4",
		"X-Real-IP":        "1.2.3.4",
		"Forwarded":        "for=1.2.3.4",
		"Authorization":    "Bearer abc",
	})
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}
	if got := up.last.URL.RequestURI(); got != "/session/minecraft/hasJoined?username=a%20b&serverId=x" {
		t.Errorf("upstream URI = %q", got)
	}
	for _, h := range []string{"CF-Connecting-IP", "X-Forwarded-For", "X-Real-IP", "Forwarded"} {
		if v := up.last.Header.Get(h); v != "" {
			t.Errorf("%s leaked upstream: %q", h, v)
		}
	}
	if got := up.last.Header.Get("Authorization"); got != "Bearer abc" {
		t.Errorf("Authorization = %q, want passthrough", got)
	}
}

func TestProxyToken(t *testing.T) {
	up := newUpstream(t)
	app := newTestApp(t, Config{ProxyToken: "s3cr et"}, up)
	host := map[string]string{"Host": "services.yggdrasil.example.com"}

	w := do(app, http.MethodGet, "/s3cr%20et/publickeys%2Fx?a=1", host)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}
	if got := up.last.URL.RequestURI(); got != "/publickeys%2Fx?a=1" {
		t.Errorf("upstream URI = %q", got)
	}

	for _, target := range []string{"/publickeys", "/wrong/publickeys", "/"} {
		up.last = nil
		w := do(app, http.MethodGet, target, host)
		if w.Code != http.StatusNotFound || up.last != nil {
			t.Errorf("%s: status = %d, forwarded = %v; want 404 and no forward", target, w.Code, up.last != nil)
		}
	}

	// A token implies no landing page.
	w = do(app, http.MethodGet, "/", map[string]string{"Host": "yggdrasil.example.com", "Accept": "text/html"})
	if w.Code != http.StatusNotFound {
		t.Errorf("landing status = %d, want 404", w.Code)
	}
}

func TestBadGateway(t *testing.T) {
	up := newUpstream(t)
	up.Close()
	app := newTestApp(t, Config{}, up)
	logs := captureLog(t)

	w := do(app, http.MethodGet, "/publickeys", map[string]string{"Host": "services.yggdrasil.example.com"})
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body["error"] != "Bad Gateway" {
		t.Errorf("body = %s", w.Body)
	}
	if !strings.Contains(logs.String(), `"event":"proxy_error"`) {
		t.Errorf("log = %q", logs)
	}
}

func captureLog(t *testing.T) *strings.Builder {
	var b strings.Builder
	prev := log.Writer()
	log.SetOutput(&b)
	t.Cleanup(func() { log.SetOutput(prev) })
	return &b
}

func TestModulesGateSubdomains(t *testing.T) {
	up := newUpstream(t)
	app := newTestApp(t, Config{Modules: map[Module]bool{ModuleAuthlibInjector: true}}, up)

	for host, want := range map[string]int{
		"session.yggdrasil.example.com":          http.StatusNotFound,
		"prism-meta.yggdrasil.example.com":       http.StatusNotFound,
		"authlib-injector.yggdrasil.example.com": http.StatusNoContent,
	} {
		up.last = nil
		w := do(app, http.MethodGet, "/sessionserver/session/minecraft/hasJoined", map[string]string{"Host": host})
		if w.Code != want || (want == http.StatusNotFound) != (up.last == nil) {
			t.Errorf("%s: status = %d, want %d", host, w.Code, want)
		}
	}

	w := do(app, http.MethodGet, "/", map[string]string{"Host": "yggdrasil.example.com"})
	if body := w.Body.String(); strings.Contains(body, "-Dminecraft.api") || strings.Contains(body, "prism-meta") || !strings.Contains(body, "-javaagent:") {
		t.Errorf("usage text lists disabled modules: %s", body)
	}
}

func TestParseModules(t *testing.T) {
	got, err := ParseModules(" Yggdrasil, ,prism-meta,")
	if err != nil || len(got) != 2 || !got[ModuleYggdrasil] || !got[ModulePrismMeta] {
		t.Errorf("got %v, %v", got, err)
	}
	if got, err := ParseModules(""); err != nil || len(got) != 0 {
		t.Errorf("empty: got %v, %v", got, err)
	}
	if _, err := ParseModules("yggdrasil,bogus"); err == nil {
		t.Error("unknown module accepted")
	}
}

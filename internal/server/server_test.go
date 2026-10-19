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

func newTestApp(t *testing.T, cfg Config, up *upstream) *Server {
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

func TestUsageText(t *testing.T) {
	app := newTestApp(t, Config{}, nil)
	w := do(app, http.MethodGet, "/", map[string]string{
		"Host":              "internal:8787",
		"X-Forwarded-Proto": "https",
		"X-Forwarded-Host":  "yggdrasil.example.com",
	})
	if w.Code != http.StatusOK || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("status = %d, content-type = %q", w.Code, w.Header().Get("Content-Type"))
	}
	body := w.Body.String()
	for _, want := range []string{
		"https://session.yggdrasil.example.com -> https://sessionserver.mojang.com",
		"-Dminecraft.api.profiles.host=https://profiles.yggdrasil.example.com",
		"-Dmojang.sessionserver=https://session.yggdrasil.example.com/session/minecraft/hasJoined",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("usage text missing %q", want)
		}
	}
	if w.Header().Get("Vary") != "Accept" {
		t.Errorf("Vary = %q", w.Header().Get("Vary"))
	}
}

func TestLandingPage(t *testing.T) {
	app := newTestApp(t, Config{}, nil)
	html := map[string]string{"Host": "yggdrasil.example.com", "Accept": "text/html"}

	w := do(app, http.MethodGet, "/", html)
	if w.Code != http.StatusOK || w.Body.String() != "<p>https://session.yggdrasil.example.com</p>" {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body)
	}
	if w.Header().Get("Vary") != "Accept, Accept-Language, Cookie" {
		t.Errorf("Vary = %q", w.Header().Get("Vary"))
	}

	// The canonical host is served untouched; non-HTML is never rewritten.
	w = do(app, http.MethodGet, "/", map[string]string{"Host": CanonicalHost, "Accept": "text/html"})
	if !strings.Contains(w.Body.String(), "session."+CanonicalHost) {
		t.Errorf("canonical body = %q", w.Body)
	}
	w = do(app, http.MethodGet, "/favicon.svg", html)
	if w.Body.String() != "<svg>"+CanonicalHost+"</svg>" {
		t.Errorf("favicon body = %q", w.Body)
	}

	for _, target := range []string{"/zh-cn/", "/zh-cn"} {
		if w := do(app, http.MethodGet, target, html); w.Body.String() != "<p>zh</p>" {
			t.Errorf("%s: status = %d, body = %q", target, w.Code, w.Body)
		}
	}
	for _, target := range []string{"/missing", "/../../etc/passwd", "/.gitkeep"} {
		if w := do(app, http.MethodGet, target, html); w.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", target, w.Code)
		}
	}
	if w := do(app, http.MethodPost, "/", html); w.Code != http.StatusNotFound {
		t.Errorf("POST /: status = %d, want 404", w.Code)
	}
}

func TestChineseRedirect(t *testing.T) {
	app := newTestApp(t, Config{}, nil)
	h := map[string]string{"Host": "yggdrasil.example.com", "Accept": "text/html", "Accept-Language": "en;q=0.5, zh-TW"}

	w := do(app, http.MethodGet, "/", h)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/zh-cn/" {
		t.Fatalf("status = %d, location = %q", w.Code, w.Header().Get("Location"))
	}
	if !strings.HasPrefix(w.Header().Get("Set-Cookie"), "yggdrasil_lang=zh") {
		t.Errorf("Set-Cookie = %q", w.Header().Get("Set-Cookie"))
	}

	h["Cookie"] = "yggdrasil_lang=zh"
	if w := do(app, http.MethodGet, "/", h); w.Code != http.StatusOK {
		t.Errorf("with cookie: status = %d, want 200", w.Code)
	}
}

func TestDisableLandingPage(t *testing.T) {
	app := newTestApp(t, Config{DisableLandingPage: true}, nil)
	w := do(app, http.MethodGet, "/", map[string]string{"Host": "yggdrasil.example.com"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	var body map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["errorMessage"] != "Unknown endpoint." {
		t.Errorf("errorMessage = %q", body["errorMessage"])
	}
}

func TestPrefersChinese(t *testing.T) {
	for header, want := range map[string]bool{
		"":                        false,
		"zh-CN,zh;q=0.9,en;q=0.8": true,
		"en-US,zh;q=0.9":          false,
		"en;q=0.5,zh-TW":          true,
		"zh;q=0.5,en;q=0.5":       true,
		"en;q=abc,zh;q=0.1":       true,
		"en;q=,zh;q=0.1":          true,
	} {
		if got := prefersChinese(header); got != want {
			t.Errorf("prefersChinese(%q) = %v, want %v", header, got, want)
		}
	}
}

func captureLog(t *testing.T) *strings.Builder {
	var b strings.Builder
	prev := log.Writer()
	log.SetOutput(&b)
	t.Cleanup(func() { log.SetOutput(prev) })
	return &b
}

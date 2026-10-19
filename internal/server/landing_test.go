package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

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
		"-javaagent:authlib-injector.jar=https://authlib-injector.yggdrasil.example.com",
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

package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestAuthlibInjector(t *testing.T) {
	up := newUpstream(t)
	app := newTestApp(t, Config{}, up)
	host := map[string]string{"Host": "authlib-injector.yggdrasil.example.com", "X-Forwarded-Proto": "https"}

	w := do(app, http.MethodGet, "/", host)
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("metadata: status = %d, content-type = %q", w.Code, w.Header().Get("Content-Type"))
	}
	var meta struct {
		Meta               map[string]any `json:"meta"`
		SkinDomains        []string       `json:"skinDomains"`
		SignaturePublickey string         `json:"signaturePublickey"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &meta); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(meta.SignaturePublickey, "-----BEGIN PUBLIC KEY-----\n") {
		t.Errorf("signaturePublickey = %q", meta.SignaturePublickey)
	}
	if links, _ := meta.Meta["links"].(map[string]any); links["homepage"] != "https://yggdrasil.example.com/" {
		t.Errorf("links = %v", meta.Meta["links"])
	}

	for target, want := range map[string]string{
		"/sessionserver/session/minecraft/hasJoined?username=a&serverId=x": "/session/minecraft/hasJoined?username=a&serverId=x",
		"/api/profiles/minecraft":       "/profiles/minecraft",
		"/authserver/authenticate":      "/authenticate",
		"/minecraftservices/publickeys": "/publickeys",
	} {
		up.last = nil
		if w := do(app, http.MethodGet, target, host); w.Code != http.StatusNoContent || up.last == nil {
			t.Errorf("%s: status = %d, want 204", target, w.Code)
			continue
		}
		if got := up.last.URL.RequestURI(); got != want {
			t.Errorf("%s: upstream URI = %q, want %q", target, got, want)
		}
	}

	for _, target := range []string{"/unknown/x", "/session/minecraft/hasJoined"} {
		up.last = nil
		if w := do(app, http.MethodGet, target, host); w.Code != http.StatusNotFound || up.last != nil {
			t.Errorf("%s: status = %d, forwarded = %v; want 404 and no forward", target, w.Code, up.last != nil)
		}
	}
	if w := do(app, http.MethodPost, "/", host); w.Code != http.StatusNotFound {
		t.Errorf("POST /: status = %d, want 404", w.Code)
	}
}

func TestAuthlibInjectorToken(t *testing.T) {
	up := newUpstream(t)
	app := newTestApp(t, Config{ProxyToken: "tok"}, up)
	host := map[string]string{"Host": "authlib-injector.yggdrasil.example.com"}

	w := do(app, http.MethodGet, "/tok/", host)
	if w.Code != http.StatusOK {
		t.Fatalf("metadata: status = %d, want 200", w.Code)
	}
	if strings.Contains(w.Body.String(), "homepage") {
		t.Errorf("private instance advertises a homepage: %s", w.Body)
	}
	if w := do(app, http.MethodGet, "/tok/sessionserver/session/minecraft/join", host); w.Code != http.StatusNoContent || up.last.URL.Path != "/session/minecraft/join" {
		t.Errorf("proxied: status = %d", w.Code)
	}
	for _, target := range []string{"/", "/sessionserver/session/minecraft/join", "/bad/"} {
		if w := do(app, http.MethodGet, target, host); w.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", target, w.Code)
		}
	}
}

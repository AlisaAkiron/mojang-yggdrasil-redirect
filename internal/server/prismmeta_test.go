package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newPrismUpstream serves a fixed body with the given content type and
// records the last request.
func newPrismUpstream(t *testing.T, contentType, body string) (*httptest.Server, **http.Request) {
	var last *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		last = r
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &last
}

func TestPrismMetaRewritesDownloads(t *testing.T) {
	up, last := newPrismUpstream(t, "application/json", `{
		"formatVersion": 1,
		"versions": [{"version": "1.21", "sha256": "abc", "releaseTime": "2024-06-13T08:24:03+00:00"}],
		"libraries": [{"url": "https://libraries.minecraft.net/", "sha1": "keep"}],
		"mainJar": {"url": "https://piston-data.mojang.com/v1/objects/x/client.jar"},
		"liteloader": ["https://dl.liteloader.com/versions/versions.json", "https://dl.liteloader.com/versions/x.jar"]
	}`)
	app := newTestApp(t, Config{PrismMetaUpstream: up.URL + "/v1"}, nil)

	w := do(app, http.MethodGet, "/net.minecraft/index.json", map[string]string{
		"Host":            "prism-meta.yggdrasil.example.com",
		"Accept-Encoding": "gzip",
		"Range":           "bytes=0-10",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if got := (*last).URL.Path; got != "/v1/net.minecraft/index.json" {
		t.Errorf("upstream path = %q", got)
	}
	if (*last).Header.Get("Range") != "" {
		t.Error("Range forwarded")
	}
	if w.Header().Get("Content-Type") != "application/json; charset=utf-8" || w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("headers = %v", w.Header())
	}

	var doc struct {
		Versions   []map[string]any `json:"versions"`
		Libraries  []map[string]any `json:"libraries"`
		MainJar    map[string]any   `json:"mainJar"`
		LiteLoader []string         `json:"liteloader"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatalf("%v: %s", err, w.Body)
	}
	if _, ok := doc.Versions[0]["sha256"]; ok || doc.Versions[0]["version"] != "1.21" {
		t.Errorf("versions = %v", doc.Versions)
	}
	if doc.Libraries[0]["url"] != "https://bmclapi2.bangbang93.com/maven/" || doc.Libraries[0]["sha1"] != "keep" {
		t.Errorf("libraries = %v", doc.Libraries)
	}
	if doc.MainJar["url"] != "https://bmclapi2.bangbang93.com/v1/objects/x/client.jar" {
		t.Errorf("mainJar = %v", doc.MainJar)
	}
	if doc.LiteLoader[0] != "https://bmclapi2.bangbang93.com/maven/com/mumfrey/liteloader/versions.json" ||
		doc.LiteLoader[1] != "https://bmclapi2.bangbang93.com/maven/x.jar" {
		t.Errorf("liteloader = %v", doc.LiteLoader)
	}
}

func TestPrismMetaPassesThroughNonJSON(t *testing.T) {
	const body = "https://libraries.minecraft.net/"
	up, _ := newPrismUpstream(t, "application/octet-stream", body)
	app := newTestApp(t, Config{PrismMetaUpstream: up.URL + "/v1"}, nil)

	w := do(app, http.MethodGet, "/x.bin", map[string]string{"Host": "prism-meta.yggdrasil.example.com"})
	if w.Code != http.StatusOK || w.Body.String() != body {
		t.Errorf("status = %d, body = %q", w.Code, w.Body)
	}
}

func TestPrismMetaToken(t *testing.T) {
	up, last := newPrismUpstream(t, "application/json", `{}`)
	app := newTestApp(t, Config{ProxyToken: "tok", PrismMetaUpstream: up.URL + "/v1"}, nil)
	host := map[string]string{"Host": "prism-meta.yggdrasil.example.com"}

	if w := do(app, http.MethodGet, "/tok/index.json", host); w.Code != http.StatusOK || (*last).URL.Path != "/v1/index.json" {
		t.Errorf("with token: status = %d", w.Code)
	}
	*last = nil
	if w := do(app, http.MethodGet, "/index.json", host); w.Code != http.StatusNotFound || *last != nil {
		t.Errorf("without token: status = %d", w.Code)
	}
}

func TestStripIndexHashes(t *testing.T) {
	in := `{"packages":[{"uid":"a","sha256":"x"}],"formatVersion":1}`
	out := string(stripIndexHashes([]byte(in)))
	if strings.Contains(out, "sha256") || !strings.Contains(out, `"uid":"a"`) {
		t.Errorf("out = %s", out)
	}
	for _, unchanged := range []string{`{"versions":[{"version":"1"}]}`, `not json`, `[1,2]`} {
		if got := string(stripIndexHashes([]byte(unchanged))); got != unchanged {
			t.Errorf("%s -> %s", unchanged, got)
		}
	}
}

package server

import (
	"cmp"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// Config is the runtime configuration, read from the environment in main.
type Config struct {
	// Modules is the set of enabled modules; anything absent is disabled.
	Modules map[Module]bool
	// ProxyToken makes the instance private: every proxied request must carry
	// the token as the first path segment, e.g.
	//   -Dminecraft.api.session.host=https://session.yggdrasil.example.com/<token>
	// The segment is stripped before forwarding. The token lives in the URL —
	// the only credential channel vanilla authlib clients can reproduce — so
	// the Authorization header stays free for Mojang's own Bearer tokens.
	ProxyToken string
	// DisableLandingPage makes the instance proxy-only: the base domain serves
	// no documentation at all (HTML or plain text) and returns the JSON 404
	// like any unknown endpoint. A configured ProxyToken implies this — a
	// private proxy has no business serving public docs.
	DisableLandingPage bool
	// Site is the generated docs site (site/dist). Required unless the
	// landing page is disabled.
	Site fs.FS
	// Transport overrides the upstream transport (tests).
	Transport http.RoundTripper
	// Upstreams overrides the service upstream URLs (tests).
	Upstreams map[Service]string
	// PrismMetaUpstream overrides PrismMetaUpstream (tests).
	PrismMetaUpstream string
}

// Server is the HTTP handler: it proxies service subdomains and serves the
// landing page on the base domain.
type Server struct {
	tokenHash      [32]byte
	tokenEnabled   bool
	landingEnabled bool
	modules        map[Module]bool
	site           *site
	proxies        map[Service]*httputil.ReverseProxy
	prismMeta      *httputil.ReverseProxy
}

func New(cfg Config) (*Server, error) {
	a := &Server{
		tokenEnabled:   cfg.ProxyToken != "",
		tokenHash:      sha256.Sum256([]byte(cfg.ProxyToken)),
		landingEnabled: cfg.ProxyToken == "" && !cfg.DisableLandingPage,
		modules:        cfg.Modules,
		proxies:        map[Service]*httputil.ReverseProxy{},
	}
	upstreams := cfg.Upstreams
	if upstreams == nil {
		upstreams = Upstreams
	}
	for s, raw := range upstreams {
		target, err := url.Parse(raw)
		if err != nil {
			return nil, err
		}
		a.proxies[s] = a.newProxy(target, cfg.Transport)
	}
	prismTarget, err := url.Parse(cmp.Or(cfg.PrismMetaUpstream, PrismMetaUpstream))
	if err != nil {
		return nil, err
	}
	a.prismMeta = a.newPrismMetaProxy(prismTarget, cfg.Transport)
	if a.landingEnabled {
		if cfg.Site == nil {
			return nil, errors.New("landing page enabled but no docs site given")
		}
		a.site = &site{files: cfg.Site}
	}
	return a, nil
}

// ServeHTTP dispatches on the first hostname label: a service subdomain
// (e.g. session.yggdrasil.alisaqaq.moe) is proxied with its path forwarded
// as-is, the authlib-injector subdomain routes by path prefix, the prism-meta
// subdomain serves rewritten launcher metadata, and anything else is the base
// domain's landing page. A subdomain whose module is disabled gets the JSON
// 404. With a proxy token, every proxy subdomain requires it as the first
// path segment (see stripToken).
func (a *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	label, _, _ := strings.Cut(hostname(r.Host), ".")
	var handler http.HandlerFunc
	var module Module
	switch proxy, isService := a.proxies[Service(label)]; {
	case label == AuthlibInjectorLabel:
		handler, module = a.serveAuthlibInjector, ModuleAuthlibInjector
	case label == PrismMetaLabel:
		handler, module = a.prismMeta.ServeHTTP, ModulePrismMeta
	case isService:
		handler, module = proxy.ServeHTTP, ModuleYggdrasil
	default:
		a.serveLanding(w, r)
		return
	}
	if !a.modules[module] {
		a.notFound(w)
		return
	}

	if a.tokenEnabled {
		var ok bool
		if r, ok = a.stripToken(r); !ok {
			a.notFound(w)
			return
		}
	}
	handler(w, r)
}

func (a *Server) notFound(w http.ResponseWriter) {
	msg := "Unknown endpoint. See the base domain for usage."
	if !a.landingEnabled {
		msg = "Unknown endpoint."
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not Found", "errorMessage": msg})
}

func hostname(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.ToLower(host)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func logJSON(v any) {
	b, _ := json.Marshal(v)
	log.Println(string(b))
}

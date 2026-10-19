package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"math"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Config is the runtime configuration, read from the environment in main.
type Config struct {
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
}

// Server is the HTTP handler: it proxies service subdomains and serves the
// landing page on the base domain.
type Server struct {
	tokenHash      [32]byte
	tokenEnabled   bool
	landingEnabled bool
	site           *site
	proxies        map[Service]*httputil.ReverseProxy
}

func New(cfg Config) (*Server, error) {
	a := &Server{
		tokenEnabled:   cfg.ProxyToken != "",
		tokenHash:      sha256.Sum256([]byte(cfg.ProxyToken)),
		landingEnabled: cfg.ProxyToken == "" && !cfg.DisableLandingPage,
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
	if a.landingEnabled {
		if cfg.Site == nil {
			return nil, errors.New("landing page enabled but no docs site given")
		}
		a.site = &site{files: cfg.Site}
	}
	return a, nil
}

// Hop-by-hop headers plus anything that reveals the original client or the
// Cloudflare hop. httputil.ReverseProxy already drops the hop-by-hop set;
// they are listed again so the policy reads in one place.
var strippedHeaders = map[string]bool{
	"connection":          true,
	"keep-alive":          true,
	"proxy-authenticate":  true,
	"proxy-authorization": true,
	"te":                  true,
	"trailer":             true,
	"transfer-encoding":   true,
	"upgrade":             true,
	"host":                true,
	"forwarded":           true,
	"x-real-ip":           true,
	"true-client-ip":      true,
	"cdn-loop":            true,
}

func (a *Server) newProxy(target *url.URL, transport http.RoundTripper) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Transport: transport,
		Rewrite: func(pr *httputil.ProxyRequest) {
			// SetURL joins the target with the (already token-stripped) inbound
			// path and points the Host header at the upstream.
			pr.SetURL(target)
			for name := range pr.Out.Header {
				lower := strings.ToLower(name)
				if strings.HasPrefix(lower, "cf-") || strings.HasPrefix(lower, "x-forwarded-") || strippedHeaders[lower] {
					pr.Out.Header.Del(name)
				}
			}
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			logJSON(map[string]string{"event": "proxy_error", "path": r.URL.Path, "error": err.Error()})
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error":        "Bad Gateway",
				"errorMessage": "Failed to reach the upstream Mojang service.",
			})
		},
	}
}

func (a *Server) tokenMatches(candidate string) bool {
	// Hash both sides to equal length so the comparison is timing-safe.
	h := sha256.Sum256([]byte(candidate))
	return subtle.ConstantTimeCompare(h[:], a.tokenHash[:]) == 1
}

// Each service lives on its own subdomain (e.g. session.yggdrasil.alisaqaq.moe);
// the first hostname label selects the upstream and the path is forwarded
// as-is. With a proxy token, the path must start with /<token>, which is
// validated and stripped; anything else gets the same 404 as unknown
// endpoints, revealing nothing.
func (a *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	label, _, _ := strings.Cut(hostname(r.Host), ".")
	if proxy, ok := a.proxies[Service(label)]; ok && isService(label) {
		if a.tokenEnabled {
			escaped := strings.TrimPrefix(r.URL.EscapedPath(), "/")
			first, rest, _ := strings.Cut(escaped, "/")
			candidate, err := url.PathUnescape(first)
			if err != nil || !utf8.ValidString(candidate) {
				candidate = ""
			}
			if candidate == "" || !a.tokenMatches(candidate) {
				a.notFound(w)
				return
			}
			r = r.Clone(r.Context())
			if err := setEscapedPath(r.URL, "/"+rest); err != nil {
				a.notFound(w)
				return
			}
		}
		proxy.ServeHTTP(w, r)
		return
	}

	if a.landingEnabled {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			if r.URL.Path == "/" && a.serveRoot(w, r) {
				return
			}
			if a.site.serve(w, r, rewriteHost(r)) {
				return
			}
		}
	}
	a.notFound(w)
}

// serveRoot handles the base domain (yggdrasil.alisaqaq.moe): anything not asking
// for text/html (curl, scripts) gets plain-text usage; browsers get the docs
// site. It reports false when the static site should take over.
func (a *Server) serveRoot(w http.ResponseWriter, r *http.Request) bool {
	if !strings.Contains(r.Header.Get("Accept"), "text/html") {
		w.Header().Set("Vary", "Accept")
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = w.Write([]byte(renderUsageText(modelFrom(r))))
		}
		return true
	}
	w.Header().Set("Vary", "Accept, Accept-Language, Cookie")
	// First HTML visit only: if this visitor hasn't been sorted yet and their
	// browser prefers Chinese, send them to the localized docs and remember it
	// with a cookie. Without the cookie the language picker's link back to `/`
	// would be bounced straight to /zh-cn/ again, trapping them on Chinese.
	if _, err := r.Cookie("yggdrasil_lang"); err != nil && prefersChinese(r.Header.Get("Accept-Language")) {
		http.SetCookie(w, &http.Cookie{
			Name:     "yggdrasil_lang",
			Value:    "zh",
			Path:     "/",
			MaxAge:   60 * 60 * 24 * 365,
			SameSite: http.SameSiteLaxMode,
		})
		w.Header().Set("Location", "/zh-cn/")
		w.WriteHeader(http.StatusFound)
		return true
	}
	return false
}

func (a *Server) notFound(w http.ResponseWriter) {
	msg := "Unknown endpoint. See the base domain for usage."
	if !a.landingEnabled {
		msg = "Unknown endpoint."
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not Found", "errorMessage": msg})
}

func modelFrom(r *http.Request) DeploymentModel {
	proto := "http"
	if r.TLS != nil {
		proto = "https"
	}
	return buildModel(proto, r.Host, r.Header.Get("X-Forwarded-Proto"), r.Header.Get("X-Forwarded-Host"))
}

// rewriteHost is the domain the docs should show: the serving domain, as
// seen by the visitor.
func rewriteHost(r *http.Request) string {
	if h := r.Header.Get("X-Forwarded-Host"); h != "" {
		return h
	}
	return r.Host
}

func renderUsageText(m DeploymentModel) string {
	var b strings.Builder
	b.WriteString("mojang-yggdrasil-redirect\n\n")
	fmt.Fprintf(&b, "A reverse proxy for the %d Mojang services used by Minecraft (authlib).\n\n", len(Services))
	b.WriteString("Endpoints:\n")
	for _, s := range Services {
		fmt.Fprintf(&b, "  %s -> %s\n", m.ServiceURL(s), Upstreams[s])
	}
	b.WriteString("\nUsage (Minecraft client / server, MC 1.16+): add these JVM flags before -jar\n\n")
	for _, f := range m.Flags {
		fmt.Fprintf(&b, "  %s\n", f)
	}
	fmt.Fprintf(&b, "\nOne-liner:\n\n  %s\n", m.OneLiner)
	fmt.Fprintf(&b, "\nVelocity:\n\n  %s\n", m.VelocityFlag)
	return b.String()
}

// prefersChinese picks the visitor's top-preference language from
// Accept-Language (honoring q-values) and reports whether it is any Chinese
// variant. We only ship Simplified Chinese, so zh-TW/zh-HK fall through to it
// too.
func prefersChinese(header string) bool {
	var topTag string
	topQ := -1.0
	for part := range strings.SplitSeq(header, ",") {
		tag, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		tag = strings.ToLower(strings.TrimSpace(tag))
		q := 1.0
		for p := range strings.SplitSeq(params, ";") {
			if v, ok := strings.CutPrefix(strings.TrimSpace(p), "q="); ok {
				q = parseQ(v)
				break
			}
		}
		// Strictly greater keeps the earliest entry among equal q-values.
		if tag != "" && q == q && q > topQ {
			topTag, topQ = tag, q
		}
	}
	return strings.HasPrefix(topTag, "zh")
}

// parseQ parses an Accept-Language q-value, returning NaN for anything that
// is not a finite number so the caller can skip the entry.
func parseQ(v string) float64 {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	q, err := strconv.ParseFloat(v, 64)
	if err != nil || math.IsInf(q, 0) {
		return math.NaN()
	}
	return q
}

func hostname(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.ToLower(host)
}

// setEscapedPath sets both the decoded and raw form of u's path from an
// escaped path, so percent-encoding reaches the upstream unchanged.
func setEscapedPath(u *url.URL, escaped string) error {
	p, err := url.PathUnescape(escaped)
	if err != nil {
		return err
	}
	u.Path, u.RawPath = p, escaped
	return nil
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

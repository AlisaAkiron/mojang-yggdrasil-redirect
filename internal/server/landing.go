package server

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
)

// serveLanding handles the base domain: the docs site for browsers, plain
// usage text for everything else, and the JSON 404 when the landing page is
// disabled.
func (a *Server) serveLanding(w http.ResponseWriter, r *http.Request) {
	if a.landingEnabled && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
		if r.URL.Path == "/" && a.serveRoot(w, r) {
			return
		}
		if a.site.serve(w, r, rewriteHost(r)) {
			return
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
	b.WriteString("\nOlder versions (pre-1.16, e.g. 1.7.10) via authlib-injector: add before -jar\n\n")
	fmt.Fprintf(&b, "  %s\n", m.JavaAgentFlag)
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

package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"unicode/utf8"
)

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

// stripToken validates the proxy token in the first path segment and returns
// the request without it. Callers answer a failure with the same 404 as
// unknown endpoints, revealing nothing.
func (a *Server) stripToken(r *http.Request) (*http.Request, bool) {
	candidate, rest, ok := cutFirstSegment(r)
	if !ok || candidate == "" || !a.tokenMatches(candidate) {
		return nil, false
	}
	return rest, true
}

func (a *Server) tokenMatches(candidate string) bool {
	// Hash both sides to equal length so the comparison is timing-safe.
	h := sha256.Sum256([]byte(candidate))
	return subtle.ConstantTimeCompare(h[:], a.tokenHash[:]) == 1
}

// cutFirstSegment splits r's path after its first segment: it returns that
// segment unescaped and a clone of r whose path is the remainder, with the
// original percent-encoding preserved. It reports false for a malformed path.
func cutFirstSegment(r *http.Request) (string, *http.Request, bool) {
	first, rest, _ := strings.Cut(strings.TrimPrefix(r.URL.EscapedPath(), "/"), "/")
	segment, err := url.PathUnescape(first)
	if err != nil || !utf8.ValidString(segment) {
		return "", nil, false
	}
	r = r.Clone(r.Context())
	if err := setEscapedPath(r.URL, "/"+rest); err != nil {
		return "", nil, false
	}
	return segment, r, true
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

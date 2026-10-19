package server

import (
	"bytes"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

// site serves the generated docs site.
type site struct {
	files fs.FS
}

// serve writes the static file for r's path, or reports false when there is
// none. Directories resolve to their index.html; dotfiles are never served.
// The pages are generated with the canonical public domain; HTML is rewritten
// to host so self-hosted instances (and local dev) render their own URLs.
func (s *site) serve(w http.ResponseWriter, r *http.Request, host string) bool {
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name == "" {
		name = "."
	}
	if strings.HasPrefix(name, ".") && name != "." || strings.Contains(name, "/.") {
		return false
	}
	if info, err := fs.Stat(s.files, name); err == nil && info.IsDir() {
		name = path.Join(name, "index.html")
	}
	body, err := fs.ReadFile(s.files, name)
	if err != nil {
		return false
	}

	contentType := mime.TypeByExtension(path.Ext(name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	if strings.HasPrefix(contentType, "text/html") && host != CanonicalHost {
		body = bytes.ReplaceAll(body, []byte(CanonicalHost), []byte(host))
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(body))
	return true
}

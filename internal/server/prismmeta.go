package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
)

// PrismMetaLabel is the subdomain that mirrors Prism Launcher's metadata
// server with every download URL pointed at BMCLAPI, for players who can
// reach neither Mojang's nor the mod loaders' download hosts. Set it as the
// "Meta URL" in Prism's API settings. Ported from
// https://github.com/SheepYhangCN/prism-meta-bmclapi (MIT).
const PrismMetaLabel = "prism-meta"

// PrismMetaUpstream is the metadata root; the inbound path is appended.
const PrismMetaUpstream = "https://meta.prismlauncher.org/v1"

// prismMetaMaxBody caps how much of an upstream document is buffered for
// rewriting. The largest index files are a few MB.
const prismMetaMaxBody = 64 << 20

// bmclapiReplacer points the download hosts in the metadata at BMCLAPI. At
// each position the earliest-listed match wins, so longer prefixes of the
// same host come first.
var bmclapiReplacer = strings.NewReplacer(
	// Mojang
	"launchermeta.mojang.com", "bmclapi2.bangbang93.com",
	"piston-meta.mojang.com", "bmclapi2.bangbang93.com",
	"piston-data.mojang.com", "bmclapi2.bangbang93.com",
	"launcher.mojang.com", "bmclapi2.bangbang93.com",
	"resources.download.minecraft.net", "bmclapi2.bangbang93.com/assets",
	"libraries.minecraft.net", "bmclapi2.bangbang93.com/maven",
	// Fabric
	"meta.fabricmc.net", "bmclapi2.bangbang93.com/fabric-meta",
	"maven.fabricmc.net", "bmclapi2.bangbang93.com/maven",
	// Forge
	"maven.minecraftforge.net", "bmclapi2.bangbang93.com/maven",
	"files.minecraftforge.net/maven", "bmclapi2.bangbang93.com/maven",
	// LiteLoader
	"dl.liteloader.com/versions/versions.json", "bmclapi2.bangbang93.com/maven/com/mumfrey/liteloader/versions.json",
	"dl.liteloader.com/versions", "bmclapi2.bangbang93.com/maven",
	// NeoForge
	"maven.neoforged.net/releases", "bmclapi2.bangbang93.com/maven",
	// Quilt
	"maven.quiltmc.org/repository/release", "bmclapi2.bangbang93.com/maven",
	"meta.quiltmc.org", "bmclapi2.bangbang93.com/quilt-meta",
)

func (a *Server) newPrismMetaProxy(target *url.URL, transport http.RoundTripper) *httputil.ReverseProxy {
	p := a.newProxy(target, transport)
	rewrite := p.Rewrite
	p.Rewrite = func(pr *httputil.ProxyRequest) {
		rewrite(pr)
		// The body is rewritten, so fetch it whole and uncompressed (the
		// transport negotiates and decodes gzip itself without these).
		pr.Out.Header.Del("Accept-Encoding")
		pr.Out.Header.Del("Range")
		pr.Out.Header.Del("If-Range")
	}
	p.ModifyResponse = rewritePrismMeta
	p.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logJSON(map[string]string{"event": "proxy_error", "path": r.URL.Path, "error": err.Error()})
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error":        "Bad Gateway",
			"errorMessage": "Failed to fetch the Prism Launcher metadata.",
		})
	}
	return p
}

// rewritePrismMeta rewrites a successful JSON/text metadata response: the
// download hosts become BMCLAPI, and the index hashes, which no longer match
// the rewritten documents, are dropped.
func rewritePrismMeta(resp *http.Response) error {
	ct := resp.Header.Get("Content-Type")
	if resp.StatusCode != http.StatusOK || (!strings.Contains(ct, "json") && !strings.Contains(ct, "text")) {
		return nil
	}
	if resp.Request.Method == http.MethodHead {
		// The rewritten length is unknown without the body.
		resp.Header.Del("Content-Length")
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, prismMetaMaxBody+1))
	resp.Body.Close()
	if err != nil {
		return err
	}
	if len(body) > prismMetaMaxBody {
		return errors.New("prism meta document too large")
	}
	body = stripIndexHashes([]byte(bmclapiReplacer.Replace(string(body))))

	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
	resp.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp.Header.Set("Access-Control-Allow-Origin", "*")
	return nil
}

// stripIndexHashes removes the sha256/sha1 of each entry in an index
// document's "versions" (package index) or "packages" (root index): they hash
// the upstream files, which we rewrite. Prism skips verification without
// them. Anything else is returned unchanged.
func stripIndexHashes(body []byte) []byte {
	var doc map[string]json.RawMessage
	if json.Unmarshal(body, &doc) != nil {
		return body
	}
	modified := false
	for _, key := range []string{"versions", "packages"} {
		var entries []map[string]json.RawMessage
		if json.Unmarshal(doc[key], &entries) != nil {
			continue
		}
		stripped := false
		for _, e := range entries {
			for _, h := range []string{"sha256", "sha1"} {
				if _, ok := e[h]; ok {
					delete(e, h)
					stripped = true
				}
			}
		}
		if !stripped {
			continue
		}
		raw, err := json.Marshal(entries)
		if err != nil {
			return body
		}
		doc[key], modified = raw, true
	}
	if !modified {
		return body
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return body
	}
	return out
}

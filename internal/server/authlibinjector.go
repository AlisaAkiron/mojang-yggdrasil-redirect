package server

import (
	"net/http"
	"strings"
)

// AuthlibInjectorLabel is the subdomain that speaks the authlib-injector
// Yggdrasil API layout, for Minecraft versions whose authlib hardcodes the
// Mojang URLs (pre-1.16, e.g. 1.7.10 modpacks). authlib-injector rewrites
// https://<mojang host>/<path> to <api root>/<prefix>/<path>; the prefix
// picks the upstream and is stripped before forwarding.
const AuthlibInjectorLabel = "authlib-injector"

var authlibInjectorPrefixes = map[string]Service{
	"authserver":        "auth",
	"sessionserver":     "session",
	"api":               "account",
	"minecraftservices": "services",
}

// mojangSessionPublicKey is Mojang's Yggdrasil session key, the one vanilla
// authlib ships as yggdrasil_session_pubkey.der (and lists in
// api.minecraftservices.com/publickeys). authlib-injector verifies profile
// property signatures against the key in our metadata instead of authlib's,
// and the textures we pass through are signed by Mojang.
const mojangSessionPublicKey = `-----BEGIN PUBLIC KEY-----
MIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAylB4B6m5lz7jwrcFz6Fd
/fnfUhcvlxsTSn5kIK/2aGG1C3kMy4VjhwlxF6BFUSnfxhNswPjh3ZitkBxEAFY2
5uzkJFRwHwVA9mdwjashXILtR6OqdLXXFVyUPIURLOSWqGNBtb08EN5fMnG8iFLg
EJIBMxs9BvF3s3/FhuHyPKiVTZmXY0WY4ZyYqvoKR+XjaTRPPvBsDa4WI2u1zxXM
eHlodT3lnCzVvyOYBLXL6CJgByuOxccJ8hnXfF9yY4F0aeL080Jz/3+EBNG8RO4B
yhtBf4Ny8NQ6stWsjfeUIvH7bU/4zCYcYOq4WrInXHqS8qruDmIl7P5XXGcabuzQ
stPf/h2CRAUpP/PlHXcMlvewjmGU6MfDK+lifScNYwjPxRo4nKTGFZf/0aqHCh/E
AsQyLKrOIYRE0lDG3bzBh8ogIMLAugsAfBb6M3mqCqKaTMAf/VAjh5FFJnjS+7bE
+bZEV0qwax1CEoPPJL1fIQjOS8zj086gjpGRCtSy9+bTPTfTR/SJ+VUB5G2IeCIt
kNHpJX2ygojFZ9n5Fnj7R9ZnOM+L8nyIjPu3aePvtcrXlyLhH/hvOfIOjPxOlqW+
O5QwSFP4OEcyLAUgDdUgyW36Z5mB285uKW/ighzZsOTevVUG2QwDItObIV6i8RCx
FbN2oDHyPaO5j1tTaBNyVt8CAwEAAQ==
-----END PUBLIC KEY-----
`

// serveAuthlibInjector handles the authlib-injector subdomain after any proxy
// token has been stripped: the root serves the API metadata, a known prefix
// forwards the rest of the path to its upstream.
func (a *Server) serveAuthlibInjector(w http.ResponseWriter, r *http.Request) {
	prefix, rest, ok := cutFirstSegment(r)
	if !ok {
		a.notFound(w)
		return
	}
	if prefix == "" && rest.URL.Path == "/" {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			a.notFound(w)
			return
		}
		writeJSON(w, http.StatusOK, a.authlibInjectorMetadata(modelFrom(r)))
		return
	}
	proxy, ok := a.proxies[authlibInjectorPrefixes[prefix]]
	if !ok {
		a.notFound(w)
		return
	}
	proxy.ServeHTTP(w, rest)
}

// authlibInjectorMetadata is the API root document authlib-injector fetches
// at startup. Every request ends up at Mojang, so the features make it behave
// like vanilla: profile keys and Mojang's own chat reporting/telemetry
// endpoints stay on, and the "@mojang" namespace is pointless.
func (a *Server) authlibInjectorMetadata(m DeploymentModel) map[string]any {
	meta := map[string]any{
		"serverName":                          "mojang-yggdrasil-redirect",
		"implementationName":                  "mojang-yggdrasil-redirect",
		"feature.non_email_login":             true,
		"feature.enable_profile_key":          true,
		"feature.enable_mojang_anti_features": true,
		"feature.no_mojang_namespace":         true,
	}
	if a.landingEnabled {
		// The docs live on the base domain, one label up.
		base := strings.TrimPrefix(m.Host, AuthlibInjectorLabel+".")
		meta["links"] = map[string]string{"homepage": m.Proto + "://" + base + "/"}
	}
	return map[string]any{
		"meta":               meta,
		"skinDomains":        []string{"textures.minecraft.net"},
		"signaturePublickey": mojangSessionPublicKey,
	}
}

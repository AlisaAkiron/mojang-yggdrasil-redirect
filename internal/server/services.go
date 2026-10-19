package server

// Service is the first hostname label that selects an upstream.
type Service string

// Services lists the proxied services in display order.
var Services = []Service{"auth", "account", "session", "services", "profiles"}

var Upstreams = map[Service]string{
	"auth":     "https://authserver.mojang.com",     // -Dminecraft.api.auth.host (legacy login; unused since MC 1.20.2)
	"account":  "https://api.mojang.com",            // -Dminecraft.api.account.host (player info; unused since MC 1.20.3)
	"session":  "https://sessionserver.mojang.com",  // -Dminecraft.api.session.host (online-mode join)
	"services": "https://api.minecraftservices.com", // -Dminecraft.api.services.host (chat reports etc.; since MC 1.16.4)
	"profiles": "https://api.mojang.com",            // -Dminecraft.api.profiles.host (player info; since authlib 7.0.61)
}

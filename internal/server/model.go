package server

import (
	"fmt"
	"strings"
)

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

func isService(name string) bool {
	_, ok := Upstreams[Service(name)]
	return ok
}

// CanonicalHost is the domain baked into the static site at build time; the
// HTML rewrite swaps it for the serving domain on self-hosted instances.
const CanonicalHost = "yggdrasil.alisaqaq.moe"

type DeploymentModel struct {
	Proto        string
	Host         string
	Origin       string
	Flags        []string
	OneLiner     string
	VelocityFlag string
}

func (m DeploymentModel) ServiceURL(s Service) string {
	return fmt.Sprintf("%s://%s.%s", m.Proto, s, m.Host)
}

func (m DeploymentModel) Flag(s Service) string {
	return fmt.Sprintf("-Dminecraft.api.%s.host=%s", s, m.ServiceURL(s))
}

// buildModel renders the public URLs of this deployment. Behind a
// TLS-terminating reverse proxy the request reaches us over HTTP, so the
// standard forwarded headers win when present.
func buildModel(proto, host, forwardedProto, forwardedHost string) DeploymentModel {
	if forwardedProto != "" {
		proto = forwardedProto
	}
	if forwardedHost != "" {
		host = forwardedHost
	}
	m := DeploymentModel{Proto: proto, Host: host, Origin: proto + "://" + host}
	for _, s := range Services {
		m.Flags = append(m.Flags, m.Flag(s))
	}
	m.OneLiner = strings.Join(m.Flags, " ")
	m.VelocityFlag = "-Dmojang.sessionserver=" + m.ServiceURL("session") + "/session/minecraft/hasJoined"
	return m
}

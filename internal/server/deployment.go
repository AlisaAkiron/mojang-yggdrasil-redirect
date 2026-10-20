package server

import (
	"fmt"
	"net/http"
	"strings"
)

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
	// AuthlibInjectorURL is the API root for the authlib-injector agent
	// (pre-1.16 versions, which ignore the minecraft.api.* flags).
	AuthlibInjectorURL string
	JavaAgentFlag      string
	// PrismMetaURL is the Meta URL for Prism Launcher's API settings.
	PrismMetaURL string
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
	m.AuthlibInjectorURL = fmt.Sprintf("%s://%s.%s", m.Proto, AuthlibInjectorLabel, m.Host)
	m.JavaAgentFlag = "-javaagent:authlib-injector.jar=" + m.AuthlibInjectorURL
	m.PrismMetaURL = fmt.Sprintf("%s://%s.%s/", m.Proto, PrismMetaLabel, m.Host)
	return m
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

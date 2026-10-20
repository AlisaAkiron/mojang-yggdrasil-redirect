package server

import (
	"fmt"
	"slices"
	"strings"
)

// Module is a feature that can be switched on with ENABLE_MODULE. Every
// module is off by default; a disabled module's subdomains get the JSON 404.
type Module string

const (
	// ModuleYggdrasil is the five Mojang service subdomains (see Services).
	ModuleYggdrasil Module = "yggdrasil"
	// ModuleAuthlibInjector is the authlib-injector subdomain.
	ModuleAuthlibInjector Module = "authlib-injector"
	// ModulePrismMeta is the Prism Launcher metadata subdomain.
	ModulePrismMeta Module = "prism-meta"
)

// Modules lists every module in display order.
var Modules = []Module{ModuleYggdrasil, ModuleAuthlibInjector, ModulePrismMeta}

// ParseModules parses a comma-separated module list such as
// "yggdrasil,prism-meta". Names are case-insensitive and blanks are ignored;
// an unknown name is an error.
func ParseModules(list string) (map[Module]bool, error) {
	enabled := map[Module]bool{}
	for name := range strings.SplitSeq(list, ",") {
		m := Module(strings.ToLower(strings.TrimSpace(name)))
		if m == "" {
			continue
		}
		if !slices.Contains(Modules, m) {
			return nil, fmt.Errorf("unknown module %q (valid: %s)", m, moduleNames())
		}
		enabled[m] = true
	}
	return enabled, nil
}

func moduleNames() string {
	names := make([]string, len(Modules))
	for i, m := range Modules {
		names[i] = string(m)
	}
	return strings.Join(names, ", ")
}

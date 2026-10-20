// Command api is the mojang-yggdrasil-redirect reverse proxy.
package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/AlisaAkiron/mojang-yggdrasil-redirect/internal/server"
	"github.com/AlisaAkiron/mojang-yggdrasil-redirect/site"
)

func main() {
	log.SetFlags(0)

	modules, err := server.ParseModules(os.Getenv("ENABLE_MODULE"))
	if err != nil {
		log.Fatalf("mojang-yggdrasil-redirect: ENABLE_MODULE: %v", err)
	}
	if len(modules) == 0 {
		log.Fatal("mojang-yggdrasil-redirect: no modules enabled; set ENABLE_MODULE, e.g. yggdrasil,authlib-injector,prism-meta")
	}
	cfg := server.Config{
		Modules:            modules,
		ProxyToken:         os.Getenv("PROXY_TOKEN"),
		DisableLandingPage: slices.Contains([]string{"1", "true", "yes"}, strings.ToLower(os.Getenv("DISABLE_LANDING_PAGE"))),
		Site:               site.Dist(),
	}
	app, err := server.New(cfg)
	if err != nil {
		log.Fatalf("mojang-yggdrasil-redirect: %v", err)
	}

	addr := net.JoinHostPort(envOr("HOST", "0.0.0.0"), envOr("PORT", "8787"))
	srv := &http.Server{
		Addr:              addr,
		Handler:           app,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	var enabled []string
	for _, m := range server.Modules {
		if modules[m] {
			enabled = append(enabled, string(m))
		}
	}
	log.Printf("mojang-yggdrasil-redirect: enabled modules: %s", strings.Join(enabled, ", "))
	log.Printf("mojang-yggdrasil-redirect listening on http://%s", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

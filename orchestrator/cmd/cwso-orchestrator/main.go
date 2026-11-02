// Package main is the entry point for the CWSO orchestrator MCP server.
//
// Phase 1 (PoC): stdio + Streamable HTTP transports, baseline filesystem tools,
// JWT auth + Origin validation. Async dispatch and sandbox runners are stubbed
// until Phase 3/4.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/emage/cwso/orchestrator/internal/config"
	"github.com/emage/cwso/orchestrator/internal/logging"
	"github.com/emage/cwso/orchestrator/internal/server"
)

var version = "0.1.0-dev"

func main() {
	transport := flag.String("transport", "stdio", "transport: stdio | http")
	addr := flag.String("addr", "", "HTTP listen address (http transport only; default: CWSO_HTTP_ADDR, else "+config.DefaultHTTPAddr+")")
	configPath := flag.String("config", "", "optional config file path")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("cwso-orchestrator", version)
		return
	}

	cfg, err := loadConfig(*configPath, *transport, *addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(2)
	}

	log := logging.New(cfg.LogLevel)
	log.Info().Str("version", version).Str("transport", cfg.Transport).Msg("cwso-orchestrator starting")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv, err := server.New(cfg, log)
	if err != nil {
		log.Fatal().Err(err).Msg("server init failed")
	}

	if err := srv.Run(ctx); err != nil {
		log.Fatal().Err(err).Msg("server exited with error")
	}
	log.Info().Msg("cwso-orchestrator stopped cleanly")
}

// loadConfig reads the configuration, applies the command-line overrides and
// re-runs the HTTP security checks on the result, so that `-transport http`
// meets the same JWT secret and Origin checks as CWSO_TRANSPORT=http
// (SEC-T695-03, SEC-T695-07). An empty addr keeps CWSO_HTTP_ADDR or the
// loopback default (SEC-T695-10).
func loadConfig(path, transport, addr string) (*config.Config, error) {
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	cfg.Transport = transport
	if addr != "" {
		cfg.HTTPAddr = addr
	}
	if err := cfg.ValidateHTTPSecurity(); err != nil {
		return nil, err
	}
	return cfg, nil
}

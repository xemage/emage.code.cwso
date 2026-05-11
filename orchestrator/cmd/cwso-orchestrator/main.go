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
	addr := flag.String("addr", ":8080", "HTTP listen address (http transport only)")
	configPath := flag.String("config", "", "optional config file path")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("cwso-orchestrator", version)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(2)
	}
	cfg.Transport = *transport
	cfg.HTTPAddr = *addr

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

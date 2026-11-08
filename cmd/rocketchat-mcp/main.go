// Command rocketchat-mcp serves Rocket.Chat tools over the Model Context
// Protocol, using stdio or streamable HTTP.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"forgejo.srv.hnatekmar.dev/hnatekmar/rocketchat-mcp/internal/config"
	"forgejo.srv.hnatekmar.dev/hnatekmar/rocketchat-mcp/internal/mcpserver"
	"forgejo.srv.hnatekmar.dev/hnatekmar/rocketchat-mcp/internal/rocketchat"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	flags := flag.NewFlagSet("rocketchat-mcp", flag.ContinueOnError)
	var (
		transport   = flags.String("transport", "stdio", `transport to use: "stdio" or "http"`)
		addr        = flags.String("addr", ":8080", "listen address for -transport=http")
		logLevel    = flags.String("log-level", "info", "log level: debug, info, warn, or error")
		showVersion = flags.Bool("version", false, "print the version and exit")
	)
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "Usage: %s [flags]\n\n", flags.Name())
		flags.PrintDefaults()
	}
	if err := flags.Parse(os.Args[1:]); err != nil {
		return err
	}
	if *showVersion {
		fmt.Println(version)
		return nil
	}

	level, err := parseLogLevel(*logLevel)
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	client, err := rocketchat.New(cfg.RocketChatURL, cfg.UserID, cfg.AuthToken)
	if err != nil {
		return err
	}

	server := mcpserver.New(client, mcpserver.Options{
		Version: version,
		Logger:  logger,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch *transport {
	case "stdio":
		logger.Info("serving MCP over stdio", "version", version)
		err := server.Run(ctx, &mcp.StdioTransport{})
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	case "http":
		return serveHTTP(ctx, server, *addr, logger)
	default:
		return fmt.Errorf("unknown transport %q: use \"stdio\" or \"http\"", *transport)
	}
}

func parseLogLevel(value string) (slog.Level, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(value)); err != nil {
		return 0, fmt.Errorf("invalid -log-level %q: %w", value, err)
	}
	return level, nil
}

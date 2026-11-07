// Command nexus-chat-server runs a Nexus Chat instance: the HTTP API,
// WebSocket gateway, federation endpoints and web client.
//
// Configuration comes from environment variables (see internal/config). Read
// this file first, then application.go, which wires every dependency
// together explicitly (code standards §8.2: no dependency-injection
// framework, no global state).
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/aazam476/nexus-chat/apps/server/internal/config"
)

func main() {
	os.Exit(runMain(os.Getenv, os.Stdout, os.Stderr))
}

// runMain runs the server until SIGINT or SIGTERM and returns the process
// exit code. It is separate from main so tests can call it.
func runMain(getenv config.Getenv, logOutput io.Writer, errorOutput io.Writer) int {
	// Docker sends SIGTERM on `docker compose down`; Ctrl+C sends SIGINT.
	// Either one cancels ctx, which starts a graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, getenv, logOutput); err != nil {
		// If stderr itself is broken there is nowhere left to report to.
		_, _ = fmt.Fprintln(errorOutput, "nexus-chat-server:", err)
		return 1
	}
	return 0
}

// run starts the server and blocks until ctx is cancelled or serving fails.
// It takes its environment and log output as parameters so tests can run
// the real startup path.
func run(ctx context.Context, getenv config.Getenv, logOutput io.Writer) error {
	configuration, err := config.Load(getenv)
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(logOutput, &slog.HandlerOptions{Level: configuration.LogLevel}))

	app, err := newApplication(ctx, configuration, logger)
	if err != nil {
		return err
	}
	defer app.close()

	sockets, err := listen(ctx, configuration)
	if err != nil {
		return err
	}
	logger.InfoContext(ctx, "serving",
		slog.String("http", sockets.plain.Addr().String()),
		slog.String("https", sockets.secure.Addr().String()),
		slog.Bool("https_active", app.certificates.Active()))
	return app.serve(ctx, sockets)
}

// listen opens the HTTP and HTTPS ports. The HTTPS port is always open; it
// completes handshakes only once a valid certificate is installed, which
// is what lets an admin enable HTTPS without restarting.
func listen(ctx context.Context, configuration config.Config) (listeners, error) {
	var listenConfig net.ListenConfig
	plainAddress := net.JoinHostPort("", strconv.Itoa(configuration.HTTPPort))
	plain, err := listenConfig.Listen(ctx, "tcp", plainAddress)
	if err != nil {
		return listeners{}, fmt.Errorf("listening on %s: %w", plainAddress, err)
	}
	secureAddress := net.JoinHostPort("", strconv.Itoa(configuration.HTTPSPort))
	secure, err := listenConfig.Listen(ctx, "tcp", secureAddress)
	if err != nil {
		_ = plain.Close() // already failing; the listen error is the one to report
		return listeners{}, fmt.Errorf("listening on %s: %w", secureAddress, err)
	}
	return listeners{plain: plain, secure: secure}, nil
}

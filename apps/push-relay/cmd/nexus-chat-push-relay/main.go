// Command nexus-chat-push-relay forwards opaque push payloads from Nexus
// Chat instances to APNs and FCM (docs/push-notifications.md).
//
// In milestone M0 it only serves a health probe; the relay protocol is built
// in M8. Read this file first, then internal/relayconfig.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/aazam476/nexus-chat/apps/push-relay/internal/relayconfig"
	"github.com/aazam476/nexus-chat/apps/push-relay/internal/relayhealth"
)

const shutdownTimeout = 8 * time.Second

func main() {
	os.Exit(runMain(os.Getenv, os.Stdout, os.Stderr))
}

// runMain runs the relay until SIGINT or SIGTERM and returns the process
// exit code. It is separate from main so tests can call it.
func runMain(getenv relayconfig.Getenv, logOutput io.Writer, errorOutput io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, getenv, logOutput); err != nil {
		// If stderr itself is broken there is nowhere left to report to.
		_, _ = fmt.Fprintln(errorOutput, "nexus-chat-push-relay:", err)
		return 1
	}
	return 0
}

// run serves the relay until ctx is cancelled.
func run(ctx context.Context, getenv relayconfig.Getenv, logOutput io.Writer) error {
	config, err := relayconfig.Load(getenv)
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(logOutput, &slog.HandlerOptions{Level: config.LogLevel}))
	server := &http.Server{
		Handler:           relayhealth.NewHandler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(ctx, "tcp", net.JoinHostPort("", strconv.Itoa(config.Port)))
	if err != nil {
		return fmt.Errorf("listening on port %d: %w", config.Port, err)
	}
	logger.InfoContext(ctx, "serving HTTP", slog.String("address", listener.Addr().String()))
	return serve(ctx, server, listener)
}

func serve(ctx context.Context, server *http.Server, listener net.Listener) error {
	serveErrors := make(chan error, 1)
	// Ends when Serve returns: after Shutdown below, or if the listener fails.
	go func() { serveErrors <- server.Serve(listener) }()

	select {
	case err := <-serveErrors:
		return fmt.Errorf("serving HTTP: %w", err)
	case <-ctx.Done():
	}
	shutdownContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("shutting down: %w", err)
	}
	if err := <-serveErrors; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serving HTTP: %w", err)
	}
	return nil
}

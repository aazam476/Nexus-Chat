package main

import (
	"log/slog"
	"testing"

	"github.com/aazam476/nexus-chat/apps/server/internal/config"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/databasetest"
)

func testConfiguration(t *testing.T) config.Config {
	t.Helper()
	return config.Config{
		AdminEmail:    "admin@example.com",
		DataDirectory: t.TempDir(),
		DatabaseURL:   databasetest.NewDatabaseURL(t),
		HTTPPort:      config.DefaultHTTPPort,
		LogLevel:      slog.LevelInfo,
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

package config

import (
	"log/slog"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// environment returns a Getenv backed by a map, starting from the two
// variables a basic self hoster must set.
func environment(overrides map[string]string) Getenv {
	values := map[string]string{
		"ADMIN_EMAIL":       "admin@example.com",
		"POSTGRES_PASSWORD": "change-me",
	}
	for name, value := range overrides {
		values[name] = value
	}
	return func(name string) string { return values[name] }
}

func TestLoadUsesDefaultsForOptionalVariables(t *testing.T) {
	config, err := Load(environment(nil))

	require.NoError(t, err)
	assert.Equal(t, "admin@example.com", config.AdminEmail)
	assert.Empty(t, config.Domain)
	assert.Equal(t, DefaultHTTPPort, config.HTTPPort)
	assert.Equal(t, DefaultHTTPSPort, config.HTTPSPort)
	assert.Equal(t, DefaultDataDirectory, config.DataDirectory)
	assert.Equal(t, slog.LevelInfo, config.LogLevel)
	assert.Empty(t, config.TrustedProxies)
	assert.Equal(t, "postgres://nexuschat:change-me@postgres:5432/nexuschat?sslmode=disable", config.DatabaseURL)
}

func TestLoadReadsEveryOptionalVariable(t *testing.T) {
	config, err := Load(environment(map[string]string{
		"DOMAIN":            "chat.example.com",
		"HTTP_PORT":         "9000",
		"HTTPS_PORT":        "9443",
		"DATA_DIR":          "/srv/nexus",
		"TRUSTED_PROXIES":   "172.16.0.0/12",
		"LOG_LEVEL":         "debug",
		"POSTGRES_HOST":     "db.internal",
		"POSTGRES_PORT":     "6543",
		"POSTGRES_USER":     "chat",
		"POSTGRES_DB":       "chatdb",
		"POSTGRES_PASSWORD": "p@ss word",
	}))

	require.NoError(t, err)
	assert.Equal(t, "chat.example.com", config.Domain)
	assert.Equal(t, 9000, config.HTTPPort)
	assert.Equal(t, 9443, config.HTTPSPort)
	assert.Equal(t, "/srv/nexus", config.DataDirectory)
	assert.Equal(t, slog.LevelDebug, config.LogLevel)
	assert.True(t, config.TrustedProxies.Contains(netip.MustParseAddr("172.20.0.1")))
	// Special characters in the password must be escaped, not break the URL.
	assert.Equal(t, "postgres://chat:p%40ss%20word@db.internal:6543/chatdb?sslmode=disable", config.DatabaseURL)
}

func TestLoadPrefersExplicitDatabaseURL(t *testing.T) {
	config, err := Load(environment(map[string]string{
		"POSTGRES_PASSWORD": "",
		"DATABASE_URL":      "postgresql://user:secret@localhost:5432/test?sslmode=require",
	}))

	require.NoError(t, err)
	assert.Equal(t, "postgresql://user:secret@localhost:5432/test?sslmode=require", config.DatabaseURL)
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name      string
		overrides map[string]string
		wantError string
	}{
		{"missing admin email", map[string]string{"ADMIN_EMAIL": ""}, "ADMIN_EMAIL is required"},
		{"admin email with display name", map[string]string{"ADMIN_EMAIL": "Admin <admin@example.com>"}, "not a valid email address"},
		{"admin email without at sign", map[string]string{"ADMIN_EMAIL": "admin"}, "not a valid email address"},
		{"missing postgres password", map[string]string{"POSTGRES_PASSWORD": ""}, "POSTGRES_PASSWORD is required"},
		{"http port not a number", map[string]string{"HTTP_PORT": "eighty"}, "HTTP_PORT"},
		{"http port zero", map[string]string{"HTTP_PORT": "0"}, "HTTP_PORT"},
		{"https port too large", map[string]string{"HTTPS_PORT": "65536"}, "HTTPS_PORT"},
		{"postgres port invalid", map[string]string{"POSTGRES_PORT": "-1"}, "POSTGRES_PORT"},
		{"domain with scheme", map[string]string{"DOMAIN": "https://chat.example.com"}, "DOMAIN"},
		{"domain with port", map[string]string{"DOMAIN": "chat.example.com:8443"}, "DOMAIN"},
		{"domain with path", map[string]string{"DOMAIN": "chat.example.com/app"}, "DOMAIN"},
		{"domain without dot", map[string]string{"DOMAIN": "localhost"}, "DOMAIN"},
		{"bad trusted proxy", map[string]string{"TRUSTED_PROXIES": "10.0.0.0/8, nope"}, "TRUSTED_PROXIES"},
		{"bad log level", map[string]string{"LOG_LEVEL": "loud"}, "LOG_LEVEL"},
		{"database url wrong scheme", map[string]string{"DATABASE_URL": "mysql://u:p@host/db"}, "DATABASE_URL"},
		{"database url without host", map[string]string{"DATABASE_URL": "postgres:///db"}, "DATABASE_URL"},
		{"database url unparsable", map[string]string{"DATABASE_URL": "postgres://u:p@[::1/db"}, "DATABASE_URL"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Load(environment(test.overrides))

			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantError)
		})
	}
}

func TestLoadReportsAllProblemsAtOnce(t *testing.T) {
	_, err := Load(environment(map[string]string{"ADMIN_EMAIL": "", "HTTP_PORT": "x"}))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ADMIN_EMAIL")
	assert.Contains(t, err.Error(), "HTTP_PORT")
}

func TestLoadNeverEchoesDatabasePassword(t *testing.T) {
	_, err := Load(environment(map[string]string{"DATABASE_URL": "mysql://user:hunter2@host/db"}))

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "hunter2")
}

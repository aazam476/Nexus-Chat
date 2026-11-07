package relayconfig

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func environment(values map[string]string) Getenv {
	return func(name string) string { return values[name] }
}

func TestLoadDefaults(t *testing.T) {
	config, err := Load(environment(nil))

	require.NoError(t, err)
	assert.Equal(t, Config{Port: DefaultPort, LogLevel: slog.LevelInfo}, config)
}

func TestLoadReadsVariables(t *testing.T) {
	config, err := Load(environment(map[string]string{"PORT": "9000", "LOG_LEVEL": "warn"}))

	require.NoError(t, err)
	assert.Equal(t, Config{Port: 9000, LogLevel: slog.LevelWarn}, config)
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	tests := map[string]map[string]string{
		"port not a number": {"PORT": "abc"},
		"port zero":         {"PORT": "0"},
		"port too large":    {"PORT": "70000"},
		"bad log level":     {"LOG_LEVEL": "chatty"},
	}
	for name, values := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Load(environment(values))

			assert.Error(t, err)
		})
	}
}

func TestLoadReportsAllProblems(t *testing.T) {
	_, err := Load(environment(map[string]string{"PORT": "x", "LOG_LEVEL": "y"}))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "PORT")
	assert.Contains(t, err.Error(), "LOG_LEVEL")
}

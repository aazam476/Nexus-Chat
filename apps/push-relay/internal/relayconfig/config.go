package relayconfig

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

// DefaultPort is the relay's listening port when PORT is not set.
const DefaultPort = 8090

// Config holds the relay's boot settings.
type Config struct {
	// Port is the plain-HTTP listening port. TLS is terminated by the
	// hosting platform's load balancer (docs/push-notifications.md §4).
	Port int
	// LogLevel controls log verbosity. Logs never contain payloads, tokens
	// or handles at any level.
	LogLevel slog.Level
}

// Getenv looks up an environment variable; os.Getenv satisfies it.
type Getenv func(name string) string

// Load reads and validates the configuration, reporting every problem.
func Load(getenv Getenv) (Config, error) {
	var problems []error
	config := Config{Port: DefaultPort, LogLevel: slog.LevelInfo}

	if value := strings.TrimSpace(getenv("PORT")); value != "" {
		port, err := strconv.Atoi(value)
		if err != nil || port < 1 || port > 65535 {
			problems = append(problems, fmt.Errorf("PORT %q must be a number between 1 and 65535", value))
		}
		config.Port = port
	}
	if value := strings.TrimSpace(getenv("LOG_LEVEL")); value != "" {
		if err := config.LogLevel.UnmarshalText([]byte(value)); err != nil {
			problems = append(problems, fmt.Errorf("LOG_LEVEL %q must be debug, info, warn or error", value))
		}
	}
	if len(problems) > 0 {
		return Config{}, fmt.Errorf("invalid configuration: %w", errors.Join(problems...))
	}
	return config, nil
}

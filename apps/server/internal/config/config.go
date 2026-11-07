package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"net/url"
	"strings"
)

// Default values used when an optional variable is not set.
const (
	DefaultHTTPPort         = 8080
	DefaultHTTPSPort        = 8443
	DefaultDataDirectory    = "/data"
	DefaultPostgresHost     = "postgres"
	DefaultPostgresPort     = 5432
	DefaultPostgresUser     = "nexuschat"
	DefaultPostgresDatabase = "nexuschat"
)

// Config holds every setting the server needs before it can reach the
// database. It is created once in main and passed to constructors.
type Config struct {
	// AdminEmail receives operational warnings such as certificate expiry.
	AdminEmail string
	// Domain is the public domain, for example "chat.example.com". It may be
	// empty until the admin sets it in the setup wizard.
	Domain string
	// HTTPPort is the plain-HTTP listening port.
	HTTPPort int
	// HTTPSPort is used only after an admin uploads a certificate.
	HTTPSPort int
	// DataDirectory holds generated secrets and, by default, uploaded files.
	DataDirectory string
	// DatabaseURL is a PostgreSQL connection string.
	DatabaseURL string
	// TrustedProxies lists the networks whose X-Forwarded-* headers we trust.
	TrustedProxies TrustedProxies
	// LogLevel controls log verbosity.
	LogLevel slog.Level
}

// Getenv looks up an environment variable. os.Getenv satisfies it; tests
// pass a map lookup instead so they never touch the real environment.
type Getenv func(name string) string

// Load reads and validates the configuration. It reports every problem at
// once so an admin can fix their .env file in a single pass.
func Load(getenv Getenv) (Config, error) {
	var problems []error
	collect := func(err error) {
		if err != nil {
			problems = append(problems, err)
		}
	}

	config := Config{
		Domain:        strings.TrimSpace(getenv("DOMAIN")),
		DataDirectory: stringOrDefault(getenv("DATA_DIR"), DefaultDataDirectory),
	}
	var err error

	config.AdminEmail, err = parseAdminEmail(getenv("ADMIN_EMAIL"))
	collect(err)
	config.HTTPPort, err = parsePort("HTTP_PORT", getenv("HTTP_PORT"), DefaultHTTPPort)
	collect(err)
	config.HTTPSPort, err = parsePort("HTTPS_PORT", getenv("HTTPS_PORT"), DefaultHTTPSPort)
	collect(err)
	config.TrustedProxies, err = ParseTrustedProxies(getenv("TRUSTED_PROXIES"))
	collect(err)
	config.LogLevel, err = parseLogLevel(getenv("LOG_LEVEL"))
	collect(err)
	config.DatabaseURL, err = databaseURL(getenv)
	collect(err)
	collect(ValidateDomain(config.Domain))

	if len(problems) > 0 {
		return Config{}, fmt.Errorf("invalid configuration: %w", errors.Join(problems...))
	}
	return config, nil
}

func parseAdminEmail(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("ADMIN_EMAIL is required")
	}
	address, err := mail.ParseAddress(value)
	// ParseAddress accepts "Name <a@b>"; we want the bare address only.
	if err != nil || address.Address != value {
		return "", fmt.Errorf("ADMIN_EMAIL %q is not a valid email address", value)
	}
	return value, nil
}

func parseLogLevel(value string) (slog.Level, error) {
	if strings.TrimSpace(value) == "" {
		return slog.LevelInfo, nil
	}
	var level slog.Level
	if err := level.UnmarshalText([]byte(value)); err != nil {
		return slog.LevelInfo, fmt.Errorf("LOG_LEVEL %q must be debug, info, warn or error", value)
	}
	return level, nil
}

// ValidateDomain accepts an empty domain (it can be set later) or a bare
// host name. A scheme, path or port is a common copy-paste mistake.
func ValidateDomain(domain string) error {
	if domain == "" {
		return nil
	}
	parsed, err := url.Parse("https://" + domain)
	if err != nil || parsed.Host != domain || parsed.Port() != "" || !strings.Contains(domain, ".") {
		return fmt.Errorf("DOMAIN %q must be a host name such as chat.example.com, without https:// or a port", domain)
	}
	return nil
}

func stringOrDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

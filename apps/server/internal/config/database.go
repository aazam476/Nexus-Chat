package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// databaseURL builds the PostgreSQL connection string.
//
// Self hosters set only POSTGRES_PASSWORD; the compose file supplies the
// host, user and database name. Developers and tests can instead set
// DATABASE_URL to a full connection string, which takes precedence.
func databaseURL(getenv Getenv) (string, error) {
	if explicit := strings.TrimSpace(getenv("DATABASE_URL")); explicit != "" {
		return validateDatabaseURL(explicit)
	}

	password := getenv("POSTGRES_PASSWORD")
	if password == "" {
		return "", errors.New("POSTGRES_PASSWORD is required")
	}
	port, err := parsePort("POSTGRES_PORT", getenv("POSTGRES_PORT"), DefaultPostgresPort)
	if err != nil {
		return "", err
	}

	host := stringOrDefault(getenv("POSTGRES_HOST"), DefaultPostgresHost)
	connection := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(stringOrDefault(getenv("POSTGRES_USER"), DefaultPostgresUser), password),
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
		Path:   "/" + stringOrDefault(getenv("POSTGRES_DB"), DefaultPostgresDatabase),
	}
	// The database runs on the private compose network, so TLS is off by
	// default. An admin pointing at an external database sets DATABASE_URL
	// with the sslmode they need.
	connection.RawQuery = "sslmode=disable"
	return connection.String(), nil
}

func validateDatabaseURL(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Host == "" {
		// Never echo the value: it usually contains a password.
		return "", errors.New("DATABASE_URL must be a postgres:// connection string with a host")
	}
	return value, nil
}

// parsePort returns fallback for an empty value and rejects anything that is
// not a TCP port number.
func parsePort(name string, value string, fallback int) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("%s %q must be a number between 1 and 65535", name, value)
	}
	return port, nil
}

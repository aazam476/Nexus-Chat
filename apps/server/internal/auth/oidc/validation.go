package oidc

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
)

// Field limits for provider settings.
const (
	maximumProviderName = 64
	maximumClientID     = 256
	maximumClientSecret = 1024
	maximumIssuer       = 512
	maximumScopes       = 10
)

var (
	providerIDPattern = regexp.MustCompile(`^[a-z0-9-]{2,32}$`)
	// scopePattern is RFC 6749 §3.3's scope-token, at most 64 characters.
	scopePattern = regexp.MustCompile(`^[\x21\x23-\x5B\x5D-\x7E]{1,64}$`)

	errProviderID   = errors.New("must be 2–32 lower-case letters, digits or dashes")
	errDisplayName  = fmt.Errorf("must be 1–%d characters", maximumProviderName)
	errClientID     = fmt.Errorf("must be 1–%d characters without spaces", maximumClientID)
	errClientSecret = fmt.Errorf("must be at most %d characters", maximumClientSecret)
	errScopes       = fmt.Errorf("must be at most %d scopes, each 1–64 printable characters without quotes or backslashes", maximumScopes)
	errIssuer       = errors.New("must be an https:// address without a query or fragment (http:// only for localhost)")
)

// validate checks input and returns it cleaned up, then confirms the
// issuer publishes an OIDC configuration. checkID is false for updates,
// where the ID comes from the URL.
func (service *Providers) validate(ctx context.Context, input ProviderInput, checkID bool) (ProviderInput, error) {
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Issuer = strings.TrimSpace(input.Issuer)
	input.ClientID = strings.TrimSpace(input.ClientID)
	input.Scopes = normalizeScopes(input.Scopes)

	problems := checkFields(input, checkID)
	if err := problems.Err(); err != nil {
		return ProviderInput{}, err
	}
	if err := service.checker.Check(ctx, input.Issuer); err != nil {
		problems.Add("issuer", fmt.Errorf("could not load the provider's OpenID configuration: %w", err))
		return ProviderInput{}, problems.Err()
	}
	return input, nil
}

// checkFields records every invalid field of input.
func checkFields(input ProviderInput, checkID bool) *validation.Errors {
	var problems validation.Errors
	if checkID && !providerIDPattern.MatchString(input.ID) {
		problems.Add("id", errProviderID)
	}
	if count := utf8.RuneCountInString(input.DisplayName); count < 1 || count > maximumProviderName {
		problems.Add("displayName", errDisplayName)
	}
	problems.Add("issuer", checkIssuer(input.Issuer))
	if !validClientID(input.ClientID) {
		problems.Add("clientId", errClientID)
	}
	if input.ClientSecret != nil && len(*input.ClientSecret) > maximumClientSecret {
		problems.Add("clientSecret", errClientSecret)
	}
	if !validScopes(input.Scopes) {
		problems.Add("scopes", errScopes)
	}
	return &problems
}

func validClientID(clientID string) bool {
	return clientID != "" && len(clientID) <= maximumClientID && !strings.ContainsFunc(clientID, unicode.IsSpace)
}

// checkIssuer accepts HTTPS issuers, and plain HTTP only on loopback
// addresses for local testing. Credentials and tokens travel to this
// address, so it must be encrypted everywhere else.
func checkIssuer(issuer string) error {
	parsed, err := url.Parse(issuer)
	if err != nil || len(issuer) > maximumIssuer || parsed.Host == "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return errIssuer
	}
	switch parsed.Scheme {
	case "https":
		return nil
	case "http":
		if isLoopback(parsed.Hostname()) {
			return nil
		}
	}
	return errIssuer
}

func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// normalizeScopes trims and deduplicates scopes, uses the defaults when
// none are given, and always includes "openid", without which a provider
// returns no ID token.
func normalizeScopes(scopes []string) []string {
	if len(scopes) == 0 {
		// The claims an account needs.
		return []string{"openid", "email", "profile"}
	}
	result := []string{"openid"}
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if !slices.Contains(result, scope) {
			result = append(result, scope)
		}
	}
	return result
}

func validScopes(scopes []string) bool {
	if len(scopes) > maximumScopes {
		return false
	}
	for _, scope := range scopes {
		if !scopePattern.MatchString(scope) {
			return false
		}
	}
	return true
}

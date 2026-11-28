package instanceconfig

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"
)

// Registration modes.
const (
	RegistrationOpen       = "open"
	RegistrationInviteOnly = "invite_only"
	RegistrationClosed     = "closed"
)

// ErrInvalidRegistrationMode means a mode other than open, invite_only or closed.
var ErrInvalidRegistrationMode = errors.New(`registration is "open", "invite_only" or "closed"`)

// SetRegistrationMode changes who may sign up.
func (service *Service) SetRegistrationMode(ctx context.Context, mode string) error {
	if !slices.Contains([]string{RegistrationOpen, RegistrationInviteOnly, RegistrationClosed}, mode) {
		return ErrInvalidRegistrationMode
	}
	return service.Save(ctx, KeyRegistrationMode, mode)
}

// Federation modes (docs/federation-protocol.md): every instance but the
// blocked ones, only the allowed ones, or none.
const (
	FederationOpen      = "open"
	FederationAllowlist = "allowlist"
	FederationClosed    = "closed"
)

// MaximumFederationDomains is how many domains each list holds at most.
const MaximumFederationDomains = 1000

// ErrInvalidFederation means an unknown mode, a list too long, or something
// in a list that is not a domain.
var ErrInvalidFederation = errors.New(`federation is "open", "allowlist" or "closed", with at most 1000 domains in each list`)

// FederationPolicy is which instances this one talks to. Blocked domains
// are refused in every mode; Allowed matters only in allowlist mode.
type FederationPolicy struct {
	Mode    string   `json:"mode"`
	Allowed []string `json:"allowed"`
	Blocked []string `json:"blocked"`
}

// domainPattern is a lowercase DNS name with at least one dot.
var domainPattern = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]([a-z0-9-]{0,61}[a-z0-9])?$`)

// Federation returns the federation policy; open with nothing listed until the admin changes it.
func (service *Service) Federation(ctx context.Context) (FederationPolicy, error) {
	policy := FederationPolicy{Mode: FederationOpen, Allowed: []string{}, Blocked: []string{}}
	_, err := service.Load(ctx, KeyFederation, &policy)
	return policy, err
}

// SetFederation changes the federation policy. Domains are kept
// lowercase, without repeats, sorted.
func (service *Service) SetFederation(ctx context.Context, policy FederationPolicy) error {
	if !slices.Contains([]string{FederationOpen, FederationAllowlist, FederationClosed}, policy.Mode) {
		return ErrInvalidFederation
	}
	allowed, okAllowed := domains(policy.Allowed)
	blocked, okBlocked := domains(policy.Blocked)
	if !okAllowed || !okBlocked {
		return ErrInvalidFederation
	}
	return service.Save(ctx, KeyFederation, FederationPolicy{Mode: policy.Mode, Allowed: allowed, Blocked: blocked})
}

// domains cleans a list of domains, or reports that it is not one.
func domains(list []string) ([]string, bool) {
	if len(list) > MaximumFederationDomains {
		return nil, false
	}
	clean := make([]string, 0, len(list))
	for _, domain := range list {
		domain = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
		if len(domain) > 253 || !domainPattern.MatchString(domain) {
			return nil, false
		}
		clean = append(clean, domain)
	}
	slices.Sort(clean)
	return slices.Compact(clean), true
}

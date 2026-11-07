package health

import (
	"context"
	"time"
)

// checkTimeout bounds each dependency check, so a hung database makes the
// probe fail quickly instead of hanging until Docker's own timeout.
const checkTimeout = 2 * time.Second

// Checker is one dependency the server needs, such as the database.
// *pgxpool.Pool satisfies it through its Ping method.
type Checker interface {
	Ping(ctx context.Context) error
}

// NamedChecker pairs a dependency with the name reported by the probe.
type NamedChecker struct {
	Name    string
	Checker Checker
}

// Result is the outcome of one dependency check.
type Result struct {
	Name string
	OK   bool
}

// Service runs dependency checks.
type Service struct {
	checkers []NamedChecker
}

// NewService returns a Service that checks the given dependencies, in order.
func NewService(checkers []NamedChecker) *Service {
	return &Service{checkers: checkers}
}

// Readiness checks every dependency and reports whether all of them are
// reachable, along with each individual result.
func (service *Service) Readiness(ctx context.Context) (bool, []Result) {
	allOK := true
	results := make([]Result, 0, len(service.checkers))
	for _, named := range service.checkers {
		checkContext, cancel := context.WithTimeout(ctx, checkTimeout)
		err := named.Checker.Ping(checkContext)
		cancel()
		results = append(results, Result{Name: named.Name, OK: err == nil})
		if err != nil {
			allOK = false
		}
	}
	return allOK, results
}

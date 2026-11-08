package realtime

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
)

// sweepInterval is how often old sessions are forgotten and expired custom
// statuses removed; the protocol promises expiry within one minute.
const sweepInterval = 30 * time.Second

type statusStore interface {
	ClearExpiredCustomStatuses(ctx context.Context, now time.Time) ([]uuid.UUID, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (store.User, error)
}

// Sweeper does the hub's periodic housekeeping.
type Sweeper struct {
	hub    *Hub
	store  statusStore
	logger *slog.Logger
}

// NewSweeper returns a Sweeper for hub.
func NewSweeper(hub *Hub, statuses statusStore, logger *slog.Logger) *Sweeper {
	return &Sweeper{hub: hub, store: statuses, logger: logger}
}

// Run sweeps every interval until ctx is cancelled. The caller starts it
// in a goroutine, which stops when ctx is cancelled at shutdown.
func (sweeper *Sweeper) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sweeper.Sweep(ctx)
		}
	}
}

// Sweep runs one round: forget sessions past their resume window, and
// remove custom statuses that have expired, telling their owners' devices.
func (sweeper *Sweeper) Sweep(ctx context.Context) {
	sweeper.hub.sweep()
	cleared, err := sweeper.store.ClearExpiredCustomStatuses(ctx, sweeper.hub.clock.Now())
	if err != nil {
		sweeper.logger.WarnContext(ctx, "could not clear expired custom statuses", slog.Any("error", err))
		return
	}
	for _, userID := range cleared {
		user, err := sweeper.store.GetUserByID(ctx, userID)
		if err != nil {
			sweeper.logger.WarnContext(ctx, "could not reload user after clearing status", slog.Any("error", err))
			continue
		}
		sweeper.hub.UserChanged(ctx, user)
	}
}

// SweepInterval is the interval Run is meant to use in production.
func SweepInterval() time.Duration {
	return sweepInterval
}

package keypackages

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/keypackages/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
)

// Limits on what a device stores (docs/e2ee-design.md §3.2).
const (
	// MaximumOneTime is the most one-time packages a device may hold.
	MaximumOneTime = 200
	// MaximumBatch is the most packages in one upload.
	MaximumBatch = 100
	// RefillBelow is when the server tells a device to upload more.
	RefillBelow = 20
	// maximumSize is the largest key package accepted, in bytes.
	maximumSize = 16 * 1024
)

// header is how every key package we accept starts: MLSMessage version
// mls10 (0x0001), wire format mls_key_package (0x0005), then the
// KeyPackage's own version mls10 and ciphersuite 0x0001 (RFC 9420 §6 and
// §10; docs/e2ee-design.md §1).
var header = []byte{0x00, 0x01, 0x00, 0x05, 0x00, 0x01, 0x00, 0x01} //nolint:gochecknoglobals // constant bytes

// Errors returned to the caller.
var (
	ErrInvalidPackage = errors.New("not an MLS key package for ciphersuite 0x0001, or larger than 16 KiB")
	ErrEmptyUpload    = errors.New("an upload needs at least one key package")
	ErrBatchTooLarge  = fmt.Errorf("an upload may hold at most %d key packages", MaximumBatch)
	ErrStockFull      = fmt.Errorf("a device may store at most %d one-time key packages", MaximumOneTime)
	ErrRateLimited    = errors.New("too many key-package claims; wait a minute")
)

// Stock is a device's key packages, as the server reports it.
type Stock struct {
	OneTime       int
	HasLastResort bool
	// Refill tells the device to upload more.
	Refill bool
}

// Claimed is the key package handed out for one device.
type Claimed struct {
	DeviceID   uuid.UUID
	KeyPackage []byte
	LastResort bool
}

type stockStore interface {
	Count(ctx context.Context, sessionID uuid.UUID) (store.CountKeyPackagesRow, error)
	Add(ctx context.Context, upload store.Upload) (store.CountKeyPackagesRow, error)
	Claim(ctx context.Context, userID uuid.UUID) ([]store.Claimed, error)
}

type attemptLimiter interface {
	Allow(key string) bool
}

// Service is the key-package directory.
type Service struct {
	store   stockStore
	limiter attemptLimiter
	clock   clock.Clock
}

// NewService returns a Service. limiter limits claims per claiming user, so
// nobody can drain another user's packages.
func NewService(stock stockStore, limiter attemptLimiter, clock clock.Clock) *Service {
	return &Service{store: stock, limiter: limiter, clock: clock}
}

func valid(keyPackage []byte) bool {
	return len(keyPackage) <= maximumSize && bytes.HasPrefix(keyPackage, header) && len(keyPackage) > len(header)
}

func stockFrom(count store.CountKeyPackagesRow) Stock {
	return Stock{OneTime: int(count.OneTime), HasLastResort: count.HasLastResort, Refill: count.OneTime < RefillBelow}
}

// Upload stores key packages for the calling device; lastResort, when not
// nil, replaces its last-resort package.
func (service *Service) Upload(ctx context.Context, device session.Principal, oneTime [][]byte, lastResort []byte) (Stock, error) {
	if len(oneTime) == 0 && lastResort == nil {
		return Stock{}, ErrEmptyUpload
	}
	if len(oneTime) > MaximumBatch {
		return Stock{}, ErrBatchTooLarge
	}
	for _, keyPackage := range oneTime {
		if !valid(keyPackage) {
			return Stock{}, ErrInvalidPackage
		}
	}
	if lastResort != nil && !valid(lastResort) {
		return Stock{}, ErrInvalidPackage
	}
	count, err := service.store.Add(ctx, store.Upload{
		SessionID: device.SessionID, UserID: device.UserID, OneTime: oneTime, LastResort: lastResort,
		Now: service.clock.Now(), MaximumOneTime: MaximumOneTime,
	})
	if errors.Is(err, store.ErrStockFull) {
		return Stock{}, ErrStockFull
	}
	if err != nil {
		return Stock{}, err
	}
	return stockFrom(count), nil
}

// Count reports the calling device's stock.
func (service *Service) Count(ctx context.Context, device session.Principal) (Stock, error) {
	count, err := service.store.Count(ctx, device.SessionID)
	if err != nil {
		return Stock{}, err
	}
	return stockFrom(count), nil
}

// Claim hands out one key package for each device of userID. Devices
// without packages are left out; an empty list means none can be added.
func (service *Service) Claim(ctx context.Context, claimer session.Principal, userID uuid.UUID) ([]Claimed, error) {
	if !service.limiter.Allow(claimer.UserID.String()) {
		return nil, ErrRateLimited
	}
	rows, err := service.store.Claim(ctx, userID)
	if err != nil {
		return nil, err
	}
	claimed := make([]Claimed, 0, len(rows))
	for _, row := range rows {
		claimed = append(claimed, Claimed{DeviceID: row.SessionID, KeyPackage: row.Data, LastResort: row.LastResort})
	}
	return claimed, nil
}

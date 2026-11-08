package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aazam476/nexus-chat/apps/server/internal/platform/database"
)

// ErrStockFull means the upload would take the device past its limit.
var ErrStockFull = errors.New("too many key packages stored for this device")

// Upload is one device's batch of new key packages.
type Upload struct {
	SessionID uuid.UUID
	UserID    uuid.UUID
	OneTime   [][]byte
	// LastResort replaces the device's last-resort package when not nil.
	LastResort []byte
	Now        time.Time
	// MaximumOneTime is how many one-time packages a device may hold.
	MaximumOneTime int64
}

// Claimed is the package handed out for one device.
type Claimed struct {
	SessionID  uuid.UUID
	Data       []byte
	LastResort bool
}

// Stock stores and hands out key packages, each change in one transaction.
type Stock struct {
	pool *pgxpool.Pool
}

// NewStock returns a Stock using pool.
func NewStock(pool *pgxpool.Pool) *Stock {
	return &Stock{pool: pool}
}

// Count reports a device's stock.
func (stock *Stock) Count(ctx context.Context, sessionID uuid.UUID) (CountKeyPackagesRow, error) {
	count, err := New(stock.pool).CountKeyPackages(ctx, sessionID)
	if err != nil {
		return CountKeyPackagesRow{}, fmt.Errorf("counting key packages: %w", err)
	}
	return count, nil
}

// Add stores the upload, all of it or none, and returns the new count.
func (stock *Stock) Add(ctx context.Context, upload Upload) (CountKeyPackagesRow, error) {
	var count CountKeyPackagesRow
	err := database.InTransaction(ctx, stock.pool, func(transaction pgx.Tx) error {
		queries := New(transaction)
		current, err := queries.CountKeyPackages(ctx, upload.SessionID)
		if err != nil {
			return fmt.Errorf("counting key packages: %w", err)
		}
		if current.OneTime+int64(len(upload.OneTime)) > upload.MaximumOneTime {
			return ErrStockFull
		}
		for _, data := range upload.OneTime {
			params := InsertKeyPackageParams{SessionID: upload.SessionID, UserID: upload.UserID, Data: data, CreatedAt: upload.Now}
			if err := queries.InsertKeyPackage(ctx, params); err != nil {
				return fmt.Errorf("saving key package: %w", err)
			}
		}
		if upload.LastResort != nil {
			params := ReplaceLastResortParams{SessionID: upload.SessionID, UserID: upload.UserID, Data: upload.LastResort, CreatedAt: upload.Now}
			if err := queries.ReplaceLastResort(ctx, params); err != nil {
				return fmt.Errorf("saving last-resort key package: %w", err)
			}
		}
		count, err = queries.CountKeyPackages(ctx, upload.SessionID)
		return err
	})
	return count, err
}

// Claim hands out one package for every device of userID: a one-time
// package, used up, or the device's last-resort package when it has none.
func (stock *Stock) Claim(ctx context.Context, userID uuid.UUID) ([]Claimed, error) {
	var claimed []Claimed
	err := database.InTransaction(ctx, stock.pool, func(transaction pgx.Tx) error {
		queries := New(transaction)
		oneTime, err := queries.ClaimOneTime(ctx, userID)
		if err != nil {
			return fmt.Errorf("claiming key packages: %w", err)
		}
		served := make([]uuid.UUID, 0, len(oneTime))
		for _, row := range oneTime {
			claimed = append(claimed, Claimed{SessionID: row.SessionID, Data: row.Data})
			served = append(served, row.SessionID)
		}
		lastResort, err := queries.LastResortPackages(ctx, LastResortPackagesParams{UserID: userID, Served: served})
		if err != nil {
			return fmt.Errorf("reading last-resort key packages: %w", err)
		}
		for _, row := range lastResort {
			claimed = append(claimed, Claimed{SessionID: row.SessionID, Data: row.Data, LastResort: true})
		}
		return nil
	})
	return claimed, err
}

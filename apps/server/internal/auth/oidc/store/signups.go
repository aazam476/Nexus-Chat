package store

import (
	"context"

	"github.com/jackc/pgx/v5"

	registrationstore "github.com/aazam476/nexus-chat/apps/server/internal/registration/store"
)

// Signups creates accounts together with their first provider identity.
type Signups struct {
	registrar *registrationstore.Registrar
}

// NewSignups returns Signups saving through registrar.
func NewSignups(registrar *registrationstore.Registrar) *Signups {
	return &Signups{registrar: registrar}
}

// CreateWithIdentity uses the invite (if any) and inserts the user and the
// identity in one transaction, so a failure leaves neither behind.
func (signups *Signups) CreateWithIdentity(ctx context.Context, account registrationstore.NewAccount, identity InsertIdentityParams) error {
	account.AlsoInsert = func(ctx context.Context, transaction pgx.Tx) error {
		return New(transaction).InsertIdentity(ctx, identity)
	}
	return signups.registrar.CreateAccount(ctx, account)
}

package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// uniqueViolation is PostgreSQL's error code for a duplicate key.
const uniqueViolation = "23505"

// Names of the unique constraints callers tell apart.
const (
	ProviderKey        = "oidc_providers_pkey"
	IdentityKey        = "oidc_identities_pkey"
	IdentityPerUserKey = "oidc_identities_provider_id_user_id_key"
)

// IsDuplicate reports whether err is a unique violation of constraint.
func IsDuplicate(err error, constraint string) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == uniqueViolation && postgresError.ConstraintName == constraint
}

package store

// sqlc generates every store in one run; accounts/store/generate.go also
// triggers it, which is harmless.
//go:generate go tool sqlc generate --file ../../../sqlc.yaml

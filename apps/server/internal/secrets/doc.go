// Package secrets creates the instance's keys on first boot and loads them on
// every later boot.
//
// Self hosters never generate keys by hand (spec §3.4). On first start the
// server writes each secret to its own file under DATA_DIR/secrets with
// owner-only permissions. Later starts read the same files, so tokens and
// federation identity survive restarts and upgrades.
//
// Start reading at [LoadOrCreate]. Each secret's file format is in files.go.
package secrets

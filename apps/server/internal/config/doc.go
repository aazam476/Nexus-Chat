// Package config reads the server's boot settings from environment variables.
//
// Only a handful of values come from the environment (the `.env` file a self
// hoster edits). Everything else is stored in the database and edited in the
// Admin panel. Start reading at [Load], which returns a validated [Config].
package config

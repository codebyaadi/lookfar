// Package sqlfiles embeds the SQL migrations so the binary can run them.
package sqlfiles

import "embed"

// Schema holds the goose migrations in schema/.
//
//go:embed schema/*.sql
var Schema embed.FS

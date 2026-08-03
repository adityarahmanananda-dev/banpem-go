// Package migrations meng-embed skema SQL untuk migrasi startup & import.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

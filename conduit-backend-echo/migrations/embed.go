// Package migrations embeds the versioned SQL migrations into the binary.
package migrations

import "embed"

// FS contains every *.sql migration file (golang-migrate naming scheme).
//
//go:embed *.sql
var FS embed.FS

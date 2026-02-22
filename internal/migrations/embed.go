package migrations

import "embed"

// FS contains the golang-migrate compatible *.sql files.
//
//go:embed *.sql
var FS embed.FS

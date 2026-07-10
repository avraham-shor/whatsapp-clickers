// Package migrations embeds all goose migration files so the single
// deployed binary needs no .sql files on disk.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

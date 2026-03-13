// db/migrations/embed.go
package migrations

import "embed"

//go:embed *.sql
var MigrationFS embed.FS

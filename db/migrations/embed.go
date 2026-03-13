// Package migrations костыль для доступа к файлам миграции из модулей
package migrations

import "embed"

// MigrationFS запакованные файлы миграции
//
//go:embed *.sql
var MigrationFS embed.FS

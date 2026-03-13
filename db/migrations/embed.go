// Костыль для доступа к файлам миграции из модулей
package migrations

import "embed"

//go:embed *.sql
var MigrationFS embed.FS

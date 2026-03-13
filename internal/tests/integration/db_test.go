//go:build integration

package integration

import (
	"context"
	"database/sql"
	"log"
	"os"
	"testing"
	"time"

	"urlshortener/db/migrations"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	conn    *sql.DB
	connStr string
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	// Запускаем PostgreSQL контейнер
	pgContainer, err := postgres.Run(ctx,
		"postgres:16",
		postgres.WithDatabase("app"),
		postgres.WithUsername("app"),
		postgres.WithPassword("secret"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		log.Fatalf("start pg: %v", err)
	}
	defer func() { _ = pgContainer.Terminate(ctx) }()

	// Получаем connection string
	connStr, err = pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("get connection string: %v", err)
	}

	conn, err = sql.Open("pgx", connStr)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	// Проверяем подключение
	if err = conn.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	// Применяем миграции
	goose.SetBaseFS(migrations.MigrationFS)
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("goose dialect: %v", err)
	}

	if err := goose.Up(conn, "."); err != nil {
		log.Fatalf("goose up: %v", err)
	}

	os.Exit(m.Run())
}

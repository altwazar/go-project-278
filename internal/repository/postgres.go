package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"urlshortener/internal/db"
)

var (
	ErrNotFound      = errors.New("record not found")
	ErrAlreadyExists = errors.New("record already exists")
	ErrGenerateName  = errors.New("failed to generate unique short name")
)

type Repository struct {
	queries *db.Queries
	pool    *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Repository, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	queries := db.New(pool)

	return &Repository{
		queries: queries,
		pool:    pool,
	}, nil
}

func (r *Repository) Close() {
	if r.pool != nil {
		r.pool.Close()
	}
}

// GenerateShortName создает уникальное короткое имя
func (r *Repository) GenerateShortName(ctx context.Context) (string, error) {
	for i := 0; i < 5; i++ { // Пробуем 5 раз
		bytes := make([]byte, 3) // 6 символов в hex
		if _, err := rand.Read(bytes); err != nil {
			return "", fmt.Errorf("generate random: %w", err)
		}
		shortName := hex.EncodeToString(bytes)

		// Проверяем уникальность
		_, err := r.queries.GetLinkByShortName(ctx, shortName)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Если ссылка не найдена - имя свободно
				return shortName, nil
			}
			return "", fmt.Errorf("check short name: %w", err)
		}
	}
	return "", ErrGenerateName
}

// CreateLink создает новую ссылку
func (r *Repository) CreateLink(ctx context.Context, originalURL, shortName string) (*db.Link, error) {
	// Проверяем уникальность short_name
	_, err := r.queries.GetLinkByShortName(ctx, shortName)
	if err == nil {
		return nil, ErrAlreadyExists
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("check existing: %w", err)
	}

	link, err := r.queries.CreateLink(ctx, db.CreateLinkParams{
		OriginalUrl: originalURL,
		ShortName:   shortName,
	})
	if err != nil {
		return nil, fmt.Errorf("create link: %w", err)
	}

	return &link, nil
}

// GetLinkByID возвращает ссылку по ID
func (r *Repository) GetLinkByID(ctx context.Context, id int32) (*db.Link, error) {
	link, err := r.queries.GetLink(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get link: %w", err)
	}
	return &link, nil
}

// GetLinkByShortName возвращает ссылку по короткому имени
func (r *Repository) GetLinkByShortName(ctx context.Context, shortName string) (*db.Link, error) {
	link, err := r.queries.GetLinkByShortName(ctx, shortName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get link by short name: %w", err)
	}
	return &link, nil
}

// ListLinks возвращает все ссылки
func (r *Repository) ListLinks(ctx context.Context) ([]db.Link, error) {
	links, err := r.queries.ListLinks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list links: %w", err)
	}
	return links, nil
}

// UpdateLink обновляет ссылку
func (r *Repository) UpdateLink(ctx context.Context, id int32, originalURL, shortName *string) (*db.Link, error) {
	// Проверяем существование ссылки
	_, err := r.queries.GetLink(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("check existing: %w", err)
	}

	// Если обновляем short_name, проверяем уникальность
	if shortName != nil {
		existing, err := r.queries.GetLinkByShortName(ctx, *shortName)
		if err == nil && existing.ID != id {
			return nil, ErrAlreadyExists
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("check short name: %w", err)
		}
	}

	// Подготавливаем параметры для обновления с использованием pgtype.Text
	params := db.UpdateLinkParams{
		ID: id,
	}

	// Для OriginalUrl используем pgtype.Text
	if originalURL != nil {
		params.OriginalUrl = pgtype.Text{
			String: *originalURL,
			Valid:  true,
		}
	} else {
		params.OriginalUrl = pgtype.Text{Valid: false}
	}

	// Для ShortName используем pgtype.Text
	if shortName != nil {
		params.ShortName = pgtype.Text{
			String: *shortName,
			Valid:  true,
		}
	} else {
		params.ShortName = pgtype.Text{Valid: false}
	}

	link, err := r.queries.UpdateLink(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("update link: %w", err)
	}

	return &link, nil
}

// DeleteLink удаляет ссылку
func (r *Repository) DeleteLink(ctx context.Context, id int32) error {
	err := r.queries.DeleteLink(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("delete link: %w", err)
	}
	return nil
}

// WithTransaction выполняет функцию в транзакции
func (r *Repository) WithTransaction(ctx context.Context, fn func(*db.Queries) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := r.queries.WithTx(tx)

	if err := fn(qtx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

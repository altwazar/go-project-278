// Package repository - слой доступа к данным в базе PostreSQL и методы для работы с записями.
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
	// ErrNotFound ошибка при ненайденной записи
	ErrNotFound = errors.New("record not found")
	// ErrAlreadyExists ошибка обновления/добавления при существующей записи
	ErrAlreadyExists = errors.New("record already exists")
	// ErrGenerateName ошибка генерации уникального имени
	ErrGenerateName = errors.New("failed to generate unique short name")
)

// Repository объединяет методы взаимодействия с БД
type Repository struct {
	queries *db.Queries
	pool    *pgxpool.Pool
}

// New создаёт подключение к базе и возвращаем пул вместе с запросами
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

// SetQueries костыль для тестов, подсовывание запросов в новой транзакции
func (r *Repository) SetQueries(q *db.Queries) {
	r.queries = q
}

// Close закрывает пул
func (r *Repository) Close() {
	if r.pool != nil {
		r.pool.Close()
	}
}

// GenerateShortName создает уникальное короткое имя
func (r *Repository) GenerateShortName(ctx context.Context) (string, error) {
	for i := 0; i < 5; i++ { // Ленивая защита от коллизий
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

// ListLinks возвращает ВСЕ ссылки (без пагинации)
// Deprecated: Используйте ListLinksWithPagination для пагинации
func (r *Repository) ListLinks(ctx context.Context) ([]db.Link, error) {
	links, err := r.queries.ListLinks(ctx, db.ListLinksParams{
		Limit:  1000, // какой-то большой лимит
		Offset: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("list links: %w", err)
	}
	return links, nil
}

// ListLinksWithPagination возвращает ссылки с пагинацией и общее количество
func (r *Repository) ListLinksWithPagination(ctx context.Context, limit, offset int32) ([]db.Link, int64, error) {
	// Получаем общее количество
	count, err := r.queries.CountLinks(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count links: %w", err)
	}

	// Получаем записи с пагинацией
	links, err := r.queries.ListLinks(ctx, db.ListLinksParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list links with pagination: %w", err)
	}

	return links, count, nil
}

// UpdateLink обновляет ссылку
func (r *Repository) UpdateLink(ctx context.Context, id int32, originalURL, shortName *string) (*db.Link, error) {
	if err := r.checkLinkExists(ctx, id); err != nil {
		return nil, err
	}

	if shortName != nil {
		if err := r.checkShortNameUnique(ctx, *shortName, id); err != nil {
			return nil, err
		}
	}

	params := r.buildUpdateParams(id, originalURL, shortName)

	link, err := r.queries.UpdateLink(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("update link: %w", err)
	}

	return &link, nil
}

func (r *Repository) checkLinkExists(ctx context.Context, id int32) error {
	_, err := r.queries.GetLink(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("check existing: %w", err)
	}
	return nil
}

func (r *Repository) checkShortNameUnique(ctx context.Context, shortName string, excludeID int32) error {
	existing, err := r.queries.GetLinkByShortName(ctx, shortName)
	if err == nil && existing.ID != excludeID {
		return ErrAlreadyExists
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("check short name: %w", err)
	}
	return nil
}

func (r *Repository) buildUpdateParams(id int32, originalURL, shortName *string) db.UpdateLinkParams {
	params := db.UpdateLinkParams{ID: id}

	if originalURL != nil {
		params.OriginalUrl = pgtype.Text{String: *originalURL, Valid: true}
	}

	if shortName != nil {
		params.ShortName = pgtype.Text{String: *shortName, Valid: true}
	}

	return params
}

// DeleteLink удаляет ссылку
func (r *Repository) DeleteLink(ctx context.Context, id int32) error {
	_, err := r.queries.GetLink(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("check existing: %w", err)
	}

	err = r.queries.DeleteLink(ctx, id)
	if err != nil {
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

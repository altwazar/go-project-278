//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"urlshortener/internal/api"
	"urlshortener/internal/config"
	"urlshortener/internal/db"
	"urlshortener/internal/repository"
)

// withTransactionForSubtest создает транзакцию для одного подтеста
func withTransactionForSubtest(t *testing.T, fn func(ctx context.Context, repo *repository.Repository)) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Создаем репозиторий
	repo, err := repository.New(ctx, connStr)
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}
	defer repo.Close()

	// Начинаем транзакцию
	err = repo.WithTransaction(ctx, func(q *db.Queries) error {
		// Создаем репозиторий с транзакцией
		txRepo := &repository.Repository{}
		txRepo.SetQueries(q)

		// Выполняем тест
		fn(ctx, txRepo)
		return errors.New("rollback transaction")
	})
}

// setupTestServer создает тестовый сервер с репозиторием
func setupTestServer(t *testing.T) (*gin.Engine, *repository.Repository, *config.Config) {
	t.Helper()

	cfg := &config.Config{
		DatabaseURL: connStr,
		BaseURL:     "http://localhost:8080",
		Port:        "8080",
	}

	repo, err := repository.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	t.Cleanup(func() { repo.Close() })

	handlers := api.NewHandlers(repo, cfg)
	router := setupTestRouter(handlers)

	return router, repo, cfg
}

// setupTestRouter создает роутер без паник-мидлвары для тестов
func setupTestRouter(handlers *api.Handlers) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Logger())

	apiGroup := router.Group("/api")
	{
		apiGroup.GET("/links", handlers.ListLinks)
		apiGroup.POST("/links", handlers.CreateLink)
		apiGroup.GET("/links/:id", handlers.GetLink)
		apiGroup.PUT("/links/:id", handlers.UpdateLink)
		apiGroup.DELETE("/links/:id", handlers.DeleteLink)
	}

	router.GET("/r/:shortName", handlers.RedirectHandler)
	router.GET("/health", handlers.HealthCheck)

	return router
}

// executeRequest выполняет HTTP запрос к тестовому серверу
func executeRequest(router *gin.Engine, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// parseResponse парсит JSON ответ
func parseResponse(t *testing.T, resp *httptest.ResponseRecorder, v interface{}) {
	t.Helper()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("failed to parse response: %v, body: %s", err, resp.Body.String())
	}
}

// createTestLink создает тестовую ссылку в базе
func createTestLink(t *testing.T, ctx context.Context, repo *repository.Repository, originalURL, shortName string) *db.Link {
	t.Helper()

	link, err := repo.CreateLink(ctx, originalURL, shortName)
	if err != nil {
		t.Fatalf("failed to create test link: %v", err)
	}
	return link
}

// TestRequest упрощает создание HTTP запросов
type TestRequest struct {
	Method string
	Path   string
	Body   interface{}
}

func (tr TestRequest) toHTTP(t *testing.T) *http.Request {
	t.Helper()

	var bodyBytes []byte
	var err error

	if tr.Body != nil {
		bodyBytes, err = json.Marshal(tr.Body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
	}

	req, err := http.NewRequest(tr.Method, tr.Path, bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	if tr.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req
}

// stringPtr возвращает указатель на строку
func stringPtr(s string) *string {
	return &s
}

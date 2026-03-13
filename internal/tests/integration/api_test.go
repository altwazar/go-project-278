//go:build integration

package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"urlshortener/internal/api"
	"urlshortener/internal/config"
	"urlshortener/internal/db"
	"urlshortener/internal/repository"

	"github.com/stretchr/testify/assert"
)

func TestCreateLink(t *testing.T) {
	// Создаем дубликат для теста на наличие дубликата
	withTransactionForSubtest(t, true, func(ctx context.Context, repo *repository.Repository) {
		createTestLink(t, ctx, repo, "https://duplicate.com", "duplicate")
	})

	tests := []struct {
		name       string
		request    TestRequest
		wantStatus int
		wantFields map[string]interface{}
	}{
		{
			name: "successful creation with custom short name",
			request: TestRequest{
				Method: "POST",
				Path:   "/api/links",
				Body: api.CreateLinkRequest{
					OriginalURL: "https://example.com",
					ShortName:   "custom123",
				},
			},
			wantStatus: http.StatusCreated,
			wantFields: map[string]interface{}{
				"original_url": "https://example.com",
				"short_name":   "custom123",
			},
		},
		{
			name: "successful creation with generated short name",
			request: TestRequest{
				Method: "POST",
				Path:   "/api/links",
				Body: api.CreateLinkRequest{
					OriginalURL: "https://example.org",
				},
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "invalid URL",
			request: TestRequest{
				Method: "POST",
				Path:   "/api/links",
				Body: api.CreateLinkRequest{
					OriginalURL: "not-a-url",
					ShortName:   "test",
				},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "duplicate short name",
			request: TestRequest{
				Method: "POST",
				Path:   "/api/links",
				Body: api.CreateLinkRequest{
					OriginalURL: "https://example.com",
					ShortName:   "duplicate",
				},
			},
			wantStatus: http.StatusConflict,
		},
		{
			name: "missing original URL",
			request: TestRequest{
				Method: "POST",
				Path:   "/api/links",
				Body: api.CreateLinkRequest{
					ShortName: "test",
				},
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withTransactionForSubtest(t, false, func(ctx context.Context, repo *repository.Repository) {
				router := setupTestRouter(api.NewHandlers(repo, &config.Config{BaseURL: "http://localhost:8080"}))

				req := tt.request.toHTTP(t)
				resp := executeRequest(router, req)

				assert.Equal(t, tt.wantStatus, resp.Code)

				if tt.wantStatus == http.StatusCreated {
					var response api.LinkResponse
					parseResponse(t, resp, &response)

					assert.NotZero(t, response.ID)
					assert.NotEmpty(t, response.ShortURL)

					if tt.wantFields != nil {
						if val, ok := tt.wantFields["original_url"]; ok {
							assert.Equal(t, val, response.OriginalURL)
						}
						if val, ok := tt.wantFields["short_name"]; ok {
							assert.Equal(t, val, response.ShortName)
						}
					}
				}
			})
		})
	}
}

func TestGetLink(t *testing.T) {
	tests := []struct {
		name       string
		setupData  func(t *testing.T, repo *repository.Repository) *db.Link
		path       func(link *db.Link) string
		wantStatus int
	}{
		{
			name: "existing link",
			setupData: func(t *testing.T, repo *repository.Repository) *db.Link {
				return createTestLink(t, context.Background(), repo, "https://example.com", "test123")
			},
			path: func(link *db.Link) string {
				return "/api/links/" + strconv.Itoa(int(link.ID))
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "non-existing link",
			setupData: func(t *testing.T, repo *repository.Repository) *db.Link {
				return nil
			},
			path: func(link *db.Link) string {
				return "/api/links/999999"
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "invalid ID format",
			setupData: func(t *testing.T, repo *repository.Repository) *db.Link {
				return nil
			},
			path: func(link *db.Link) string {
				return "/api/links/invalid"
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withTransactionForSubtest(t, false, func(ctx context.Context, repo *repository.Repository) {
				testLink := tt.setupData(t, repo)

				router := setupTestRouter(api.NewHandlers(repo, &config.Config{BaseURL: "http://localhost:8080"}))

				req := TestRequest{Method: "GET", Path: tt.path(testLink)}.toHTTP(t)
				resp := executeRequest(router, req)

				assert.Equal(t, tt.wantStatus, resp.Code)

				if tt.wantStatus == http.StatusOK && testLink != nil {
					var response api.LinkResponse
					parseResponse(t, resp, &response)

					assert.Equal(t, testLink.OriginalUrl, response.OriginalURL)
					assert.Equal(t, testLink.ShortName, response.ShortName)
				}
			})
		})
	}
}

func TestListLinks(t *testing.T) {
	t.Run("list all links", func(t *testing.T) {
		withTransactionForSubtest(t, false, func(ctx context.Context, repo *repository.Repository) {
			// Создаем несколько тестовых ссылок
			createTestLink(t, ctx, repo, "https://example1.com", "link1")
			createTestLink(t, ctx, repo, "https://example2.com", "link2")
			createTestLink(t, ctx, repo, "https://example3.com", "link3")

			router := setupTestRouter(api.NewHandlers(repo, &config.Config{BaseURL: "http://localhost:8080"}))

			req := TestRequest{Method: "GET", Path: "/api/links"}.toHTTP(t)
			resp := executeRequest(router, req)

			assert.Equal(t, http.StatusOK, resp.Code)

			var response []api.LinkResponse
			parseResponse(t, resp, &response)

			assert.GreaterOrEqual(t, len(response), 3)
		})
	})
}

func TestUpdateLink(t *testing.T) {
	// Создаем ссылку с существующим именем для теста конфликта в отдельной транзакции
	var existingShortName string
	withTransactionForSubtest(t, true, func(ctx context.Context, repo *repository.Repository) {
		link := createTestLink(t, ctx, repo, "https://existing.com", "existing")
		existingShortName = link.ShortName
	})

	tests := []struct {
		name       string
		setupData  func(ctx context.Context, t *testing.T, repo *repository.Repository) *db.Link
		request    func(link *db.Link) TestRequest
		wantStatus int
		checkFunc  func(t *testing.T, resp *httptest.ResponseRecorder, originalLink *db.Link)
	}{
		{
			name: "update original URL only",
			setupData: func(ctx context.Context, t *testing.T, repo *repository.Repository) *db.Link {
				return createTestLink(t, ctx, repo, "https://example.com", "update1")
			},
			request: func(link *db.Link) TestRequest {
				return TestRequest{
					Method: "PUT",
					Path:   "/api/links/" + strconv.Itoa(int(link.ID)),
					Body: api.UpdateLinkRequest{
						OriginalURL: stringPtr("https://updated.com"),
					},
				}
			},
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, resp *httptest.ResponseRecorder, originalLink *db.Link) {
				var response api.LinkResponse
				parseResponse(t, resp, &response)
				assert.Equal(t, "https://updated.com", response.OriginalURL)
				assert.Equal(t, originalLink.ShortName, response.ShortName)
			},
		},
		{
			name: "update short name only",
			setupData: func(ctx context.Context, t *testing.T, repo *repository.Repository) *db.Link {
				return createTestLink(t, ctx, repo, "https://example.com", "update2")
			},
			request: func(link *db.Link) TestRequest {
				return TestRequest{
					Method: "PUT",
					Path:   "/api/links/" + strconv.Itoa(int(link.ID)),
					Body: api.UpdateLinkRequest{
						ShortName: stringPtr("newname2"),
					},
				}
			},
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, resp *httptest.ResponseRecorder, originalLink *db.Link) {
				var response api.LinkResponse
				parseResponse(t, resp, &response)
				assert.Equal(t, originalLink.OriginalUrl, response.OriginalURL)
				assert.Equal(t, "newname2", response.ShortName)
			},
		},
		{
			name: "update both fields",
			setupData: func(ctx context.Context, t *testing.T, repo *repository.Repository) *db.Link {
				return createTestLink(t, ctx, repo, "https://example.com", "update3")
			},
			request: func(link *db.Link) TestRequest {
				return TestRequest{
					Method: "PUT",
					Path:   "/api/links/" + strconv.Itoa(int(link.ID)),
					Body: api.UpdateLinkRequest{
						OriginalURL: stringPtr("https://both.com"),
						ShortName:   stringPtr("bothname3"),
					},
				}
			},
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, resp *httptest.ResponseRecorder, originalLink *db.Link) {
				var response api.LinkResponse
				parseResponse(t, resp, &response)
				assert.Equal(t, "https://both.com", response.OriginalURL)
				assert.Equal(t, "bothname3", response.ShortName)
			},
		},
		{
			name: "no fields to update",
			setupData: func(ctx context.Context, t *testing.T, repo *repository.Repository) *db.Link {
				return createTestLink(t, ctx, repo, "https://example.com", "update4")
			},
			request: func(link *db.Link) TestRequest {
				return TestRequest{
					Method: "PUT",
					Path:   "/api/links/" + strconv.Itoa(int(link.ID)),
					Body:   api.UpdateLinkRequest{},
				}
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "update non-existing link",
			setupData: func(ctx context.Context, t *testing.T, repo *repository.Repository) *db.Link {
				return nil
			},
			request: func(link *db.Link) TestRequest {
				return TestRequest{
					Method: "PUT",
					Path:   "/api/links/999999",
					Body: api.UpdateLinkRequest{
						OriginalURL: stringPtr("https://test.com"),
					},
				}
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "update with existing short name",
			setupData: func(ctx context.Context, t *testing.T, repo *repository.Repository) *db.Link {
				return createTestLink(t, ctx, repo, "https://example.com", "update5")
			},
			request: func(link *db.Link) TestRequest {
				return TestRequest{
					Method: "PUT",
					Path:   "/api/links/" + strconv.Itoa(int(link.ID)),
					Body: api.UpdateLinkRequest{
						ShortName: &existingShortName,
					},
				}
			},
			wantStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withTransactionForSubtest(t, false, func(ctx context.Context, repo *repository.Repository) {
				testLink := tt.setupData(ctx, t, repo)

				router := setupTestRouter(api.NewHandlers(repo, &config.Config{BaseURL: "http://localhost:8080"}))

				req := tt.request(testLink).toHTTP(t)
				resp := executeRequest(router, req)

				assert.Equal(t, tt.wantStatus, resp.Code)

				if tt.checkFunc != nil && tt.wantStatus == http.StatusOK {
					tt.checkFunc(t, resp, testLink)
				}
			})
		})
	}
}
func TestDeleteLink(t *testing.T) {
	tests := []struct {
		name       string
		setupData  func(t *testing.T, repo *repository.Repository) *db.Link
		path       func(link *db.Link) string
		wantStatus int
	}{
		{
			name: "delete existing link",
			setupData: func(t *testing.T, repo *repository.Repository) *db.Link {
				return createTestLink(t, context.Background(), repo, "https://example.com", "delete123")
			},
			path: func(link *db.Link) string {
				return "/api/links/" + strconv.Itoa(int(link.ID))
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "delete non-existing link",
			setupData: func(t *testing.T, repo *repository.Repository) *db.Link {
				return nil
			},
			path: func(link *db.Link) string {
				return "/api/links/999999"
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "invalid ID format",
			setupData: func(t *testing.T, repo *repository.Repository) *db.Link {
				return nil
			},
			path: func(link *db.Link) string {
				return "/api/links/invalid"
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withTransactionForSubtest(t, false, func(ctx context.Context, repo *repository.Repository) {
				testLink := tt.setupData(t, repo)

				router := setupTestRouter(api.NewHandlers(repo, &config.Config{BaseURL: "http://localhost:8080"}))

				req := TestRequest{Method: "DELETE", Path: tt.path(testLink)}.toHTTP(t)
				resp := executeRequest(router, req)

				assert.Equal(t, tt.wantStatus, resp.Code)
			})
		})
	}
}

func TestRedirectHandler(t *testing.T) {
	tests := []struct {
		name         string
		setupData    func(t *testing.T, repo *repository.Repository) *db.Link
		shortName    func(link *db.Link) string
		wantStatus   int
		wantLocation string
	}{
		{
			name: "existing short name",
			setupData: func(t *testing.T, repo *repository.Repository) *db.Link {
				return createTestLink(t, context.Background(), repo, "https://example.com", "redirect123")
			},
			shortName: func(link *db.Link) string {
				return link.ShortName
			},
			wantStatus:   http.StatusMovedPermanently,
			wantLocation: "https://example.com",
		},
		{
			name: "non-existing short name",
			setupData: func(t *testing.T, repo *repository.Repository) *db.Link {
				return nil
			},
			shortName: func(link *db.Link) string {
				return "nonexistent"
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "empty short name",
			setupData: func(t *testing.T, repo *repository.Repository) *db.Link {
				return nil
			},
			shortName: func(link *db.Link) string {
				return ""
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withTransactionForSubtest(t, false, func(ctx context.Context, repo *repository.Repository) {
				testLink := tt.setupData(t, repo)

				router := setupTestRouter(api.NewHandlers(repo, &config.Config{BaseURL: "http://localhost:8080"}))

				path := "/r/" + tt.shortName(testLink)
				if tt.shortName(testLink) == "" {
					path = "/r/"
				}
				req := TestRequest{Method: "GET", Path: path}.toHTTP(t)
				resp := executeRequest(router, req)

				assert.Equal(t, tt.wantStatus, resp.Code)

				if tt.wantStatus == http.StatusMovedPermanently {
					assert.Equal(t, tt.wantLocation, resp.Header().Get("Location"))
				}
			})
		})
	}
}

func TestHealthCheck(t *testing.T) {
	t.Run("health check", func(t *testing.T) {
		withTransactionForSubtest(t, false, func(ctx context.Context, repo *repository.Repository) {
			router := setupTestRouter(api.NewHandlers(repo, &config.Config{BaseURL: "http://localhost:8080"}))

			req := TestRequest{Method: "GET", Path: "/health"}.toHTTP(t)
			resp := executeRequest(router, req)

			assert.Equal(t, http.StatusOK, resp.Code)

			var response map[string]string
			parseResponse(t, resp, &response)
			assert.Equal(t, "ok", response["status"])
			assert.Equal(t, "connected", response["db"])
		})
	})
}

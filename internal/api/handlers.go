// Package api содержит обработчики HTTP запросов
// CRUD плюс проверка работы перенаправления и проверка здоровья сервиса
package api

import (
	"errors"
	"net/http"
	"strconv"

	"urlshortener/internal/db"

	"github.com/gin-gonic/gin"
	"urlshortener/internal/config"
	"urlshortener/internal/repository"
)

// Handlers стуктура прослойка для обработчиков запросов над БД
type Handlers struct {
	repo   *repository.Repository
	config *config.Config
}

// NewHandlers конструктор Handlers
func NewHandlers(repo *repository.Repository, config *config.Config) *Handlers {
	return &Handlers{
		repo:   repo,
		config: config,
	}
}

// ListLinks возвращает все ссылки
// GET /api/links
func (h *Handlers) ListLinks(c *gin.Context) {
	links, err := h.repo.ListLinks(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch links"})
		return
	}

	response := make([]LinkResponse, len(links))
	for i, link := range links {
		response[i] = LinkResponse{
			ID:          int64(link.ID),
			OriginalURL: link.OriginalUrl,
			ShortName:   link.ShortName,
			ShortURL:    h.config.BaseURL + "/r/" + link.ShortName,
		}
	}

	c.JSON(http.StatusOK, response)
}

// CreateLink создает новую ссылку
// POST /api/links
func (h *Handlers) CreateLink(c *gin.Context) {
	var req CreateLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	shortName, err := h.getOrGenerateShortName(c, req)
	if err != nil {
		return // ответ уже отправлен в getOrGenerateShortName
	}

	link, err := h.repo.CreateLink(c.Request.Context(), req.OriginalURL, shortName)
	if err != nil {
		h.handleCreateError(c, err)
		return
	}

	h.sendLinkResponse(c, http.StatusCreated, link)
}

func (h *Handlers) getOrGenerateShortName(c *gin.Context, req CreateLinkRequest) (string, error) {
	if req.ShortName != "" {
		return req.ShortName, nil
	}

	generated, err := h.repo.GenerateShortName(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate short name"})
		return "", err
	}
	return generated, nil
}

func (h *Handlers) handleCreateError(c *gin.Context, err error) {
	if errors.Is(err, repository.ErrAlreadyExists) {
		c.JSON(http.StatusConflict, gin.H{"error": "Short name already exists"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create link"})
}

func (h *Handlers) sendLinkResponse(c *gin.Context, status int, link *db.Link) {
	response := LinkResponse{
		ID:          int64(link.ID),
		OriginalURL: link.OriginalUrl,
		ShortName:   link.ShortName,
		ShortURL:    h.config.BaseURL + "/r/" + link.ShortName,
	}
	c.JSON(status, response)
}

// GetLink возвращает ссылку по ID
// GET /api/links/:id
func (h *Handlers) GetLink(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	link, err := h.repo.GetLinkByID(c.Request.Context(), int32(id))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Link not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch link"})
		return
	}

	response := LinkResponse{
		ID:          int64(link.ID),
		OriginalURL: link.OriginalUrl,
		ShortName:   link.ShortName,
		ShortURL:    h.config.BaseURL + "/r/" + link.ShortName,
	}

	c.JSON(http.StatusOK, response)
}

// UpdateLink обновляет ссылку
// PUT /api/links/:id
func (h *Handlers) UpdateLink(c *gin.Context) {
	id, err := h.parseID(c)
	if err != nil {
		return // ответ уже отправлен в parseID
	}

	var req UpdateLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !h.hasFieldsToUpdate(&req) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one field to update must be provided"})
		return
	}

	link, err := h.repo.UpdateLink(c.Request.Context(), id, req.OriginalURL, req.ShortName)
	if err != nil {
		h.handleUpdateError(c, err)
		return
	}

	h.sendLinkResponse(c, http.StatusOK, link)
}

func (h *Handlers) parseID(c *gin.Context) (int32, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return 0, err
	}
	return int32(id), nil
}

func (h *Handlers) hasFieldsToUpdate(req *UpdateLinkRequest) bool {
	return req.OriginalURL != nil || req.ShortName != nil
}

func (h *Handlers) handleUpdateError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Link not found"})
	case errors.Is(err, repository.ErrAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{"error": "Short name already exists"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update link"})
	}
}

// DeleteLink удаляет ссылку
// DELETE /api/links/:id
func (h *Handlers) DeleteLink(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	err = h.repo.DeleteLink(c.Request.Context(), int32(id))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Link not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete link"})
		return
	}

	c.Status(http.StatusNoContent)
}

// RedirectHandler перенаправляет по короткой ссылке
// GET /r/:shortName
func (h *Handlers) RedirectHandler(c *gin.Context) {
	shortName := c.Param("shortName")
	if shortName == "" {
		c.String(http.StatusBadRequest, "Short name is required")
		return
	}

	link, err := h.repo.GetLinkByShortName(c.Request.Context(), shortName)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.String(http.StatusNotFound, "Link not found")
			return
		}
		c.String(http.StatusInternalServerError, "Internal server error")
		return
	}

	// Используем 301 Moved Permanently для редиректа
	c.Redirect(http.StatusMovedPermanently, link.OriginalUrl)
}

// HealthCheck проверяет работоспособность сервиса
// GET /health
func (h *Handlers) HealthCheck(c *gin.Context) {
	// Проверяем соединение с БД
	_, err := h.repo.ListLinks(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unavailable",
			"db":     "disconnected",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"db":     "connected",
	})
}

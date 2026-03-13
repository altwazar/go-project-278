package api

// CreateLinkRequest для работы с запросами и ответами
// Запрос на создание линка
type CreateLinkRequest struct {
	OriginalURL string `json:"original_url" binding:"required,url"`
	ShortName   string `json:"short_name" binding:"omitempty,alphanum,max=50"`
}

// UpdateLinkRequest для запросов на обновление линка
type UpdateLinkRequest struct {
	OriginalURL *string `json:"original_url" binding:"omitempty,url"`
	ShortName   *string `json:"short_name" binding:"omitempty,alphanum,max=50"`
}

// LinkResponse для выдачи линка в ответе
type LinkResponse struct {
	ID          int64  `json:"id"`
	OriginalURL string `json:"original_url"`
	ShortName   string `json:"short_name"`
	ShortURL    string `json:"short_url"`
}

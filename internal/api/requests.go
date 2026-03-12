package api

type CreateLinkRequest struct {
	OriginalURL string `json:"original_url" binding:"required,url"`
	ShortName   string `json:"short_name" binding:"omitempty,alphanum,max=50"`
}

type UpdateLinkRequest struct {
	OriginalURL *string `json:"original_url" binding:"omitempty,url"`
	ShortName   *string `json:"short_name" binding:"omitempty,alphanum,max=50"`
}

type LinkResponse struct {
	ID          int64  `json:"id"`
	OriginalURL string `json:"original_url"`
	ShortName   string `json:"short_name"`
	ShortURL    string `json:"short_url"`
}

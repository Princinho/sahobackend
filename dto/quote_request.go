package dto

type CreateQuoteNoteDTO struct {
	Content string `json:"content" binding:"required,min=1,max=5000"`
}

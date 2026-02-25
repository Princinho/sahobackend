package dto

type CreateQuoteNoteDTO struct {
	Content string `json:"content" binding:"required,min=1,max=5000"`
}
type QuoteRequestItemDTO struct {
	ProductID string `json:"productId" binding:"required"`
	Quantity  int    `json:"quantity"  binding:"required,min=1"`
}

type CreateQuoteRequestDTO struct {
	FullName string `json:"fullName" binding:"required"`
	Email    string `json:"email"    binding:"required,email"`
	Phone    string `json:"phone"`

	Country string `json:"country"`
	City    string `json:"city"`
	Address string `json:"address"`

	Message string                `json:"message"`
	Items   []QuoteRequestItemDTO `json:"items" binding:"required,min=1,dive"`
}

type UpdateQuoteStatusDTO struct {
	Status string `json:"status" binding:"required"`
}
type AddAdminNoteDTO struct {
	Content string `json:"content" binding:"required"`
}

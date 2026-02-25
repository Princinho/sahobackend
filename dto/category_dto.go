package dto

// CreateCategoryDTO is parsed from the "data" multipart field (JSON)
type CreateCategoryDTO struct {
	Name        string `json:"name" binding:"required"`
	Slug        string `json:"slug"` // auto-generated from Name if empty
	Description string `json:"description"`
	IsActive    bool   `json:"isActive"`
}

// UpdateCategoryDTO — all fields are optional pointers
type UpdateCategoryDTO struct {
	Name        *string `json:"name"`
	Slug        *string `json:"slug"`
	Description *string `json:"description"`
	IsActive    *bool   `json:"isActive"`
}

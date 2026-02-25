package dto

type CreateCategoryDTO struct {
	Name string `json:"name" binding:"required,min=2"`
	Slug string `json:"slug" binding:"required,min=2"`
}

type UpdateCategoryDTO struct {
	Name *string `json:"name,omitempty"`
	Slug *string `json:"slug,omitempty"`
}

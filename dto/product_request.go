package dto

import "time"

type CreateProductRequestDTO struct {
	FullName string `json:"fullName" binding:"required"`
	Email    string `json:"email"    binding:"required,email"`
	Phone    string `json:"phone"`

	Company string `json:"company"`

	Country string `json:"country"`
	City    string `json:"city"`

	Description     string     `json:"description" binding:"required,min=5,max=8000"`
	Quantity        int        `json:"quantity" binding:"omitempty,min=1"`
	DesiredDeadline *time.Time `json:"desiredDeadline"`
	Budget          string     `json:"budget"`
	ReferenceURL    string     `json:"referenceUrl"`
}

type UpdateProductRequestStatusDTO struct {
	Status string `json:"status" binding:"required"`
}

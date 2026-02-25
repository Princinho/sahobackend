package dto

type CreateProductDTO struct {
	Name            string   `json:"name" binding:"required,min=3"`
	Price           float64  `json:"price" binding:"required,gt=0"`
	Quantity        int      `json:"quantity" binding:"required,gte=0"`
	Slug            string   `json:"slug" binding:"required"`
	CategoryIds     []string `json:"categoryIds" binding:"required,min=1"`
	Materials       []string `json:"materials"`
	Colors          []string `json:"colors"`
	Description     string   `json:"description"`
	DescriptionFull string   `json:"descriptionFull"`
	Dimensions      string   `json:"dimensions"`
	Weight          string   `json:"weight"`
	IsTrending      bool     `json:"isTrending"`
	IsDisabled      bool     `json:"isDisabled"`
}
type UpdateProductDTO struct {
	Name              *string   `json:"name,omitempty"`
	Price             *float64  `json:"price,omitempty"`
	Quantity          *int      `json:"quantity,omitempty"`
	Slug              *string   `json:"slug,omitempty"`
	Description       *string   `json:"description,omitempty"`
	DescriptionFull   *string   `json:"descriptionFull,omitempty"`
	Materials         *[]string `json:"materials,omitempty"`
	Colors            *[]string `json:"colors,omitempty"`
	Dimensions        *string   `json:"dimensions,omitempty"`
	Weight            *string   `json:"weight,omitempty"`
	IsTrending        *bool     `json:"isTrending,omitempty"`
	IsDisabled        *bool     `json:"isDisabled,omitempty"`
	CategoryIds       *[]string `json:"categoryIds" binding:"required,min=1"`
	RemovedImagesUrls []string  `json:"removedImagesUrls,omitempty"`
}

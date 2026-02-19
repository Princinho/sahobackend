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
	Name              string   `json:"name" binding:"required,min=3"`
	Price             float64  `json:"price" binding:"required,gt=0"`
	Quantity          int      `json:"quantity" binding:"required,gte=0"`
	Slug              string   `json:"slug" binding:"required"`
	CategoryIds       []string `json:"categoryIds" binding:"required,min=1"`
	RemovedImagesUrls []string `json:"removedImageUrls"`
	Materials         []string `json:"materials"`
	Colors            []string `json:"colors"`
	Description       string   `json:"description"`
	DescriptionFull   string   `json:"descriptionFull"`
	Dimensions        string   `json:"dimensions"`
	Weight            string   `json:"weight"`
	IsTrending        bool     `json:"isTrending"`
	IsDisabled        bool     `json:"isDisabled"`
}

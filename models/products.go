package models

import "go.mongodb.org/mongo-driver/v2/bson"

type Product struct {
	Id                 bson.ObjectID   `bson:"_id" json:"id"`
	Name               string          `json:"name"`
	Price              float64         `json:"price"`
	Quantity           int             `json:"quantity"`
	Slug               string          `json:"slug"`
	CategoryIds        []bson.ObjectID `json:"categoryIds"`
	ImageUrls          []string        `json:"imageUrls"`
	IsTrending         bool            `json:"isTrending"`
	Materials          []string        `json:"materials"`
	Colors             []string        `json:"colors"`
	Description        string          `json:"description"`
	DescriptionFull    string          `json:"descriptionFull"`
	Dimensions         string          `json:"dimensions"`
	Weight             string          `json:"weight"`
	SimilarProductsIds []bson.ObjectID `json:"similarProductsIds"`
	IsDisabled         bool            `json:"isDisabled"`
}

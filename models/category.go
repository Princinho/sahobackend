package models

import "go.mongodb.org/mongo-driver/v2/bson"

type Category struct {
	Id          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string        `bson:"name" json:"name"`
	Slug        string        `bson:"slug" json:"slug"`
	Description string        `bson:"description,omitempty" json:"description,omitempty"`
	IsActive    bool          `bson:"isActive" json:"isActive"`
	ImageUrl    string        `bson:"imageUrl,omitempty" json:"imageUrl,omitempty"`
}

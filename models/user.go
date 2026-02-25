package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Role string

const (
	RoleAdmin Role = "ADMIN"
)

type User struct {
	ID           bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Email        string        `bson:"email" json:"email"`
	PasswordHash string        `bson:"passwordHash" json:"-"` // never expose
	Role         Role          `bson:"role" json:"role"`
	IsActive     bool          `bson:"isActive" json:"isActive"`
	CreatedAt    time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt    time.Time     `bson:"updatedAt" json:"updatedAt"`
}

type RefreshToken struct {
	ID         bson.ObjectID `bson:"_id,omitempty"`
	UserID     bson.ObjectID `bson:"userId"`
	TokenHash  string        `bson:"tokenHash"`
	ExpiresAt  time.Time     `bson:"expiresAt"`
	CreatedAt  time.Time     `bson:"createdAt"`
	RevokedAt  *time.Time    `bson:"revokedAt,omitempty"`
	ReplacedBy *string       `bson:"replacedBy,omitempty"`
}

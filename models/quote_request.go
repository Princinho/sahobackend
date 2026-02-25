package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type QuoteRequestStatus string

const (
	QuoteStatusNew        QuoteRequestStatus = "NEW"
	QuoteStatusInProgress QuoteRequestStatus = "IN_PROGRESS"
	QuoteStatusQuoted     QuoteRequestStatus = "QUOTED"
	QuoteStatusRejected   QuoteRequestStatus = "REJECTED"
	QuoteStatusClosed     QuoteRequestStatus = "CLOSED"
)

type QuoteAttachment struct {
	PublicURL  string `bson:"publicUrl" json:"publicUrl"`
	ObjectName string `bson:"objectName" json:"objectName"`
	MimeType   string `bson:"mimeType" json:"mimeType"`
	SizeBytes  int64  `bson:"sizeBytes" json:"sizeBytes"`
}

type QuoteAdminNote struct {
	ID bson.ObjectID `bson:"_id,omitempty" json:"id"`

	AuthorID    bson.ObjectID `bson:"authorId" json:"authorId"`
	AuthorEmail string        `bson:"authorEmail" json:"authorEmail"`

	Content   string           `bson:"content" json:"content"`
	CreatedAt time.Time        `bson:"createdAt" json:"createdAt"`
	QuotePDF  *QuoteAttachment `bson:"quotePdf,omitempty" json:"quotePdf,omitempty"`
}

type QuoteRequestItem struct {
	ProductID bson.ObjectID `bson:"productId" json:"productId"`
	Quantity  int           `bson:"quantity" json:"quantity"`

	ProductName string  `bson:"productName,omitempty" json:"productName,omitempty"`
	ProductSlug string  `bson:"productSlug,omitempty" json:"productSlug,omitempty"`
	UnitPrice   float64 `bson:"unitPrice,omitempty" json:"unitPrice,omitempty"`
}

type QuoteRequest struct {
	ID bson.ObjectID `bson:"_id,omitempty" json:"id"`

	FullName string `bson:"fullName" json:"fullName"`
	Email    string `bson:"email" json:"email"`
	Phone    string `bson:"phone,omitempty" json:"phone,omitempty"`

	Country string `bson:"country,omitempty" json:"country,omitempty"`
	City    string `bson:"city,omitempty" json:"city,omitempty"`
	Address string `bson:"address,omitempty" json:"address,omitempty"`

	Message string             `bson:"message,omitempty" json:"message,omitempty"`
	Items   []QuoteRequestItem `bson:"items" json:"items"`

	Status   QuoteRequestStatus `bson:"status" json:"status"`
	QuotedAt *time.Time         `bson:"quotedAt,omitempty" json:"quotedAt,omitempty"`

	Notes []QuoteAdminNote `bson:"notes,omitempty" json:"notes,omitempty"`

	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}

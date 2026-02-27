package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type ProductRequestStatus string

const (
	ProductRequestStatusNew        ProductRequestStatus = "NEW"
	ProductRequestStatusInProgress ProductRequestStatus = "IN_PROGRESS"
	ProductRequestStatusAnswered   ProductRequestStatus = "ANSWERED"
	ProductRequestStatusRejected   ProductRequestStatus = "REJECTED"
	ProductRequestStatusClosed     ProductRequestStatus = "CLOSED"
)

type ProductRequestAttachment struct {
	ImageURL   string    `bson:"imageUrl"  json:"imageUrl"`
	ObjectName string    `bson:"objectName" json:"objectName"`
	MimeType   string    `bson:"mimeType"   json:"mimeType"`
	SizeBytes  int64     `bson:"sizeBytes"  json:"sizeBytes"`
	FileName   string    `bson:"fileName"   json:"fileName"`
	UploadedAt time.Time `bson:"uploadedAt" json:"uploadedAt"`
}

type ProductRequestAdminNote struct {
	ID          bson.ObjectID             `bson:"id"          json:"id"`
	AuthorID    bson.ObjectID             `bson:"authorId"    json:"authorId"`
	AuthorEmail string                    `bson:"authorEmail" json:"authorEmail"`
	Content     string                    `bson:"content"     json:"content"`
	Attachment  *ProductRequestAttachment `bson:"attachment,omitempty" json:"attachment,omitempty"`
	CreatedAt   time.Time                 `bson:"createdAt"   json:"createdAt"`
}

type ProductRequest struct {
	Id              bson.ObjectID             `bson:"_id" json:"id"`
	FullName        string                    `bson:"fullName"  json:"fullName"`
	Email           string                    `bson:"email"     json:"email"`
	Phone           string                    `bson:"phone"     json:"phone"`
	Company         string                    `bson:"company"   json:"company"`
	VATNumber       string                    `bson:"vatNumber" json:"vatNumber"`
	Country         string                    `bson:"country" json:"country"`
	City            string                    `bson:"city"    json:"city"`
	Description     string                    `bson:"description"     json:"description"`
	Quantity        int                       `bson:"quantity"        json:"quantity"`
	DesiredDeadline *time.Time                `bson:"desiredDeadline,omitempty" json:"desiredDeadline,omitempty"`
	Budget          string                    `bson:"budget"          json:"budget"`
	ReferenceURL    string                    `bson:"referenceUrl"    json:"referenceUrl"`
	ReferenceImage  *ProductRequestAttachment `bson:"referenceImage,omitempty" json:"referenceImage,omitempty"`
	Status          ProductRequestStatus      `bson:"status"     json:"status"`
	Notes           []ProductRequestAdminNote `bson:"notes"      json:"notes"`
	AnsweredAt      *time.Time                `bson:"answeredAt,omitempty" json:"answeredAt,omitempty"`
	CreatedAt       time.Time                 `bson:"createdAt" json:"createdAt"`
	UpdatedAt       time.Time                 `bson:"updatedAt" json:"updatedAt"`
}

package controllers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/princinho/sahobackend/database"
	"github.com/princinho/sahobackend/dto"
	"github.com/princinho/sahobackend/models"
	"github.com/princinho/sahobackend/utils"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func AdminGetQuoteNotes() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		qCol := database.OpenCollection("quote_requests")

		qid, err := bson.ObjectIDFromHex(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid quote request id"})
			return
		}

		var qr models.QuoteRequest
		if err := qCol.FindOne(ctx, bson.M{"_id": qid}).Decode(&qr); err != nil {
			c.JSON(404, gin.H{"error": "quote request not found"})
			return
		}

		c.JSON(200, gin.H{"items": qr.Notes})
	}
}

func AdminAddQuoteNote() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		qCol := database.OpenCollection("quote_requests")
		usersCol := database.OpenCollection("users")

		qid, err := bson.ObjectIDFromHex(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid quote request id"})
			return
		}

		var body dto.CreateQuoteNoteDTO
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		uidStr, _ := c.Get("userId")
		uid, err := bson.ObjectIDFromHex(uidStr.(string))
		if err != nil {
			c.JSON(401, gin.H{"error": "invalid auth user"})
			return
		}

		var user models.User
		if err := usersCol.FindOne(ctx, bson.M{"_id": uid}).Decode(&user); err != nil {
			c.JSON(401, gin.H{"error": "user not found"})
			return
		}

		now := time.Now().UTC()

		note := models.QuoteAdminNote{
			ID:          bson.NewObjectID(),
			AuthorID:    uid,
			AuthorEmail: user.Email,
			Content:     strings.TrimSpace(body.Content),
			CreatedAt:   now,
		}

		res, err := qCol.UpdateByID(ctx, qid, bson.M{
			"$push": bson.M{"notes": note},
			"$set":  bson.M{"updatedAt": now},
		})
		if err != nil {
			c.JSON(500, gin.H{"error": "failed to add note", "details": err.Error()})
			return
		}
		if res.MatchedCount == 0 {
			c.JSON(404, gin.H{"error": "quote request not found"})
			return
		}

		c.JSON(201, gin.H{"noteId": note.ID})
	}
}

func AdminAddQuoteNoteWithPDF() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		qCol := database.OpenCollection("quote_requests")
		usersCol := database.OpenCollection("users")

		qid, err := bson.ObjectIDFromHex(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid quote request id"})
			return
		}

		dataStr := c.PostForm("data")
		if dataStr == "" {
			c.JSON(400, gin.H{"error": "missing data"})
			return
		}
		var body dto.CreateQuoteNoteDTO
		if err := json.Unmarshal([]byte(dataStr), &body); err != nil {
			c.JSON(400, gin.H{"error": "invalid data json", "details": err.Error()})
			return
		}

		fh, err := c.FormFile("quotePdf")
		if err != nil {
			c.JSON(400, gin.H{"error": "missing quotePdf file"})
			return
		}
		if fh.Size > 10*1024*1024 {
			c.JSON(400, gin.H{"error": "quotePdf too large (max 10MB)"})
			return
		}

		uidStr, _ := c.Get("userId")
		uid, err := bson.ObjectIDFromHex(uidStr.(string))
		if err != nil {
			c.JSON(401, gin.H{"error": "invalid auth user"})
			return
		}

		var user models.User
		if err := usersCol.FindOne(ctx, bson.M{"_id": uid}).Decode(&user); err != nil {
			c.JSON(401, gin.H{"error": "user not found"})
			return
		}

		// Upload PDF to GCS
		GCSBucket := os.Getenv("GCS_BUCKET")
		wd, _ := os.Getwd()
		gcsClient, err := utils.NewGCSClient(ctx, filepath.Join(wd, "/gen-lang-client-0546647427-9649ea6bf52b.json"))
		if err != nil {
			c.JSON(500, gin.H{"error": "failed to init gcs client"})
			return
		}

		att, err := utils.UploadQuotePDFToGCS(ctx, gcsClient, GCSBucket, qid.Hex(), fh)
		if err != nil {
			c.JSON(400, gin.H{"error": "pdf upload failed", "details": err.Error()})
			return
		}

		now := time.Now().UTC()
		note := models.QuoteAdminNote{
			ID:          bson.NewObjectID(),
			AuthorID:    uid,
			AuthorEmail: user.Email,
			Content:     strings.TrimSpace(body.Content),
			CreatedAt:   now,
			QuotePDF:    att,
		}

		// Single update: push note + set status QUOTED + quotedAt + updatedAt
		res, err := qCol.UpdateByID(ctx, qid, bson.M{
			"$push": bson.M{"notes": note},
			"$set": bson.M{
				"status":    string(models.QuoteStatusQuoted),
				"quotedAt":  now,
				"updatedAt": now,
			},
		})
		if err != nil {
			// cleanup uploaded file best effort
			_ = utils.DeleteGCSObjects(ctx, gcsClient, GCSBucket, []string{att.ObjectName})
			c.JSON(500, gin.H{"error": "failed to add note", "details": err.Error()})
			return
		}
		if res.MatchedCount == 0 {
			_ = utils.DeleteGCSObjects(ctx, gcsClient, GCSBucket, []string{att.ObjectName})
			c.JSON(404, gin.H{"error": "quote request not found"})
			return
		}

		c.JSON(201, gin.H{"noteId": note.ID, "quotePdfUrl": att.PublicURL})
	}
}

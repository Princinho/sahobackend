package controllers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/princinho/sahobackend/database"
	"github.com/princinho/sahobackend/dto"
	"github.com/princinho/sahobackend/models"
	"github.com/princinho/sahobackend/utils"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ====== CreateQuoteRequest (public — no auth) ======================================================================
//
// POST /quote-requests
// Body: application/json
//
//	{
//	  "fullName": "Jean Dupont",
//	  "email": "jean@example.com",
//	  "phone": "+228 90 00 00 00",       // optional
//	  "country": "Togo",                  // optional
//	  "city": "Lomé",                     // optional
//	  "address": "Rue des Fleurs, 12",    // optional
//	  "message": "Je souhaite un devis.", // optional
//	  "items": [
//	    { "productId": "665f...", "quantity": 2 },
//	    { "productId": "665f...", "quantity": 1 }
//	  ]
//	}

func CreateQuoteRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var body dto.CreateQuoteRequestDTO
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 1) Convert all productId strings to ObjectIDs and build a quantity map
		productsCol := database.OpenCollection("products")
		productIDs := make([]bson.ObjectID, 0, len(body.Items))
		idToQuantity := make(map[bson.ObjectID]int, len(body.Items))

		for _, itemDTO := range body.Items {
			prodID, err := bson.ObjectIDFromHex(itemDTO.ProductID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":     "invalid productId",
					"productId": itemDTO.ProductID,
				})
				return
			}
			productIDs = append(productIDs, prodID)
			idToQuantity[prodID] = itemDTO.Quantity
		}

		// 2) Fetch all products in a single DB round-trip
		prodCursor, err := productsCol.Find(ctx, bson.M{"_id": bson.M{"$in": productIDs}})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer prodCursor.Close(ctx)

		productMap := make(map[bson.ObjectID]models.Product, len(productIDs))
		for prodCursor.Next(ctx) {
			var p models.Product
			if err := prodCursor.Decode(&p); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			productMap[p.Id] = p
		}
		if err := prodCursor.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 3) Build enriched items, verifying every requested product was found
		items := make([]models.QuoteRequestItem, 0, len(productIDs))
		for _, prodID := range productIDs {
			product, found := productMap[prodID]
			if !found {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":     "product not found",
					"productId": prodID.Hex(),
				})
				return
			}
			items = append(items, models.QuoteRequestItem{
				ProductID:   prodID,
				Quantity:    idToQuantity[prodID],
				ProductName: product.Name,
				ProductSlug: product.Slug,
				UnitPrice:   product.Price,
			})
		}

		now := time.Now().UTC()
		quote := models.QuoteRequest{
			FullName:  strings.TrimSpace(body.FullName),
			Email:     strings.TrimSpace(body.Email),
			Phone:     strings.TrimSpace(body.Phone),
			Country:   strings.TrimSpace(body.Country),
			City:      strings.TrimSpace(body.City),
			Address:   strings.TrimSpace(body.Address),
			Message:   strings.TrimSpace(body.Message),
			Items:     items,
			Status:    models.QuoteStatusNew,
			Notes:     []models.QuoteAdminNote{},
			CreatedAt: now,
			UpdatedAt: now,
		}

		col := database.OpenCollection("quote_requests")
		res, err := col.InsertOne(ctx, quote)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"id":      res.InsertedID,
			"message": "Your quote request has been submitted. We will get back to you shortly.",
		})
	}
}

func GetQuoteRequests() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		col := database.OpenCollection("quote_requests")
		maxLimit, defaultLimit := utils.GetDefaultQueryLimits()

		page := utils.ParseIntDefault(c.Query("page"), 1)
		limit := utils.ParseIntDefault(c.Query("limit"), defaultLimit)
		if page < 1 {
			page = 1
		}
		if limit < 1 || limit > maxLimit {
			limit = defaultLimit
		}
		skip := int64((page - 1) * limit)

		filter := bson.M{}
		if status := strings.TrimSpace(c.Query("status")); status != "" {
			filter["status"] = status
		}

		opts := options.Find().
			SetSkip(skip).
			SetLimit(int64(limit)).
			SetSort(bson.D{{Key: "createdAt", Value: -1}}) // newest first

		cursor, err := col.Find(ctx, filter, opts)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer cursor.Close(ctx)

		items := make([]models.QuoteRequest, 0)
		for cursor.Next(ctx) {
			var q models.QuoteRequest
			if err := cursor.Decode(&q); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			items = append(items, q)
		}
		if err := cursor.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		total, err := col.CountDocuments(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"items": items,
			"page":  page,
			"limit": limit,
			"total": total,
		})
	}
}

func GetQuoteRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		col := database.OpenCollection("quote_requests")

		id, err := bson.ObjectIDFromHex(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quote request id"})
			return
		}

		var quote models.QuoteRequest
		if err := col.FindOne(ctx, bson.M{"_id": id}).Decode(&quote); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "quote request not found"})
			return
		}

		c.JSON(http.StatusOK, quote)
	}
}

// ====== UpdateQuoteStatus (admin) ================================================================================================
//
// PATCH /admin/quote-requests/:id/status
// Body: { "status": "IN_PROGRESS" }
func UpdateQuoteStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		col := database.OpenCollection("quote_requests")

		id, err := bson.ObjectIDFromHex(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quote request id"})
			return
		}

		var body dto.UpdateQuoteStatusDTO
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Validate the status value
		allowed := map[string]bool{
			string(models.QuoteStatusNew):        true,
			string(models.QuoteStatusInProgress): true,
			string(models.QuoteStatusQuoted):     true,
			string(models.QuoteStatusRejected):   true,
			string(models.QuoteStatusClosed):     true,
		}
		if !allowed[body.Status] {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid status value",
				"allowed": []string{"NEW", "IN_PROGRESS", "QUOTED", "REJECTED", "CLOSED"},
			})
			return
		}

		set := bson.M{
			"status":    body.Status,
			"updatedAt": time.Now().UTC(),
		}

		// When moving to QUOTED, stamp quotedAt
		if body.Status == string(models.QuoteStatusQuoted) {
			now := time.Now().UTC()
			set["quotedAt"] = now
		}

		res, err := col.UpdateByID(ctx, id, bson.M{"$set": set})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if res.MatchedCount == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "quote request not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// ====== AddAdminNote (admin) ==========================================================================================================
//
// POST /admin/quote-requests/:id/notes
// Body: multipart/form-data
//
//	"data" : { "content": "Voici le devis en pièce jointe." }
//	"pdf"  : (optional PDF file)
//
// The logged-in admin's ID and email are expected to be set on the gin context
// by your auth middleware, e.g. c.Set("userID", "...") / c.Set("email", "...").
func AddQuoteNote() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		col := database.OpenCollection("quote_requests")

		quoteID, err := bson.ObjectIDFromHex(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quote request id"})
			return
		}

		// Parse JSON payload from the "data" form field
		dataStr := c.PostForm("data")
		if dataStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing data field"})
			return
		}

		var body dto.AddAdminNoteDTO
		if err := json.Unmarshal([]byte(dataStr), &body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid data json", "details": err.Error()})
			return
		}
		body.Content = strings.TrimSpace(body.Content)
		if body.Content == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
			return
		}

		// Grab admin identity from context (set by auth middleware)
		authorIDStr, _ := c.Get("userID")
		authorEmail, _ := c.Get("email")

		authorID, _ := bson.ObjectIDFromHex(authorIDStr.(string))

		note := models.QuoteAdminNote{
			ID:          bson.NewObjectID(),
			AuthorID:    authorID,
			AuthorEmail: authorEmail.(string),
			Content:     body.Content,
			CreatedAt:   time.Now().UTC(),
		}

		// Optional PDF attachment
		pdfFile, pdfErr := c.FormFile("pdf")
		if pdfErr == nil && pdfFile != nil {
			gcsClient, GCSBucket, err := utils.NewCloudClient(c)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create GCS client"})
				return
			}

			attachment, err := utils.UploadQuotePDFToCloud(ctx, gcsClient, GCSBucket, quoteID.Hex(), pdfFile)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			note.QuotePDF = attachment
		}

		// Push note into the notes array and update status + updatedAt
		update := bson.M{
			"$push": bson.M{"notes": note},
			"$set": bson.M{
				"updatedAt": time.Now().UTC(),
				// Automatically move status to IN_PROGRESS when first note is added
				// (only if still NEW — won't overwrite a more advanced status)
			},
		}

		res, err := col.UpdateOne(ctx,
			bson.M{
				"_id":    quoteID,
				"status": models.QuoteStatusNew, // only auto-advance from NEW
			},
			bson.M{
				"$push": bson.M{"notes": note},
				"$set": bson.M{
					"status":    models.QuoteStatusInProgress,
					"updatedAt": time.Now().UTC(),
				},
			},
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// If the quote wasn't NEW, just push the note without changing status
		if res.MatchedCount == 0 {
			res2, err := col.UpdateByID(ctx, quoteID, update)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if res2.MatchedCount == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "quote request not found"})
				return
			}
		}

		c.JSON(http.StatusCreated, note)
	}
}

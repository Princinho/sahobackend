package controllers

import (
	"encoding/json"
	"net/http"
	"regexp"
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

// ====== CreateProductRequest (public — no auth) ================================================================
// POST /product-requests
// multipart/form-data:
//   - data: JSON string (CreateProductRequestDTO)
//   - image: optional file (jpg/png/webp/jpeg) or even pdf
func CreateProductRequest(ImageOrPdfValidator *utils.FileValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		dataStr := c.PostForm("data")
		if dataStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing data field"})
			return
		}

		var body dto.CreateProductRequestDTO
		if err := json.Unmarshal([]byte(dataStr), &body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid data json", "details": err.Error()})
			return
		}

		body.FullName = strings.TrimSpace(body.FullName)
		body.Email = strings.TrimSpace(body.Email)
		body.Description = strings.TrimSpace(body.Description)

		if body.FullName == "" || body.Email == "" || body.Description == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "fullName, email and description are required"})
			return
		}
		if body.Quantity <= 0 {
			body.Quantity = 1
		}

		now := time.Now().UTC()
		req := models.ProductRequest{
			Id: bson.NewObjectID(),

			FullName: body.FullName,
			Email:    body.Email,
			Phone:    strings.TrimSpace(body.Phone),
			Company:  strings.TrimSpace(body.Company),

			Country: strings.TrimSpace(body.Country),
			City:    strings.TrimSpace(body.City),

			Description:     body.Description,
			Quantity:        body.Quantity,
			DesiredDeadline: body.DesiredDeadline,
			Budget:          strings.TrimSpace(body.Budget),
			ReferenceURL:    strings.TrimSpace(body.ReferenceURL),

			Status:    models.ProductRequestStatusNew,
			Notes:     []models.ProductRequestAdminNote{},
			CreatedAt: now,
			UpdatedAt: now,
		}

		// Optional reference image/file
		file, errFile := c.FormFile("image")
		if errFile == nil && file != nil {
			if _, err := ImageOrPdfValidator.ValidateFile(file); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			gcsClient, bucket, err := utils.NewGCSClient(c)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create GCS client"})
				return
			}
			att, err := utils.UploadProductRequestFileToGCS(ctx, gcsClient, bucket, req.Id.Hex(), file)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			req.ReferenceImage = att
		}

		col := database.OpenCollection("product_requests")
		_, err := col.InsertOne(ctx, req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"id":      req.Id,
			"message": "Your product request has been submitted. We will get back to you shortly.",
		})
	}
}

// ====== GetProductRequests (admin) ==========================================================================================
// GET /admin/product-requests?page=1&limit=20&status=NEW&email=a@b.com&q=...
func GetProductRequests() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		col := database.OpenCollection("product_requests")

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
		if email := strings.TrimSpace(c.Query("email")); email != "" {
			filter["email"] = email
		}
		if q := strings.TrimSpace(c.Query("q")); q != "" {
			escaped := regexp.QuoteMeta(q)
			filter["$or"] = []bson.M{
				{"fullName": bson.M{"$regex": escaped, "$options": "i"}},
				{"email": bson.M{"$regex": escaped, "$options": "i"}},
				{"company": bson.M{"$regex": escaped, "$options": "i"}},
				{"description": bson.M{"$regex": escaped, "$options": "i"}},
			}
		}

		opts := options.Find().
			SetSkip(skip).
			SetLimit(int64(limit)).
			SetSort(bson.D{{Key: "createdAt", Value: -1}})

		cursor, err := col.Find(ctx, filter, opts)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer cursor.Close(ctx)

		items := make([]models.ProductRequest, 0)
		for cursor.Next(ctx) {
			var r models.ProductRequest
			if err := cursor.Decode(&r); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			items = append(items, r)
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

// ====== GetProductRequest (admin) ============================================================================================
// GET /admin/product-requests/:id
func GetProductRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		col := database.OpenCollection("product_requests")

		id, err := bson.ObjectIDFromHex(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product request id"})
			return
		}

		var req models.ProductRequest
		if err := col.FindOne(ctx, bson.M{"_id": id}).Decode(&req); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "product request not found"})
			return
		}

		c.JSON(http.StatusOK, req)
	}
}

// ====== UpdateProductRequestStatus (admin) ==========================================================================
// PATCH /admin/product-requests/:id/status
// Body: { "status": "IN_PROGRESS" }
func UpdateProductRequestStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		col := database.OpenCollection("product_requests")

		id, err := bson.ObjectIDFromHex(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product request id"})
			return
		}

		var body dto.UpdateProductRequestStatusDTO
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		allowed := map[string]bool{
			string(models.ProductRequestStatusNew):        true,
			string(models.ProductRequestStatusInProgress): true,
			string(models.ProductRequestStatusAnswered):   true,
			string(models.ProductRequestStatusRejected):   true,
			string(models.ProductRequestStatusClosed):     true,
		}
		if !allowed[body.Status] {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid status value",
				"allowed": []string{"NEW", "IN_PROGRESS", "ANSWERED", "REJECTED", "CLOSED"},
			})
			return
		}

		set := bson.M{
			"status":    body.Status,
			"updatedAt": time.Now().UTC(),
		}

		if body.Status == string(models.ProductRequestStatusAnswered) {
			now := time.Now().UTC()
			set["answeredAt"] = now
		}

		res, err := col.UpdateByID(ctx, id, bson.M{"$set": set})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if res.MatchedCount == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "product request not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// ====== AddProductRequestNote (admin) ====================================================================================
// POST /admin/product-requests/:id/notes
// multipart/form-data:
//   - data: { "content": "..." }
//   - file: optional attachment (pdf/image)
func AddProductRequestNote() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		col := database.OpenCollection("product_requests")

		reqID, err := bson.ObjectIDFromHex(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product request id"})
			return
		}

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

		authorIDStr, _ := c.Get("userID")
		authorEmail, _ := c.Get("email")
		authorID, _ := bson.ObjectIDFromHex(authorIDStr.(string))

		note := models.ProductRequestAdminNote{
			ID:          bson.NewObjectID(),
			AuthorID:    authorID,
			AuthorEmail: authorEmail.(string),
			Content:     body.Content,
			CreatedAt:   time.Now().UTC(),
		}

		// optional attachment
		fh, ferr := c.FormFile("file")
		if ferr == nil && fh != nil {
			gcsClient, bucket, err := utils.NewGCSClient(c)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create GCS client"})
				return
			}
			att, err := utils.UploadProductRequestFileToGCS(ctx, gcsClient, bucket, reqID.Hex(), fh)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			note.Attachment = att
		}

		// auto-advance NEW -> IN_PROGRESS if adding first note
		res, err := col.UpdateOne(ctx,
			bson.M{"_id": reqID, "status": models.ProductRequestStatusNew},
			bson.M{
				"$push": bson.M{"notes": note},
				"$set": bson.M{
					"status":    models.ProductRequestStatusInProgress,
					"updatedAt": time.Now().UTC(),
				},
			},
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// if not NEW, just push note + updatedAt
		if res.MatchedCount == 0 {
			res2, err := col.UpdateByID(ctx, reqID, bson.M{
				"$push": bson.M{"notes": note},
				"$set":  bson.M{"updatedAt": time.Now().UTC()},
			})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if res2.MatchedCount == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "product request not found"})
				return
			}
		}

		c.JSON(http.StatusCreated, note)
	}
}

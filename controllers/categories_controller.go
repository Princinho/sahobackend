package controllers

import (
	"encoding/json"
	"mime/multipart"
	"net/http"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/gin-gonic/gin"
	"github.com/princinho/sahobackend/database"
	"github.com/princinho/sahobackend/dto"
	"github.com/princinho/sahobackend/models"
	"github.com/princinho/sahobackend/utils"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ─── helpers ────────────────────────────────────────────────────────────────

// func newGCSClient(c *gin.Context) (*storage.Client, string, bool) {
// 	GCSBucket := os.Getenv("GCS_BUCKET")
// 	wd, err := os.Getwd()
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get working directory"})
// 		return nil, "", false
// 	}
// 	client, err := utils.NewGCSClient(
// 		c.Request.Context(),
// 		filepath.Join(wd, "/gen-lang-client-0546647427-9649ea6bf52b.json"),
// 	)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create GCS client"})
// 		return nil, "", false
// 	}
// 	return client, GCSBucket, true
// }

// ─── AddCategory ────────────────────────────────────────────────────────────
//
// Accepts a multipart/form-data request:
//   - "data"  : JSON  { name, slug?, description?, isActive? }
//   - "image" : file  (optional)

func AddCategory() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		col := database.OpenCollection("categories")

		// 1) Parse JSON payload
		jsonData := c.PostForm("data")
		if jsonData == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing data field"})
			return
		}

		var body dto.CreateCategoryDTO
		if err := json.Unmarshal([]byte(jsonData), &body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid data json", "details": err.Error()})
			return
		}

		body.Name = strings.TrimSpace(body.Name)
		if body.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
			return
		}

		// Auto-generate slug if not provided
		body.Slug = strings.TrimSpace(body.Slug)
		if body.Slug == "" {
			body.Slug = utils.GenerateSlug(body.Name)
		}

		// 2) Upload image (optional)
		var imageUrl string
		if file, err := c.FormFile("image"); err == nil && file != nil {
			gcsClient, gcsBucket, gsError := utils.NewGCSClient(c)
			if gsError != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to connect GS Client", "details": gsError.Error()})
			}
			urls, err := utils.UploadImagesToGCSAndGetPublicURLs(
				ctx, gcsClient, gcsBucket, body.Slug, []*multipart.FileHeader{file},
			)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			imageUrl = urls[0]
		}

		// 3) Build document
		doc := models.Category{
			Name:        body.Name,
			Slug:        body.Slug,
			Description: strings.TrimSpace(body.Description),
			IsActive:    body.IsActive,
			ImageUrl:    imageUrl,
		}

		res, err := col.InsertOne(ctx, doc)
		if err != nil {
			if utils.IsDuplicateKey(err) {
				c.JSON(http.StatusConflict, gin.H{"error": "slug already exists", "field": "slug"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"id": res.InsertedID})
	}
}

func GetCategories() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		col := database.OpenCollection("categories")

		page := utils.ParseIntDefault(c.Query("page"), 1)
		limit := utils.ParseIntDefault(c.Query("limit"), 50)
		if page < 1 {
			page = 1
		}
		if limit < 1 {
			limit = 50
		}
		if limit > 200 {
			limit = 200
		}
		skip := int64((page - 1) * limit)

		q := strings.TrimSpace(c.Query("q"))
		filter := bson.M{}
		if q != "" {
			filter["name"] = bson.M{"$regex": q, "$options": "i"}
		}

		// Optional isActive filter
		if b, err := utils.ParseBoolQuery(c.Query("isActive")); err == nil && b != nil {
			filter["isActive"] = *b
		}

		opts := options.Find().
			SetSkip(skip).
			SetLimit(int64(limit)).
			SetSort(bson.D{{Key: "name", Value: 1}})

		cursor, err := col.Find(ctx, filter, opts)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer cursor.Close(ctx)

		items := make([]models.Category, 0)
		for cursor.Next(ctx) {
			var cat models.Category
			if err := cursor.Decode(&cat); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			items = append(items, cat)
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

// ─── GetCategory ─────────────────────────────────────────────────────────────
// Supports lookup by :id (ObjectID hex) or :slug

func GetCategory() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		col := database.OpenCollection("categories")

		idHex := strings.TrimSpace(c.Param("id"))
		slug := strings.TrimSpace(c.Param("slug"))

		if idHex == "" && slug == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no id or slug provided"})
			return
		}

		var filter bson.M

		if idHex != "" {
			id, err := bson.ObjectIDFromHex(idHex)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category id"})
				return
			}
			filter = bson.M{"_id": id}
		} else {
			filter = bson.M{"slug": slug}
		}

		var cat models.Category
		if err := col.FindOne(ctx, filter).Decode(&cat); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
			return
		}

		c.JSON(http.StatusOK, cat)
	}
}

// ─── UpdateCategory ──────────────────────────────────────────────────────────
//
// Accepts multipart/form-data:
//   - "data"  : JSON  { name?, slug?, description?, isActive?, removeImage? }
//   - "image" : file  (optional — replaces current image)

func UpdateCategory() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		col := database.OpenCollection("categories")

		idHex := c.Param("id")
		id, err := bson.ObjectIDFromHex(idHex)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category id"})
			return
		}

		dataStr := c.PostForm("data")
		if dataStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing data field"})
			return
		}

		var body dto.UpdateCategoryDTO
		if err := json.Unmarshal([]byte(dataStr), &body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid data json", "details": err.Error()})
			return
		}

		// 1) Load existing category (need current imageUrl for deletion)
		var existing models.Category
		if err := col.FindOne(ctx, bson.M{"_id": id}).Decode(&existing); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
			return
		}

		set := bson.M{}

		if body.Name != nil {
			v := strings.TrimSpace(*body.Name)
			if v == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "name cannot be empty"})
				return
			}
			set["name"] = v
		}
		if body.Slug != nil {
			v := strings.TrimSpace(*body.Slug)
			if v == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "slug cannot be empty"})
				return
			}
			set["slug"] = v
		}
		if body.Description != nil {
			set["description"] = strings.TrimSpace(*body.Description)
		}
		if body.IsActive != nil {
			set["isActive"] = *body.IsActive
		}

		// 2) Determine the slug to use for GCS path (prefer new slug, fall back to current)
		uploadSlug := existing.Slug
		if s, ok := set["slug"]; ok {
			uploadSlug = s.(string)
		}

		// 3) Handle image replacement / removal
		newImageUrl := ""
		newFile, fileErr := c.FormFile("image")
		hasNewFile := fileErr == nil && newFile != nil

		var gcsClient *storage.Client
		var gcsBucket string

		if hasNewFile {
			gcsClient, gcsBucket, err = utils.NewGCSClient(c)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to open GS Client", "details": err.Error()})
			}
			urls, err := utils.UploadImagesToGCSAndGetPublicURLs(
				ctx, gcsClient, gcsBucket, uploadSlug, []*multipart.FileHeader{newFile},
			)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			newImageUrl = urls[0]
			set["imageUrl"] = newImageUrl
		}

		if len(set) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no updates provided"})
			return
		}

		result, err := col.UpdateByID(ctx, id, bson.M{"$set": set})
		if err != nil {
			// Roll back: delete newly uploaded image (if any)
			if newImageUrl != "" && gcsClient != nil {
				if objName, e := utils.ObjectNameFromGCSPublicURL(gcsBucket, newImageUrl); e == nil {
					_ = utils.DeleteGCSObjects(ctx, gcsClient, gcsBucket, []string{objName})
				}
			}
			if utils.IsDuplicateKey(err) {
				c.JSON(http.StatusConflict, gin.H{"error": "slug already exists", "field": "slug"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if result.MatchedCount == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
			return
		}

		// 5) DB OK → delete old image from GCS if replaced or removed
		if (hasNewFile) && existing.ImageUrl != "" && gcsClient != nil {
			if objName, e := utils.ObjectNameFromGCSPublicURL(gcsBucket, existing.ImageUrl); e == nil {
				_ = utils.DeleteGCSObjects(ctx, gcsClient, gcsBucket, []string{objName})
			}
		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// ─── DeleteCategory ──────────────────────────────────────────────────────────

func DeleteCategory() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		col := database.OpenCollection("categories")

		idHex := c.Param("id")
		id, err := bson.ObjectIDFromHex(idHex)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category id"})
			return
		}

		var existing models.Category
		if err := col.FindOne(ctx, bson.M{"_id": id}).Decode(&existing); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
			return
		}

		res, err := col.DeleteOne(ctx, bson.M{"_id": id})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if res.DeletedCount == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
			return
		}

		// Clean up GCS image
		if existing.ImageUrl != "" {
			gcsClient, gcsBucket, err := utils.NewGCSClient(c)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "failed to open GS Client", "details": err.Error()})
			}
			if objName, e := utils.ObjectNameFromGCSPublicURL(gcsBucket, existing.ImageUrl); e == nil {
				_ = utils.DeleteGCSObjects(ctx, gcsClient, gcsBucket, []string{objName})
			}

		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

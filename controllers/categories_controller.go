package controllers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/princinho/sahobackend/database"
	"github.com/princinho/sahobackend/dto"
	"github.com/princinho/sahobackend/models"
	"github.com/princinho/sahobackend/utils"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func AddCategory() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		col := database.OpenCollection("categories")

		var body dto.CreateCategoryDTO
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		body.Name = strings.TrimSpace(body.Name)
		body.Slug = strings.TrimSpace(body.Slug)

		doc := models.Category{
			Name: body.Name,
			Slug: body.Slug,
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

		// pagination (optional)
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

func GetCategory() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		col := database.OpenCollection("categories")

		idHex := c.Param("id")
		slug := strings.TrimSpace(c.Param("slug"))
		if idHex == "" && slug == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No Id or slug provided"})
		}
		if idHex != "" {
			id, err := bson.ObjectIDFromHex(idHex)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category id"})
				return
			}

			var cat models.Category
			if err := col.FindOne(ctx, bson.M{"_id": id}).Decode(&cat); err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
				return
			}

			c.JSON(http.StatusOK, cat)
		}

		if slug != "" {
			var cat models.Category
			if err := col.FindOne(ctx, bson.M{"slug": slug}).Decode(&cat); err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
				return
			}

			c.JSON(http.StatusOK, cat)
		}

	}
}

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

		var body dto.UpdateCategoryDTO
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

		if len(set) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no updates provided"})
			return
		}

		res, err := col.UpdateByID(ctx, id, bson.M{"$set": set})
		if err != nil {
			if utils.IsDuplicateKey(err) {
				c.JSON(http.StatusConflict, gin.H{"error": "slug already exists", "field": "slug"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if res.MatchedCount == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

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

		res, err := col.DeleteOne(ctx, bson.M{"_id": id})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if res.DeletedCount == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

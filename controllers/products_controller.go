package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"cloud.google.com/go/storage"
	"github.com/gin-gonic/gin"
	"github.com/princinho/sahobackend/database"
	"github.com/princinho/sahobackend/dto"
	"github.com/princinho/sahobackend/models"
	"github.com/princinho/sahobackend/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type App struct {
	GCSClient   *storage.Client
	GCSBucket   string
	ProductsCol *mongo.Collection
}

func GetAllProducts() gin.HandlerFunc {
	collection := database.OpenCollection("products")
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c, 10*time.Second)
		defer cancel()

		var products []models.Product
		cursor, err := collection.Find(ctx, bson.M{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer cursor.Close(ctx)
		if err = cursor.All(ctx, &products); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse movies"})
			return
		}
		c.JSON(http.StatusOK, products)
	}
}

func GetFeaturedProducts() gin.HandlerFunc {
	collection := database.OpenCollection("products")
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c, 10*time.Second)
		defer cancel()

		var movies []models.Product
		cursor, err := collection.Find(ctx, bson.M{"isTrending": true})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer cursor.Close(ctx)
		if err = cursor.All(ctx, &movies); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse movies"})
			return
		}
		c.JSON(http.StatusOK, movies)
	}
}

// func AddProduct() gin.HandlerFunc {
// 	collection := database.OpenCollection("products")
// 	return func(c *gin.Context) {
// 		ctx, cancel := context.WithTimeout(c, 100*time.Second)
// 		defer cancel()
// 		var product dto.CreateProductDTO
// 		if err := c.ShouldBindJSON(&product); err != nil {
// 			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
// 			return
// 		}

//			validate := validator.New()
//			if err := validate.Struct(product); err != nil {
//				c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": err.Error()})
//				return
//			}
//			result, err := collection.InsertOne(ctx, product)
//			if err != nil {
//				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add movie"})
//				return
//			}
//			c.JSON(http.StatusCreated, result)
//		}
//	}
func TestUpload() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.PostForm("name")
		email := c.PostForm("email")

		// Multipart form
		form, err := c.MultipartForm()
		if err != nil {
			c.String(http.StatusBadRequest, "get form err: %s", err.Error())
			return
		}
		files := form.File["files"]

		for _, file := range files {
			filename := filepath.Base(file.Filename)
			if err := c.SaveUploadedFile(file, filename); err != nil {
				c.String(http.StatusBadRequest, "upload file err: %s", err.Error())
				return
			}
		}

		c.String(
			http.StatusOK,
			"Uploaded successfully %d files with fields name=%s and email=%s.",
			len(files),
			name,
			email,
		)
	}

}

func AddProduct() gin.HandlerFunc {
	return func(c *gin.Context) {
		collection := database.OpenCollection("products")

		GCSBucket := os.Getenv("GCS_BUCKET")
		wd, err := os.Getwd()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to get working directory"})
		}
		GCSClient, err := utils.NewGCSClient(c.Request.Context(), filepath.Join(wd, "/gen-lang-client-0546647427-9649ea6bf52b.json"))
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to create GCS client"})
		}
		jsonData := c.PostForm("data")
		if jsonData == "" {
			c.JSON(400, gin.H{"error": "missing data"})
			return
		}

		var dto dto.CreateProductDTO
		if err := json.Unmarshal([]byte(jsonData), &dto); err != nil {
			c.JSON(400, gin.H{"error": "invalid data json"})
			return
		}

		form, err := c.MultipartForm()
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid multipart form"})
			return
		}
		files := form.File["images"]

		imageUrls, err := utils.UploadImagesToGCSAndGetPublicURLs(
			c.Request.Context(),
			GCSClient,
			GCSBucket,
			dto.Slug,
			files,
		)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		categoryIdBsons, err := utils.StringsToObjectIDs(dto.CategoryIds)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
		}
		// Insert product with imageUrls
		product := models.Product{
			Name:            dto.Name,
			Slug:            dto.Slug,
			Price:           dto.Price,
			Quantity:        dto.Quantity,
			ImageUrls:       imageUrls,
			CategoryIds:     categoryIdBsons,
			Materials:       dto.Materials,
			Colors:          dto.Colors,
			Description:     dto.Description,
			DescriptionFull: dto.DescriptionFull,
			Dimensions:      dto.Dimensions,
			Weight:          dto.Weight,
			IsTrending:      dto.IsTrending,
			IsDisabled:      dto.IsDisabled,
		}

		_, err = collection.InsertOne(c.Request.Context(), product)
		if err != nil {
			if utils.IsDuplicateKey(err) {
				c.JSON(409, gin.H{
					"error": "slug already exists",
					"field": "slug",
				})
				return
			}
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(201, product)
	}

}

func UpdateProduct() gin.HandlerFunc {
	return func(c *gin.Context) {
		collection := database.OpenCollection("products")

		GCSBucket := os.Getenv("GCS_BUCKET")
		wd, err := os.Getwd()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to get working directory"})
		}
		GCSClient, err := utils.NewGCSClient(c.Request.Context(), filepath.Join(wd, "/gen-lang-client-0546647427-9649ea6bf52b.json"))
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to create GCS client"})
		}
		jsonData := c.PostForm("data")
		if jsonData == "" {
			c.JSON(400, gin.H{"error": "missing data"})
			return
		}

		var dto dto.CreateProductDTO
		if err := json.Unmarshal([]byte(jsonData), &dto); err != nil {
			c.JSON(400, gin.H{"error": "invalid data json"})
			return
		}

		form, err := c.MultipartForm()
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid multipart form"})
			return
		}
		files := form.File["images"]

		imageUrls, err := utils.UploadImagesToGCSAndGetPublicURLs(
			c.Request.Context(),
			GCSClient,
			GCSBucket,
			dto.Slug,
			files,
		)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		categoryIdBsons, err := utils.StringsToObjectIDs(dto.CategoryIds)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
		}
		// Insert product with imageUrls
		product := models.Product{
			Name:            dto.Name,
			Slug:            dto.Slug,
			Price:           dto.Price,
			Quantity:        dto.Quantity,
			ImageUrls:       imageUrls,
			CategoryIds:     categoryIdBsons,
			Materials:       dto.Materials,
			Colors:          dto.Colors,
			Description:     dto.Description,
			DescriptionFull: dto.DescriptionFull,
			Dimensions:      dto.Dimensions,
			Weight:          dto.Weight,
			IsTrending:      dto.IsTrending,
			IsDisabled:      dto.IsDisabled,
		}

		_, err = collection.InsertOne(c.Request.Context(), product)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(201, product)
	}

}

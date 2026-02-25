package controllers

import (
	"encoding/json"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/gin-gonic/gin"
	"github.com/princinho/sahobackend/database"
	"github.com/princinho/sahobackend/dto"
	"github.com/princinho/sahobackend/models"
	"github.com/princinho/sahobackend/utils"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type App struct {
	GCSClient   *storage.Client
	GCSBucket   string
	ProductsCol *mongo.Collection
}

func GetProducts() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		categorySlug := strings.TrimSpace(c.Query("category"))
		page := utils.ParseIntDefault(c.Query("page"), 1)
		limit := utils.ParseIntDefault(c.Query("limit"), 20)
		if page < 1 {
			page = 1
		}
		if limit < 1 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}
		skip := int64((page - 1) * limit)

		// Optional sorting
		sortParam := strings.TrimSpace(c.Query("sort"))
		sortDoc := bson.D{{Key: "name", Value: 1}}
		switch sortParam {
		case "price_asc":
			sortDoc = bson.D{{Key: "price", Value: 1}}
		case "price_desc":
			sortDoc = bson.D{{Key: "price", Value: -1}}
		case "stock_asc":
			sortDoc = bson.D{{Key: "createdAt", Value: 1}}
		case "stock_desc":
			sortDoc = bson.D{{Key: "createdAt", Value: -1}}
		case "":
			sortDoc = bson.D{{Key: "name", Value: 1}}
		}

		productsCol := database.OpenCollection("products")
		categoriesCol := database.OpenCollection("categories")

		// Build filter
		filter := bson.M{}

		// If category slug is provided: resolve it -> ObjectID, then filter products by it
		if categorySlug != "" {
			var cat models.Category
			if err := categoriesCol.FindOne(ctx, bson.M{"slug": categorySlug}).Decode(&cat); err != nil {
				// slug not found => return empty list (or 404; your choice)
				c.JSON(http.StatusOK, gin.H{
					"items": []models.Product{},
					"page":  page,
					"limit": limit,
					"total": 0,
				})
				return
			}

			// Products where categoryIds array contains cat.ID
			filter["categoryIds"] = bson.M{"$in": bson.A{cat.Id}}
		}
		if b, err := utils.ParseBoolQuery(c.Query("isTrending")); err == nil && b != nil {
			filter["isTrending"] = *b
		}
		filter["isDisabled"] = false
		if b, err := utils.ParseBoolQuery(c.Query("isDisabled")); err == nil && b != nil {
			filter["isDisabled"] = *b
		}
		findOpts := options.Find().
			SetSkip(skip).
			SetLimit(int64(limit)).
			SetSort(sortDoc)

		cursor, err := productsCol.Find(ctx, filter, findOpts)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer cursor.Close(ctx)

		products := make([]models.Product, 0)
		for cursor.Next(ctx) {
			var p models.Product
			if err := cursor.Decode(&p); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			products = append(products, p)
		}
		if err := cursor.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Total count for pagination UI
		total, err := productsCol.CountDocuments(ctx, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"items": products,
			"page":  page,
			"limit": limit,
			"total": total,
			// helpful for debugging on frontend:
			"category": categorySlug,
			"sort":     sortParam,
			"ts":       time.Now().UTC().Format(time.RFC3339),
		})
	}
}

func AddProduct() gin.HandlerFunc {
	return func(c *gin.Context) {
		collection := database.OpenCollection("products")

		GCSClient, GSBucket, err := utils.NewGCSClient(c)
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
		dto.Slug = utils.GenerateSlug(dto.Name)
		form, err := c.MultipartForm()
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid multipart form"})
			return
		}
		files := form.File["images"]

		imageUrls, err := utils.UploadImagesToGCSAndGetPublicURLs(
			c.Request.Context(),
			GCSClient,
			GSBucket,
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
		// parse id
		idHex := c.Param("id")
		prodID, err := bson.ObjectIDFromHex(idHex)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
			return
		}
		collection := database.OpenCollection("products")
		GCSClient, bucket, err := utils.NewGCSClient(c)

		dataStr := c.PostForm("data")
		if dataStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing data"})
			return
		}

		var dto dto.UpdateProductDTO
		if err := json.Unmarshal([]byte(dataStr), &dto); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid data json", "details": err.Error()})
			return
		}
		log.Println("Init Ok")

		ctx := c.Request.Context()

		// 1) Load product (need current imageUrls)
		var product models.Product
		if err := collection.FindOne(ctx, bson.M{"_id": prodID}).Decode(&product); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}

		// 2) Filter out imageUrls that don't belong to product
		imagesToDelete := utils.IntersectStrings(dto.RemovedImagesUrls, product.ImageUrls)

		// 5) Collect new image files
		var newFiles []*multipart.FileHeader
		if form, err := c.MultipartForm(); err == nil && form != nil {
			newFiles = form.File["images"]
		}
		maxProdImages, err := strconv.Atoi(os.Getenv("MAX_PROD_IMAGES"))
		if err != nil {
			maxProdImages = 4
		}
		totalImageCount := len(product.ImageUrls) - len(imagesToDelete) + len(newFiles)
		if totalImageCount > maxProdImages {
			c.JSON(400, gin.H{"error": fmt.Sprintf("Max %v images", maxProdImages)})
			return
		}
		log.Println("Parsing Images OK")
		// 6) Upload new images (if any)
		newObjectNames := []string{} // for cleanup if DB update fails
		var imageUrls []string
		if len(newFiles) > 0 {
			urls, err := utils.UploadImagesToGCSAndGetPublicURLs(
				c.Request.Context(),
				GCSClient,
				bucket,
				product.Slug,
				newFiles,
			)
			imageUrls = urls
			if err != nil {
				c.JSON(400, gin.H{"image-upload-error": err.Error()})
				return
			}
		}

		for _, imageUrl := range imageUrls {
			objName, _ := utils.ObjectNameFromGCSPublicURL(bucket, imageUrl)
			newObjectNames = append(newObjectNames, objName)
		}
		log.Println("Images Uploading Ok")
		update := bson.M{}
		set := bson.M{}

		if dto.Name != nil {
			set["name"] = *dto.Name
		}
		if dto.Price != nil {
			set["price"] = *dto.Price
		}
		if dto.Quantity != nil {
			set["quantity"] = *dto.Quantity
		}
		if dto.Slug != nil {
			set["slug"] = *dto.Slug
		}
		if dto.Description != nil {
			set["description"] = *dto.Description
		}
		if dto.DescriptionFull != nil {
			set["descriptionFull"] = *dto.DescriptionFull
		}
		if dto.Materials != nil {
			set["materials"] = *dto.Materials
		}
		if dto.Colors != nil {
			set["colors"] = *dto.Colors
		}
		if dto.Dimensions != nil {
			set["dimensions"] = *dto.Dimensions
		}
		if dto.Weight != nil {
			set["weight"] = *dto.Weight
		}
		if dto.IsTrending != nil {
			set["isTrending"] = *dto.IsTrending
		}
		if dto.IsDisabled != nil {
			set["isDisabled"] = *dto.IsDisabled
		}

		mergedImageUrls := utils.MergeImageUrlsArrays(product.ImageUrls, imagesToDelete, imageUrls)
		if len(imagesToDelete) > 0 || len(imageUrls) > 0 {
			set["imageUrls"] = mergedImageUrls
		}

		if len(set) > 0 {
			update["$set"] = set
		}

		if len(update) == 0 {
			c.JSON(400, gin.H{"error": "no updates provided"})
			return
		}

		// 4) Update DB first
		_, err = collection.UpdateByID(ctx, prodID, update)

		if err != nil {
			// 5) Delete new images from GCS
			if len(newObjectNames) > 0 {
				_ = utils.DeleteGCSObjects(ctx, GCSClient, bucket, newObjectNames)
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db update failed", "details": err.Error()})
			return
		}
		log.Println("Db Update Ok")

		// 5) DB update went fine. Delete old images from GCS
		if len(imagesToDelete) > 0 {
			objectNames := make([]string, 0, len(imagesToDelete))
			for _, imageUrl := range imagesToDelete {
				obj, err := utils.ObjectNameFromGCSPublicURL(bucket, imageUrl)
				if err == nil {
					objectNames = append(objectNames, obj)
				}
			}
			_ = utils.DeleteGCSObjects(ctx, GCSClient, bucket, objectNames)
		}

		log.Println("Deleting images Ok")

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

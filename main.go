package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/princinho/sahobackend/controllers"
	"github.com/princinho/sahobackend/database"
	"github.com/princinho/sahobackend/middleware"
	"github.com/princinho/sahobackend/utils"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}
	//seeding admin user
	ctx := context.Background()
	usersCol := database.OpenCollection("users")
	if err := utils.SeedAdminUser(ctx, usersCol); err != nil {
		log.Fatal(err)
	}

	r := gin.New()
	v := utils.NewPDFOrImageValidator()

	origins := os.Getenv("ALLOWED_ORIGINS")
	log.Printf("Env config origins list: %q", origins)
	allowedOrigins := map[string]bool{}
	for _, origin := range strings.Split(origins, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowedOrigins[origin] = true
		}
	}
	log.Printf("Allowed origins: %v", allowedOrigins)
	r.Use(cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			result := allowedOrigins[origin]
			log.Printf("CORS check — origin: %q, allowed: %v", origin, result)
			return allowedOrigins[origin]
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	r.POST("/auth/login", controllers.Login())
	r.POST("/auth/refresh", controllers.Refresh())

	r.GET("/products", controllers.GetProducts())
	r.GET("/categories", controllers.GetCategories())
	r.GET("/categories/:id", controllers.GetCategory())
	r.GET("/categories/slug/:slug", controllers.GetCategory())
	r.POST("/quote-requests", controllers.CreateQuoteRequest())
	r.POST("/product-requests", controllers.CreateProductRequest(v))

	admin := r.Group("/admin")
	admin.Use(middleware.AuthMiddleware())
	{
		admin.POST("/products/add", controllers.AddProduct())
		admin.PATCH("/products/update/:id", controllers.UpdateProduct())

		admin.POST("/categories", controllers.AddCategory())
		admin.PATCH("/categories/:id", controllers.UpdateCategory())
		admin.DELETE("/categories/:id", controllers.DeleteCategory())

		admin.GET("/quote-requests", controllers.GetQuoteRequests())
		admin.GET("/quote-requests/:id", controllers.GetQuoteRequest())
		admin.PATCH("/quote-requests/:id/status", controllers.UpdateQuoteStatus())
		admin.POST("/quote-requests/:id/notes", controllers.AddQuoteNote())

		admin.GET("/product-requests", controllers.GetProductRequests())
		admin.GET("/product-requests/:id", controllers.GetProductRequest())
		admin.PATCH("/product-requests/:id/status", controllers.UpdateProductRequestStatus())
		admin.POST("/product-requests/:id/notes", controllers.AddProductRequestNote())
		admin.POST("/users", controllers.CreateUser())
		admin.POST("/users/me/password", controllers.ChangeMyPassword())
	}
	// Start server on port 8080 (default)
	// Server will listen on 0.0.0.0:8080 (localhost:8080 on Windows)
	r.Run()
}

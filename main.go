package main

import (
	"context"
	"log"
	"net/http"
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

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:8081"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

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
	}
	// Start server on port 8080 (default)
	// Server will listen on 0.0.0.0:8080 (localhost:8080 on Windows)
	r.Run()
}

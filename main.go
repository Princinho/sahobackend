package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/princinho/sahobackend/controllers"
	"github.com/princinho/sahobackend/database"
	"github.com/princinho/sahobackend/middleware"
	"github.com/princinho/sahobackend/utils"
)

func main() {
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

	r.GET("/auth/login", controllers.Login())
	r.GET("/auth/refresh", controllers.Refresh())
	r.GET("/products", controllers.GetProducts())
	r.GET("/categories", controllers.GetCategories())

	admin := r.Group("/admin")
	admin.Use(middleware.AuthMiddleware())
	{
		admin.POST("/products/add", controllers.AddProduct())
		admin.PATCH("/products/update/:id", controllers.UpdateProduct())
		admin.POST("/categories/add", controllers.AddCategory())
		admin.PATCH("/categories/update/:id", controllers.UpdateCategory())
		admin.DELETE("/categories/delete/:id", controllers.DeleteCategory())

		admin.GET("/quote-requests/:id/notes", controllers.AdminGetQuoteNotes())
		admin.POST("/quote-requests/:id/notes", controllers.AdminAddQuoteNote())
		admin.POST("/quote-requests/:id/notes/with-quote", controllers.AdminAddQuoteNoteWithPDF())
	}
	// Start server on port 8080 (default)
	// Server will listen on 0.0.0.0:8080 (localhost:8080 on Windows)
	r.Run()
}

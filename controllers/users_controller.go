package controllers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/princinho/sahobackend/database"
	"github.com/princinho/sahobackend/dto"
	"github.com/princinho/sahobackend/models"
	"github.com/princinho/sahobackend/utils"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func requireAdmin(c *gin.Context) bool {
	roleVal, ok := c.Get("role")
	if !ok || roleVal.(string) != string(models.RoleAdmin) {
		return false
	}
	return true
}

// POST /admin/users
func CreateUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAdmin(c) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only Admins can open accounts"})
		}

		var body dto.RegisterUserDTO
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		email := strings.ToLower(strings.TrimSpace(body.Email))

		usersCol := database.OpenCollection("users")

		hash, err := utils.HashPassword(body.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
			return
		}

		now := time.Now().UTC()
		user := models.User{
			ID:           bson.NewObjectID(),
			Email:        email,
			PasswordHash: hash,
			Role:         models.RoleAdmin,
			IsActive:     true,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		if _, err := usersCol.InsertOne(c.Request.Context(), user); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"id":        user.ID,
			"email":     user.Email,
			"role":      user.Role,
			"isActive":  user.IsActive,
			"createdAt": user.CreatedAt,
			"updatedAt": user.UpdatedAt,
		})
	}
}

// POST /admin/users/me/password
func ChangeMyPassword() gin.HandlerFunc {
	return func(c *gin.Context) {
		var body dto.ChangeMyPasswordDTO
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		userIDStr, ok := c.Get("userID")
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing auth context"})
			return
		}

		userID, err := bson.ObjectIDFromHex(userIDStr.(string))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid auth context"})
			return
		}

		usersCol := database.OpenCollection("users")

		var user models.User
		if err := usersCol.FindOne(c.Request.Context(), bson.M{"_id": userID}).Decode(&user); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
			return
		}

		if err := utils.CheckPassword(user.PasswordHash, body.CurrentPassword); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "current password is incorrect"})
			return
		}

		newHash, err := utils.HashPassword(body.NewPassword)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
			return
		}

		now := time.Now().UTC()
		_, err = usersCol.UpdateByID(c.Request.Context(), userID, bson.M{
			"$set": bson.M{
				"passwordHash": newHash,
				"updatedAt":    now,
			},
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		_ = RevokeAllRefreshTokens(c, userID)
		utils.ClearRefreshCookie(c)

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

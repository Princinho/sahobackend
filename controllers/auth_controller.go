package controllers

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/princinho/sahobackend/database"
	"github.com/princinho/sahobackend/dto"
	"github.com/princinho/sahobackend/models"
	"github.com/princinho/sahobackend/utils"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func Login() gin.HandlerFunc {
	return func(c *gin.Context) {
		var dto dto.LoginDTO
		if err := c.ShouldBindJSON(&dto); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var user models.User
		usersCol := database.OpenCollection("users")
		if err := usersCol.FindOne(c, bson.M{"email": dto.Email}).Decode(&user); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		if err := utils.CheckPassword(user.PasswordHash, dto.Password); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		if !user.IsActive {
			c.JSON(http.StatusForbidden, gin.H{"error": "account disabled"})
			return
		}

		accessToken, _ := utils.GenerateAccessToken(user.ID.Hex(), user.Email, string(user.Role), utils.AccessTTL())
		refreshToken, _ := utils.GenerateRefreshToken(user.ID.Hex())

		refreshTokensCol := database.OpenCollection("refresh_tokens")
		result, err := refreshTokensCol.InsertOne(c, models.RefreshToken{
			UserID:     user.ID,
			TokenHash:  refreshToken,
			ExpiresAt:  time.Now().Add(time.Hour * 24 * 30),
			CreatedAt:  time.Now(),
			RevokedAt:  nil,
			ReplacedBy: nil,
		})
		if result.InsertedID == nil || err != nil {
			log.Print("Connection failed ", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "connection failed"})
			return
		}
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     "refreshToken",
			Value:    refreshToken,
			Path:     "/auth/refresh",
			MaxAge:   int((7 * 24 * time.Hour).Seconds()),
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteNoneMode, // for cross-site
		})
		c.JSON(http.StatusOK, gin.H{
			"access_token": accessToken,
		})
	}
}
func Refresh() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		usersCol := database.OpenCollection("users")
		refreshCol := database.OpenCollection("refresh_tokens")

		hash, err := c.Cookie("refreshToken")
		if err != nil || hash == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing refresh token"})
			return
		}
		var rt models.RefreshToken
		err = refreshCol.FindOne(ctx, bson.M{
			"tokenHash": hash,
			"revokedAt": bson.M{"$exists": false},
			"expiresAt": bson.M{"$gt": time.Now().UTC()},
		}).Decode(&rt)

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
			return
		}

		var user models.User
		if err := usersCol.FindOne(ctx, bson.M{"_id": rt.UserID}).Decode(&user); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
			return
		}
		if !user.IsActive {
			c.JSON(http.StatusForbidden, gin.H{"error": "account disabled"})
			return
		}

		accessTTL := utils.AccessTTL()
		refreshTTL := utils.RefreshTTL()

		// Rotate refresh token
		newHash, err := utils.GenerateRefreshToken(user.ID.String())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to rotate refresh token"})
			return
		}

		now := time.Now().UTC()

		_, err = refreshCol.UpdateByID(ctx, rt.ID, bson.M{
			"$set": bson.M{
				"revokedAt":  now,
				"replacedBy": newHash,
			},
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke refresh token"})
			return
		}

		// Insert new token
		_, err = refreshCol.InsertOne(ctx, models.RefreshToken{
			UserID:    user.ID,
			TokenHash: newHash,
			ExpiresAt: now.Add(refreshTTL),
			CreatedAt: now,
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store refresh token"})
			return
		}

		accessToken, err := utils.GenerateAccessToken(user.ID.Hex(), user.Email, string(user.Role), accessTTL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate access token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"accessToken": accessToken})
	}
}

func Logout() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		refreshCol := database.OpenCollection("refresh_tokens")

		hash, _ := c.Cookie("refreshToken")
		utils.ClearRefreshCookie(c)

		// best effort revoke
		if hash != "" {
			now := time.Now().UTC()
			_, _ = refreshCol.UpdateOne(ctx, bson.M{
				"tokenHash": hash,
				"revokedAt": bson.M{"$exists": false},
			}, bson.M{
				"$set": bson.M{"revokedAt": now},
			})
		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

func RevokeAllRefreshTokens(ctx *gin.Context, userID bson.ObjectID) error {
	refreshCol := database.OpenCollection("refresh_tokens")
	now := time.Now().UTC()
	_, err := refreshCol.UpdateMany(ctx.Request.Context(), bson.M{
		"userId":    userID,
		"revokedAt": bson.M{"$exists": false},
	}, bson.M{
		"$set": bson.M{"revokedAt": now},
	})
	return err
}

package utils

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/princinho/sahobackend/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func SeedAdminUser(ctx context.Context, usersCol *mongo.Collection) error {
	email := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_EMAIL")))
	pass := os.Getenv("ADMIN_PASSWORD")

	if email == "" || pass == "" {
		return fmt.Errorf("missing ADMIN_EMAIL or ADMIN_PASSWORD env vars")
	}

	hash, err := HashPassword(pass)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	now := time.Now().UTC()

	// Only insert if it doesn't exist
	filter := bson.M{"email": email}
	update := bson.M{
		"$setOnInsert": bson.M{
			"email":        email,
			"passwordHash": hash,
			"role":         models.RoleAdmin,
			"isActive":     true,
			"createdAt":    now,
			"updatedAt":    now,
		},
	}

	opts := options.UpdateOne().SetUpsert(true)

	res, err := usersCol.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("seed admin upsert failed: %w", err)
	}

	if res.UpsertedCount == 1 {
		fmt.Println("Admin user seeded:", email)
	} else {
		fmt.Println("Admin user already exists:", email)
	}

	return nil
}

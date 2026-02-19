package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"mime/multipart"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"google.golang.org/api/option"
)

// uploadFile uploads an object.
func UploadFile(w io.Writer, bucket, object string) error {
	// bucket := "bucket-name"
	// object := "object-name"
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("storage.NewClient: %w", err)
	}
	defer client.Close()

	// Open local file.
	f, err := os.Open("notes.txt")
	if err != nil {
		return fmt.Errorf("os.Open: %w", err)
	}
	defer f.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*50)
	defer cancel()

	o := client.Bucket(bucket).Object(object)

	// Optional: set a generation-match precondition to avoid potential race
	// conditions and data corruptions. The request to upload is aborted if the
	// object's generation number does not match your precondition.
	// For an object that does not yet exist, set the DoesNotExist precondition.
	o = o.If(storage.Conditions{DoesNotExist: true})
	// If the live object already exists in your bucket, set instead a
	// generation-match precondition using the live object's generation number.
	// attrs, err := o.Attrs(ctx)
	// if err != nil {
	// 	return fmt.Errorf("object.Attrs: %w", err)
	// }
	// o = o.If(storage.Conditions{GenerationMatch: attrs.Generation})

	// Upload an object with storage.Writer.
	wc := o.NewWriter(ctx)
	if _, err = io.Copy(wc, f); err != nil {
		return fmt.Errorf("io.Copy: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("Writer.Close: %w", err)
	}
	fmt.Fprintf(w, "Blob %v uploaded.\n", object)
	return nil
}

func NewGCSClient(ctx context.Context, serviceKeyPath string) (*storage.Client, error) {
	return storage.NewClient(ctx, option.WithAuthCredentialsFile(option.ServiceAccount, serviceKeyPath))
}
func UploadImagesToGCSAndGetPublicURLs(
	ctx context.Context,
	gcs *storage.Client,
	bucketName string,
	productSlug string,
	files []*multipart.FileHeader,
) ([]string, error) {

	if len(files) < 1 || len(files) > 4 {
		return nil, fmt.Errorf("images must be 1 to 4")
	}

	urls := make([]string, 0, len(files))

	for _, fh := range files {
		// Build a safe unique object name
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		if ext == "" {
			ext = ".bin"
		}
		objectName := fmt.Sprintf("products/%s/%d%s", productSlug, time.Now().UnixNano(), ext)

		f, err := fh.Open()
		if err != nil {
			return nil, fmt.Errorf("open file: %w", err)
		}

		w := gcs.Bucket(bucketName).Object(objectName).NewWriter(ctx)

		// Keep content-type if provided
		if ct := fh.Header.Get("Content-Type"); ct != "" {
			w.ContentType = ct
		}

		if _, err := io.Copy(w, f); err != nil {
			_ = f.Close()
			_ = w.Close()
			return nil, fmt.Errorf("upload copy: %w", err)
		}

		_ = f.Close()

		if err := w.Close(); err != nil {
			return nil, fmt.Errorf("upload close: %w", err)
		}

		publicURL := fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucketName, objectName)
		urls = append(urls, publicURL)
	}

	return urls, nil
}

func StringsToObjectIDs(ids []string) ([]bson.ObjectID, error) {
	objectIDs := make([]bson.ObjectID, 0, len(ids))

	for _, id := range ids {
		objID, err := bson.ObjectIDFromHex(id)
		if err != nil {
			return nil, err
		}
		objectIDs = append(objectIDs, objID)
	}

	return objectIDs, nil
}

func IsDuplicateKey(err error) bool {
	// Preferred: typed error
	var we mongo.WriteException
	if errors.As(err, &we) {
		for _, e := range we.WriteErrors {
			log.Println("Error code", e.Code)
			if e.Code == 11000 || e.Code == 11001 {
				return true
			}
		}
	}

	// Sometimes we might get a BulkWriteException
	var bwe mongo.BulkWriteException
	if errors.As(err, &bwe) {
		for _, e := range bwe.WriteErrors {
			if e.Code == 11000 || e.Code == 11001 {
				return true
			}
		}
	}

	// Fallback
	msg := err.Error()
	return strings.Contains(msg, "E11000 duplicate key error")
}

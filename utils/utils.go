package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/princinho/sahobackend/models"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/text/unicode/norm"

	"mime/multipart"
	"path/filepath"

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

//	func NewGCSClient(ctx context.Context, serviceKeyPath string) (*storage.Client, error) {
//		return storage.NewClient(ctx, option.WithAuthCredentialsFile(option.ServiceAccount, serviceKeyPath))
//	}
func NewGCSClient(c *gin.Context) (*storage.Client, string, error) {
	GCSBucket := os.Getenv("GCS_BUCKET")
	credentialsPath := os.Getenv("CREDENTIALS_FILE_LOCATION")
	wd, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}
	client, err := storage.NewClient(c, option.WithAuthCredentialsFile(option.ServiceAccount, filepath.Join(wd, credentialsPath)))

	if err != nil {
		return nil, "", err
	}
	return client, GCSBucket, err
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

func GenerateSlug(name string) string {
	// Normalize accents
	t := norm.NFD.String(name)
	var b strings.Builder
	for _, r := range t {
		if unicode.Is(unicode.Mn, r) {
			continue // remove accent marks
		}
		b.WriteRune(r)
	}

	s := strings.ToLower(b.String())

	// Replace non-alphanumeric with hyphen
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	s = reg.ReplaceAllString(s, "-")

	// Trim hyphens
	s = strings.Trim(s, "-")

	return s
}

func IntersectStrings(a, b []string) []string {
	set := make(map[string]struct{}, len(b))
	for _, x := range b {
		set[x] = struct{}{}
	}
	out := make([]string, 0)
	for _, x := range a {
		if _, ok := set[x]; ok {
			out = append(out, x)
		}
	}
	return out
}

func ObjectNameFromGCSPublicURL(bucket string, raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}

	host := strings.ToLower(u.Host)
	path := strings.TrimPrefix(u.Path, "/")

	// style 1: storage.googleapis.com/<bucket>/<object>
	if host == "storage.googleapis.com" {
		prefix := bucket + "/"
		if !strings.HasPrefix(path, prefix) {
			return "", fmt.Errorf("url bucket mismatch")
		}
		return strings.TrimPrefix(path, prefix), nil
	}

	// style 2: <bucket>.storage.googleapis.com/<object>
	if host == strings.ToLower(bucket)+".storage.googleapis.com" {
		if path == "" {
			return "", fmt.Errorf("missing object path")
		}
		return path, nil
	}

	return "", fmt.Errorf("not a gcs public url")
}

func DeleteGCSObjects(ctx context.Context, client *storage.Client, bucket string, objectNames []string) error {
	var firstErr error

	for _, obj := range objectNames {
		if obj == "" {
			continue
		}
		err := client.Bucket(bucket).Object(obj).Delete(ctx)
		if err != nil && firstErr == nil {
			firstErr = fmt.Errorf("delete %s: %w", obj, err)
		}
	}

	return firstErr
}

func UploadQuotePDFToGCS(
	ctx context.Context,
	client *storage.Client,
	bucketName string,
	quoteID string,
	fileHeader *multipart.FileHeader,
) (*models.QuoteAttachment, error) {

	// Only allow PDF
	if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".pdf") {
		return nil, fmt.Errorf("only PDF files are allowed")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Generate unique object name
	timestamp := time.Now().UTC().Unix()
	random := uuid.New().String()

	objectName := fmt.Sprintf(
		"quotes/%s/%d-%s.pdf",
		quoteID,
		timestamp,
		random,
	)

	bucket := client.Bucket(bucketName)
	obj := bucket.Object(objectName)

	writer := obj.NewWriter(ctx)
	writer.ContentType = "application/pdf"
	writer.CacheControl = "no-cache"

	if _, err := io.Copy(writer, file); err != nil {
		_ = writer.Close()
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	publicURL := fmt.Sprintf(
		"https://storage.googleapis.com/%s/%s",
		bucketName,
		objectName,
	)

	return &models.QuoteAttachment{
		PublicURL:  publicURL,
		ObjectName: objectName,
		MimeType:   "application/pdf",
		SizeBytes:  fileHeader.Size,
	}, nil
}

func MergeImageUrlsArrays(
	oldUrls []string,
	toRemove []string,
	toAdd []string,
) []string {

	// Step 1: build a set of urls to remove
	removeSet := make(map[string]struct{}, len(toRemove))
	for _, u := range toRemove {
		removeSet[u] = struct{}{}
	}

	// Step 2: keep old urls that are NOT removed
	final := make([]string, 0, len(oldUrls)+len(toAdd))
	exists := make(map[string]struct{}) // prevent duplicates

	for _, u := range oldUrls {
		if _, shouldRemove := removeSet[u]; !shouldRemove {
			final = append(final, u)
			exists[u] = struct{}{}
		}
	}

	// Step 3: append new urls (avoid duplicates)
	for _, u := range toAdd {
		if _, already := exists[u]; !already {
			final = append(final, u)
			exists[u] = struct{}{}
		}
	}

	return final
}

func ParseBoolQuery(value string) (*bool, error) {
	if value == "" {
		return nil, nil // not provided
	}

	b, err := strconv.ParseBool(value)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func ParseIntDefault(v string, def int) int {
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPassword(hash string, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func GenerateAccessToken(userID, email, role string, accessTTL time.Duration) (string, error) {
	claims := Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(accessTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(os.Getenv("JWT_SECRET")))
}

func GenerateRefreshToken(userID string) (string, error) {
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(os.Getenv("JWT_REFRESH_SECRET")))
}

func ValidateToken(tokenStr string, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, err
	}
	return token.Claims.(*Claims), nil
}
func ClearRefreshCookie(c *gin.Context) {
	secure := os.Getenv("COOKIE_SECURE") == "true"
	domain := os.Getenv("COOKIE_DOMAIN")
	path := "/auth"

	c.SetCookie("refreshToken", "", -1, path, domain, secure, true)
}
func AccessTTL() time.Duration {
	minStr := os.Getenv("ACCESS_TOKEN_TTL_MINUTES")
	min, _ := strconv.Atoi(minStr)
	if min <= 0 {
		min = 15
	}
	return time.Duration(min) * time.Minute
}

func RefreshTTL() time.Duration {
	dStr := os.Getenv("REFRESH_TOKEN_TTL_DAYS")
	days, _ := strconv.Atoi(dStr)
	if days <= 0 {
		days = 14
	}
	return time.Duration(days) * 24 * time.Hour
}

func UploadProductRequestFileToGCS(
	ctx context.Context,
	client *storage.Client,
	bucketName string,
	requestID string,
	fileHeader *multipart.FileHeader,
) (*models.ProductRequestAttachment, error) {

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))

	allowed := map[string]bool{
		".pdf":  true,
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".webp": true,
	}

	if !allowed[ext] {
		return nil, fmt.Errorf("file type not allowed (allowed: pdf, jpg, jpeg, png, webp)")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	timestamp := time.Now().UTC().Unix()
	random := uuid.New().String()

	objectName := fmt.Sprintf(
		"product-requests/%s/%d-%s%s",
		requestID,
		timestamp,
		random,
		ext,
	)

	obj := client.Bucket(bucketName).Object(objectName)
	writer := obj.NewWriter(ctx)

	ct := fileHeader.Header.Get("Content-Type")
	if ct == "" {
		ct = mime.TypeByExtension(ext)
		if ct == "" {
			ct = "application/octet-stream"
		}
	}
	writer.ContentType = ct
	writer.CacheControl = "no-cache"

	if _, err := io.Copy(writer, file); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	publicURL := fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucketName, objectName)

	return &models.ProductRequestAttachment{
		ImageURL:   publicURL,
		ObjectName: objectName,
		MimeType:   ct,
		SizeBytes:  fileHeader.Size,
		FileName:   fileHeader.Filename,
		UploadedAt: time.Now().UTC(),
	}, nil
}

type FileValidator struct {
	allowedExt  map[string]bool
	allowedMime map[string]bool
	maxSize     int64
}

func NewPDFOrImageValidator() *FileValidator {
	allowedExt := make(map[string]bool)
	for _, ext := range strings.Split(os.Getenv("ALLOWED_FILE_EXTENSIONS"), ",") {
		if ext = strings.TrimSpace(strings.ToLower(ext)); ext != "" {
			allowedExt[ext] = true
		}
	}

	allowedMime := make(map[string]bool)
	for _, m := range strings.Split(os.Getenv("ALLOWED_FILE_MIME_TYPES"), ",") {
		if m = strings.TrimSpace(strings.ToLower(m)); m != "" {
			allowedMime[m] = true
		}
	}

	sizeMB := 5
	if v := os.Getenv("MAX_UPLOAD_SIZE_MB"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			sizeMB = parsed
		}
	}

	return &FileValidator{
		allowedExt:  allowedExt,
		allowedMime: allowedMime,
		maxSize:     int64(sizeMB) << 20,
	}
}

func (v *FileValidator) ValidateFile(fileHeader *multipart.FileHeader) (string, error) {
	if fileHeader.Size > v.maxSize {
		return "", fmt.Errorf("file too large (max %d MB)", v.maxSize>>20)
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if !v.allowedExt[ext] {
		return "", fmt.Errorf("invalid file extension")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	buffer := make([]byte, 512)
	if _, err = file.Read(buffer); err != nil {
		return "", fmt.Errorf("failed to read file header")
	}
	if _, err = file.Seek(0, 0); err != nil {
		return "", fmt.Errorf("failed to reset file reader")
	}

	detectedMime := strings.ToLower(http.DetectContentType(buffer))
	if !v.allowedMime[detectedMime] {
		return "", fmt.Errorf("invalid file type")
	}

	return detectedMime, nil
}

func GetDefaultQueryLimits() (int, int) {
	maxLimitStr := os.Getenv("READ_QUERY_MAX_LIMIT")
	defaultLimitStr := os.Getenv("DEFAULT_READ_QUERY_LIMIT")
	maxLimit, err := strconv.Atoi(maxLimitStr)
	if err != nil {
		// handle error or fall back to a default
		maxLimit = 100
	}
	defaultLimit, err := strconv.Atoi(defaultLimitStr)
	if err != nil {
		// handle error or fall back to a default
		defaultLimit = 20
	}
	return maxLimit, defaultLimit
}

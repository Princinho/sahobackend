package utils

import (
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/princinho/sahobackend/models"
)

// R2Client wraps the S3 client + bucket name so callers get the same
// two-value return they had with NewGCSClient.
type R2Client struct {
	S3     *s3.Client
	Bucket string
}

// NewCloudClient is kept with its original signature so no controller code changes.
// It now returns an *R2Client (as the *storage.Client slot) and the bucket name.
func NewCloudClient(c *gin.Context) (*R2Client, string, error) {
	bucket := os.Getenv("R2_BUCKET")
	accessKey := os.Getenv("R2_ACCESS_KEY_ID")
	secretKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	endpoint := os.Getenv("R2_ENDPOINT") // https://<account-id>.r2.cloudflarestorage.com

	if bucket == "" || accessKey == "" || secretKey == "" || endpoint == "" {
		return nil, "", fmt.Errorf("missing R2 env vars (R2_BUCKET, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY, R2_ENDPOINT)")
	}

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, "", fmt.Errorf("r2 config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true // required for R2
	})

	return &R2Client{S3: client, Bucket: bucket}, bucket, nil
}

// UploadImagesToCloudAndGetPublicURLs — same signature, now uploads to R2.
// Public URLs use the R2 public domain set via R2_PUBLIC_DOMAIN env var
// (e.g. "https://files.yourdomain.com" if you've connected a custom domain,
// or the r2.dev public URL you can enable in the R2 bucket settings).
func UploadImagesToCloudAndGetPublicURLs(
	ctx context.Context,
	r2 *R2Client,
	_ string, // bucketName arg kept for signature compatibility — we use r2.Bucket
	productSlug string,
	files []*multipart.FileHeader,
) ([]string, error) {

	if len(files) < 1 || len(files) > 4 {
		return nil, fmt.Errorf("images must be 1 to 4")
	}

	urls := make([]string, 0, len(files))

	for _, fh := range files {
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		if ext == "" {
			ext = ".bin"
		}
		objectName := fmt.Sprintf("products/%s/%d%s", productSlug, time.Now().UnixNano(), ext)

		ct := fh.Header.Get("Content-Type")
		if ct == "" {
			ct = mime.TypeByExtension(ext)
		}
		if ct == "" {
			ct = "application/octet-stream"
		}

		f, err := fh.Open()
		if err != nil {
			return nil, fmt.Errorf("open file: %w", err)
		}

		_, err = r2.S3.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(r2.Bucket),
			Key:         aws.String(objectName),
			Body:        f,
			ContentType: aws.String(ct),
		})
		_ = f.Close()
		if err != nil {
			return nil, fmt.Errorf("upload %s: %w", fh.Filename, err)
		}

		urls = append(urls, publicURL(objectName))
	}

	return urls, nil
}

// DeleteCloudObjects — same signature, now deletes from R2.
func DeleteCloudObjects(ctx context.Context, r2 *R2Client, _ string, objectNames []string) error {
	var firstErr error
	for _, obj := range objectNames {
		if obj == "" {
			continue
		}
		_, err := r2.S3.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(r2.Bucket),
			Key:    aws.String(obj),
		})
		if err != nil && firstErr == nil {
			firstErr = fmt.Errorf("delete %s: %w", obj, err)
		}
	}
	return firstErr
}

// ObjectNameFromCloudPublicURL — same signature, now parses R2 public URLs.
// Supports both custom domain (R2_PUBLIC_DOMAIN) and r2.dev subdomain URLs.
func ObjectNameFromCloudPublicURL(_ string, raw string) (string, error) {
	domain := strings.TrimRight(os.Getenv("R2_PUBLIC_DOMAIN"), "/")
	bucket := os.Getenv("R2_BUCKET")
	if domain != "" && strings.HasPrefix(raw, domain+"/"+bucket+"/") {
		return strings.TrimPrefix(raw, domain+"/"), nil
	}

	// r2.dev style: https://<bucket>.<account>.r2.dev/<object>
	// Just strip the scheme + host and return the path
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(raw, prefix) {
			withoutScheme := strings.TrimPrefix(raw, prefix)
			slash := strings.Index(withoutScheme, "/")
			if slash == -1 {
				return "", fmt.Errorf("no object path in url")
			}
			return withoutScheme[slash+1:], nil
		}
	}

	return "", fmt.Errorf("not a recognised R2 public url")
}

// UploadQuotePDFToCloud — same signature, now uploads to R2.
func UploadQuotePDFToCloud(
	ctx context.Context,
	r2 *R2Client,
	_ string, // bucketName — kept for compatibility
	quoteID string,
	fileHeader *multipart.FileHeader,
) (*models.QuoteAttachment, error) {

	if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".pdf") {
		return nil, fmt.Errorf("only PDF files are allowed")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	objectName := fmt.Sprintf("quotes/%s/%d-%s.pdf", quoteID, time.Now().UTC().Unix(), uuid.New().String())

	_, err = r2.S3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(r2.Bucket),
		Key:          aws.String(objectName),
		Body:         file,
		ContentType:  aws.String("application/pdf"),
		CacheControl: aws.String("no-cache"),
	})
	if err != nil {
		return nil, fmt.Errorf("upload pdf: %w", err)
	}

	return &models.QuoteAttachment{
		PublicURL:  publicURL(objectName),
		ObjectName: objectName,
		MimeType:   "application/pdf",
		SizeBytes:  fileHeader.Size,
	}, nil
}

// UploadProductRequestFileToGCS — same signature, now uploads to R2.
func UploadProductRequestFileToGCS(
	ctx context.Context,
	r2 *R2Client,
	_ string, // bucketName — kept for compatibility
	requestID string,
	fileHeader *multipart.FileHeader,
) (*models.ProductRequestAttachment, error) {

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	allowed := map[string]bool{
		".pdf": true, ".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
	}
	if !allowed[ext] {
		return nil, fmt.Errorf("file type not allowed (allowed: pdf, jpg, jpeg, png, webp)")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	ct := fileHeader.Header.Get("Content-Type")
	if ct == "" {
		ct = mime.TypeByExtension(ext)
	}
	if ct == "" {
		ct = "application/octet-stream"
	}

	objectName := fmt.Sprintf(
		"product-requests/%s/%d-%s%s",
		requestID, time.Now().UTC().Unix(), uuid.New().String(), ext,
	)

	_, err = r2.S3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(r2.Bucket),
		Key:          aws.String(objectName),
		Body:         file,
		ContentType:  aws.String(ct),
		CacheControl: aws.String("no-cache"),
	})
	if err != nil {
		return nil, fmt.Errorf("upload file: %w", err)
	}

	return &models.ProductRequestAttachment{
		ImageURL:   publicURL(objectName),
		ObjectName: objectName,
		MimeType:   ct,
		SizeBytes:  fileHeader.Size,
		FileName:   fileHeader.Filename,
		UploadedAt: time.Now().UTC(),
	}, nil
}

// publicURL builds the public URL for a stored object.
// Set R2_PUBLIC_DOMAIN in your env to your custom domain or r2.dev URL,
// e.g. "https://files.yourdomain.com" or "https://pub-xxx.r2.dev"
func publicURL(objectName string) string {
	bucket := os.Getenv("R2_BUCKET")
	domain := strings.TrimRight(os.Getenv("R2_PUBLIC_DOMAIN"), "/")
	fullUrl := fmt.Sprintf("%s/%s/%s", domain, bucket, objectName)
	return fmt.Sprintf("%s", fullUrl)
}

// streamBody wraps an io.Reader so it satisfies io.ReadSeeker — needed by
// the AWS SDK when Content-Length cannot be determined from the body alone.
// (In practice PutObject accepts a plain io.Reader; this is kept as a
// safety net in case your SDK version requires it.)
type streamBody struct {
	r io.Reader
}

func (s streamBody) Read(p []byte) (int, error)         { return s.r.Read(p) }
func (s streamBody) Seek(_ int64, _ int) (int64, error) { return 0, nil }

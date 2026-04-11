package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIO provides file storage via S3-compatible MinIO.
type MinIO struct {
	client *minio.Client
	bucket string
}

// NewMinIO creates a new MinIO storage client and ensures the bucket exists.
func NewMinIO() (*MinIO, error) {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:9000"
	}
	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	if accessKey == "" {
		accessKey = "devsocial"
	}
	secretKey := os.Getenv("MINIO_SECRET_KEY")
	if secretKey == "" {
		secretKey = "devsocial123"
	}
	bucket := os.Getenv("MINIO_BUCKET")
	if bucket == "" {
		bucket = "devsocial"
	}
	useSSL := os.Getenv("MINIO_USE_SSL") == "true"

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	m := &MinIO{client: client, bucket: bucket}

	// Ensure bucket exists
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		log.Printf("[minio] bucket check error: %v", err)
		return m, nil
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			log.Printf("[minio] create bucket error: %v", err)
		} else {
			log.Printf("[minio] created bucket: %s", bucket)
		}
	}

	return m, nil
}

// Upload stores a file in the bucket and returns the S3 key.
func (m *MinIO) Upload(ctx context.Context, s3Key string, reader io.Reader, size int64, contentType string) error {
	opts := minio.PutObjectOptions{
		ContentType: contentType,
	}
	_, err := m.client.PutObject(ctx, m.bucket, s3Key, reader, size, opts)
	return err
}

// Download retrieves a file from the bucket.
func (m *MinIO) Download(ctx context.Context, s3Key string) (*minio.Object, error) {
	return m.client.GetObject(ctx, m.bucket, s3Key, minio.GetObjectOptions{})
}

// Delete removes a file from the bucket.
func (m *MinIO) Delete(ctx context.Context, s3Key string) error {
	return m.client.RemoveObject(ctx, m.bucket, s3Key, minio.RemoveObjectOptions{})
}

// Health checks MinIO connectivity.
func (m *MinIO) Health(ctx context.Context) error {
	_, err := m.client.ListBuckets(ctx)
	return err
}

// MakeS3Key creates a deterministic key for a workspace file.
func MakeS3Key(workspaceID int64, filename string) string {
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	clean := strings.Map(safeChar, base)
	if len(clean) > 64 {
		clean = clean[:64]
	}
	return fmt.Sprintf("workspaces/%d/%s%d%s", workspaceID, clean, time.Now().UnixNano(), ext)
}

func safeChar(r rune) rune {
	if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
		return r
	}
	return '_'
}

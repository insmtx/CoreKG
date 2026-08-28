package s3util

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/ygpkg/yg-go/config"
	"github.com/ygpkg/yg-go/storage"
)

func TestNewPresignerUsesPublicEndpointWithoutNetworkProbe(t *testing.T) {
	presigner, err := NewPresigner(config.S3StorageConfig{
		EndPoint:        "http://minio:9000",
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minio123456",
		Region:          "minio",
		Bucket:          "corekg-bucket",
		UsePathStyle:    true,
	}, config.StorageOption{PresignedTimeout: time.Hour}, "http://localhost:3001/")
	if err != nil {
		t.Fatalf("NewPresigner() error = %v", err)
	}

	method := http.MethodPut
	path := "forest-file/1/example.md"
	urlValue, err := presigner.GeneratePresignedURL(context.Background(), &storage.GeneratePresignedURLInput{
		Method:      &method,
		StoragePath: &path,
	})
	if err != nil {
		t.Fatalf("GeneratePresignedURL() error = %v", err)
	}

	parsed, err := url.Parse(*urlValue)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	if parsed.Scheme != "http" || parsed.Host != "localhost:3001" {
		t.Fatalf("signed URL origin = %s://%s, want http://localhost:3001", parsed.Scheme, parsed.Host)
	}
	if parsed.Path != "/corekg-bucket/forest-file/1/example.md" {
		t.Fatalf("signed URL path = %q", parsed.Path)
	}
	if !strings.Contains(parsed.RawQuery, "X-Amz-Signature=") {
		t.Fatalf("signed URL has no signature: %q", parsed.RawQuery)
	}
}

func TestNormalizeEndpointRejectsPath(t *testing.T) {
	if _, err := normalizeEndpoint("http://localhost:3001/corekg-bucket"); err == nil {
		t.Fatal("normalizeEndpoint() expected path validation error")
	}
	if _, err := normalizeEndpoint("http://localhost:3001/"); err != nil {
		t.Fatalf("normalizeEndpoint() trailing slash error = %v", err)
	}
}

func TestPresignerRequiresMultipartPartNumber(t *testing.T) {
	presigner, err := NewPresigner(config.S3StorageConfig{
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minio123456",
		Region:          "minio",
		Bucket:          "corekg-bucket",
		UsePathStyle:    true,
	}, config.StorageOption{}, "http://localhost:3001")
	if err != nil {
		t.Fatalf("NewPresigner() error = %v", err)
	}

	method := http.MethodPut
	path := "forest-file/1/example.md"
	uploadID := "upload-id"
	_, err = presigner.GeneratePresignedURL(context.Background(), &storage.GeneratePresignedURLInput{
		Method:      &method,
		StoragePath: &path,
		UploadID:    aws.String(uploadID),
	})
	if err == nil {
		t.Fatal("GeneratePresignedURL() expected missing part number error")
	}
}

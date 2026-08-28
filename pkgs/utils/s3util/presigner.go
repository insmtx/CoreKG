package s3util

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ygpkg/yg-go/config"
	"github.com/ygpkg/yg-go/storage"
)

// Presigner generates S3 URLs without contacting the configured endpoint.
type Presigner struct {
	client  *s3.PresignClient
	bucket  string
	timeout time.Duration
}

// NewPresigner creates a signer for a public endpoint. The endpoint is only
// used when constructing signed URLs; it is never probed or contacted.
func NewPresigner(cfg config.S3StorageConfig, opt config.StorageOption, endpoint string) (*Presigner, error) {
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("s3 bucket is empty")
	}

	endpoint, err := normalizeEndpoint(endpoint)
	if err != nil {
		return nil, err
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = cfg.UsePathStyle
	})

	return &Presigner{
		client:  s3.NewPresignClient(client),
		bucket:  cfg.Bucket,
		timeout: opt.PresignedTimeout,
	}, nil
}

// GeneratePresignedURL implements the narrow interface used by Forest file
// upload and preview flows.
func (p *Presigner) GeneratePresignedURL(ctx context.Context, in *storage.GeneratePresignedURLInput) (*string, error) {
	if in == nil || in.StoragePath == nil || strings.TrimSpace(aws.ToString(in.StoragePath)) == "" {
		return nil, fmt.Errorf("storage path is empty")
	}

	bucket := p.bucket
	if in.Bucket != nil && strings.TrimSpace(aws.ToString(in.Bucket)) != "" {
		bucket = aws.ToString(in.Bucket)
	}

	method := aws.ToString(in.Method)
	switch method {
	case http.MethodGet:
		out, err := p.client.PresignGetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    in.StoragePath,
		}, func(options *s3.PresignOptions) {
			options.Expires = p.timeout
		})
		if err != nil {
			return nil, err
		}
		return aws.String(out.URL), nil
	case http.MethodPut:
		if in.UploadID == nil || strings.TrimSpace(aws.ToString(in.UploadID)) == "" {
			out, err := p.client.PresignPutObject(ctx, &s3.PutObjectInput{
				Bucket:      aws.String(bucket),
				Key:         in.StoragePath,
				ContentType: in.ContentType,
			}, func(options *s3.PresignOptions) {
				options.Expires = p.timeout
			})
			if err != nil {
				return nil, err
			}
			return aws.String(out.URL), nil
		}
		if in.PartNumber == nil || *in.PartNumber <= 0 {
			return nil, fmt.Errorf("part number is required for multipart upload")
		}

		out, err := p.client.PresignUploadPart(ctx, &s3.UploadPartInput{
			Bucket:        aws.String(bucket),
			Key:           in.StoragePath,
			UploadId:      in.UploadID,
			PartNumber:    aws.Int32(int32(*in.PartNumber)),
			ContentMD5:    in.ContentMD5,
			ContentLength: in.ContentLength,
		}, func(options *s3.PresignOptions) {
			options.Expires = p.timeout
		})
		if err != nil {
			return nil, err
		}
		return aws.String(out.URL), nil
	default:
		return nil, fmt.Errorf("only GET and PUT are allowed, now: %s", method)
	}
}

func normalizeEndpoint(raw string) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid s3 endpoint %q", raw)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported s3 endpoint scheme %q", parsed.Scheme)
	}
	if parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("s3 endpoint must not contain a path or query: %q", raw)
	}
	return parsed.String(), nil
}

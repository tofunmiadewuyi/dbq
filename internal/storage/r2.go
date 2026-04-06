package storage

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/tofunmiadewuyi/dbq/internal/job"
)

// R2Client wraps an S3-compatible client pointed at a Cloudflare R2 endpoint.
type R2Client struct {
	client *s3.Client
	bucket string
}

func NewR2Client(cfg *job.CloudStorage) (*R2Client, error) {
	if cfg.Endpoint == "" || cfg.AKID == "" || cfg.SAK == "" || cfg.Bucket == "" {
		return nil, fmt.Errorf("missing required R2 configuration (endpoint, access_key, secret_key, bucket)")
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion("auto"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AKID,
			cfg.SAK,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load R2 config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
	})

	return &R2Client{client: client, bucket: cfg.Bucket}, nil
}

func (r *R2Client) TestConnection(ctx context.Context) error {
	_, err := r.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(r.bucket),
	})
	if err != nil {
		return fmt.Errorf("failed to access R2 bucket: %w", err)
	}
	return nil
}

func (r *R2Client) UploadBackup(ctx context.Context, timestamp time.Time, backupName, dbName, contentType string, reader io.Reader) (string, error) {
	key := fmt.Sprintf("backups/%s/%s/%d", backupName, filepath.Base(dbName), timestamp.Unix())

	_, err := r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.bucket),
		Key:         aws.String(key),
		Body:        reader,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to R2: %w", err)
	}

	return key, nil
}

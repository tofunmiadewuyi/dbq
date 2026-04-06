package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/tofunmiadewuyi/dbq/internal/job"
)

// S3Client wraps the AWS S3 client with helper methods
type S3Client struct {
	client *s3.Client
	bucket string
}

// NewS3Client creates a new S3 client
func NewS3Client(cfg *job.CloudStorage) (*S3Client, error) {
	if cfg.Region == "" || cfg.AKID == "" || cfg.SAK == "" || cfg.Bucket == "" {
		return nil, fmt.Errorf("missing required AWS configuration")
	}

	// Create AWS config with credentials
	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AKID,
			cfg.SAK,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg)

	log.Println("✓ S3 client initialized")
	return &S3Client{
		client: client,
		bucket: cfg.Bucket,
	}, nil
}

// UploadBackup uploads a backup file to S3
// timestamp: time when the backup was made
// backupName: name of the backup job
// dbName: database Name
// contentType: MIME type
// reader: attachment file content
// Returns the S3 key for the uploaded file
func (s *S3Client) UploadBackup(ctx context.Context, timestamp time.Time, backupName string, dbName string, contentType string, reader io.Reader) (string, error) {
	// Sanitize filename to avoid path traversal issues
	safeFilename := filepath.Base(dbName)
	key := fmt.Sprintf("backups/%s/%s/%d", backupName, safeFilename, timestamp.Unix())

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        reader,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload attachment to S3: %w", err)
	}

	return key, nil
}


// DownloadBackup downloads a backup file from S3
func (s *S3Client) DownloadBackup(ctx context.Context, s3Key string) ([]byte, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download attachment from S3: %w", err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read attachment content: %w", err)
	}

	return data, nil
}

// PresignPutURL returns a presigned PUT URL for direct server-to-S3 uploads.
func (s *S3Client) PresignPutURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s.client)
	req, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned upload URL: %w", err)
	}
	return req.URL, nil
}

// GetPresignedURL generates a presigned URL for downloading a file
// Useful for allowing direct downloads from S3 without proxying through your API
func (s *S3Client) GetPresignedURL(ctx context.Context, s3Key string, expirationMinutes int) (string, error) {
	presignClient := s3.NewPresignClient(s.client)

	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s3Key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = time.Duration(expirationMinutes) * time.Minute
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return request.URL, nil
}


// DeleteBackup deletes an attachment file from S3
func (s *S3Client) DeleteBackup(ctx context.Context, s3Key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete backup from S3: %w", err)
	}

	return nil
}

// TestConnection tests the S3 connection by checking bucket access
func (s *S3Client) TestConnection(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err != nil {
		return fmt.Errorf("failed to access S3 bucket: %w", err)
	}

	log.Printf("✓ Successfully connected to S3 bucket: %s", s.bucket)
	return nil
}

// ListArchiveFolders lists all unique archive folder prefixes in S3
func (s *S3Client) ListArchiveFolders(ctx context.Context) (map[string]bool, error) {
	archiveFolders := make(map[string]bool)

	// List objects with "backups/" prefix using delimiter to get "folders"
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket:    aws.String(s.bucket),
		Prefix:    aws.String("backups/"),
		Delimiter: aws.String("/"),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list S3 objects: %w", err)
		}

		// each prefix is a "folder" under backups/ — extract the timestamp segment
		for _, prefix := range page.CommonPrefixes {
			if prefix.Prefix != nil && len(*prefix.Prefix) > 8 {
				timestamp := (*prefix.Prefix)[8:]
				timestamp = strings.TrimSuffix(timestamp, "/")
				if timestamp != "" {
					archiveFolders[timestamp] = true
				}
			}
		}
	}

	return archiveFolders, nil
}

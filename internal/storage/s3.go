package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// listBackups is shared by the S3 and R2 clients (both wrap *s3.Client). It
// paginates every object under the job/db prefix and returns the ones whose
// names carry a valid dbq timestamp; anything unrecognized is skipped so it's
// never considered for pruning.
func listBackups(ctx context.Context, client *s3.Client, bucket, jobName, dbName string) ([]BackupObject, error) {
	prefix := BackupPrefix(jobName, dbName)
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})

	var objects []BackupObject
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list backups: %w", err)
		}
		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			ts, ok := ParseBackupTime(jobName, dbName, *obj.Key)
			if !ok {
				continue // not a recognizable dbq backup — leave it untouched
			}
			objects = append(objects, BackupObject{Key: *obj.Key, Timestamp: ts})
		}
	}
	return objects, nil
}

// S3Client wraps the AWS S3 client with helper methods
type S3Client struct {
	client *s3.Client
	bucket string
}

// NewS3Client creates a new S3 client
func NewS3Client(cfg *CloudStorage) (*S3Client, error) {
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

	fmt.Println("✓ S3 client initialized")
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
	key := BackupKey(backupName, dbName, timestamp, ".zip")

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

	fmt.Printf("✓ Successfully connected to S3 bucket: %s\n", s.bucket)
	return nil
}

// ListBackups lists every backup object stored for the given job/db.
func (s *S3Client) ListBackups(ctx context.Context, jobName, dbName string) ([]BackupObject, error) {
	return listBackups(ctx, s.client, s.bucket, jobName, dbName)
}

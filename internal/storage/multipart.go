package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// One replayable part is buffered at a time, so SDK retries never reread the
// producer. 64 MiB parts support streams up to 625 GiB (S3's 10,000-part limit).
const uploadPartSize = 64 * 1024 * 1024

// uploadStream accepts unknown-length, non-seekable input. EOF is the producer's
// success signal: producers must report dump/compression failures with a read
// error (for example PipeWriter.CloseWithError), never a normal EOF. Callers own
// the reader and must unblock a pending Read when cancelling (close a pipe).
func uploadStream(ctx context.Context, client *s3.Client, bucket, key, contentType string, source io.Reader) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	started, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket), Key: aws.String(key), ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("create multipart upload: %w", err)
	}
	if started.UploadId == nil || *started.UploadId == "" {
		return errors.New("create multipart upload returned no upload ID")
	}
	completed := false
	defer func() {
		if completed {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		_, abortErr := client.AbortMultipartUpload(cleanupCtx, &s3.AbortMultipartUploadInput{
			Bucket: aws.String(bucket), Key: aws.String(key), UploadId: started.UploadId,
		})
		if abortErr != nil {
			err = errors.Join(err, fmt.Errorf("abort multipart upload: %w", abortErr))
		}
	}()
	buffer := make([]byte, uploadPartSize)
	parts := make([]types.CompletedPart, 0)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readErr := readUploadPart(ctx, source, buffer)
		if readErr != nil && readErr != io.EOF {
			return fmt.Errorf("read backup stream: %w", readErr)
		}
		// An empty stream still needs one part to create an empty object.
		if n > 0 || len(parts) == 0 {
			if len(parts) == 10000 {
				return errors.New("backup exceeds multipart upload limit of 625 GiB")
			}
			number := int32(len(parts) + 1)
			part, err := client.UploadPart(ctx, &s3.UploadPartInput{
				Bucket: aws.String(bucket), Key: aws.String(key), UploadId: started.UploadId,
				PartNumber: aws.Int32(number), ContentLength: aws.Int64(int64(n)), Body: bytes.NewReader(buffer[:n]),
			})
			if err != nil {
				return fmt.Errorf("upload backup part %d: %w", number, err)
			}
			if part.ETag == nil || *part.ETag == "" {
				return fmt.Errorf("upload backup part %d returned no ETag", number)
			}
			parts = append(parts, types.CompletedPart{ETag: part.ETag, PartNumber: aws.Int32(number)})
		}
		if readErr != nil {
			break
		}
	}
	_, err = client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket: aws.String(bucket), Key: aws.String(key), UploadId: started.UploadId,
		MultipartUpload: &types.CompletedMultipartUpload{Parts: parts},
	})
	if err != nil {
		return fmt.Errorf("complete multipart upload: %w", err)
	}
	completed = true
	return nil
}

// Preserve producer errors, including ErrUnexpectedEOF; only EOF means success.
func readUploadPart(ctx context.Context, source io.Reader, buffer []byte) (int, error) {
	total, emptyReads := 0, 0
	for total < len(buffer) {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		n, err := source.Read(buffer[total:])
		total += n
		if err != nil {
			return total, err
		}
		if n == 0 {
			emptyReads++
			if emptyReads >= 100 {
				return total, io.ErrNoProgress
			}
		} else {
			emptyReads = 0
		}
	}
	return total, nil
}

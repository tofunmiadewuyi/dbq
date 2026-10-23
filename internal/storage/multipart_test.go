package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type failingBackupReader struct{}

func (failingBackupReader) Read([]byte) (int, error) { return 0, errors.New("dump failed") }

type zeroBackupReader struct{}

func (zeroBackupReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

// Exercise both public providers through the real SDK and an HTTP endpoint:
// unknown-length input, producer failures after uploaded parts, storage failure,
// and cancellation must never complete an incomplete object.
func TestUploadBackupStream(t *testing.T) {
	for _, provider := range []string{"s3", "r2"} {
		for _, scenario := range []string{"success", "empty", "producer failure", "upload failure", "cancel", "completion failure"} {
			t.Run(provider+"/"+scenario, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				var sizes []int64
				var aborted, completed bool
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/xml")
					switch {
					case r.Method == http.MethodPost && r.URL.Query().Has("uploads"):
						fmt.Fprint(w, `<InitiateMultipartUploadResult><UploadId>test-upload</UploadId></InitiateMultipartUploadResult>`)
					case r.Method == http.MethodPut:
						n, _ := io.Copy(io.Discard, r.Body)
						sizes = append(sizes, n)
						if scenario == "upload failure" {
							w.WriteHeader(403)
							fmt.Fprint(w, `<Error><Code>AccessDenied</Code></Error>`)
							return
						}
						w.Header().Set("ETag", fmt.Sprintf(`"part-%d"`, len(sizes)))
						if scenario == "cancel" {
							cancel()
						}
					case r.Method == http.MethodDelete:
						aborted = true
						w.WriteHeader(204)
					case r.Method == http.MethodPost:
						if scenario == "completion failure" {
							w.WriteHeader(403)
							fmt.Fprint(w, `<Error><Code>AccessDenied</Code></Error>`)
							return
						}
						completed = true
						fmt.Fprint(w, `<CompleteMultipartUploadResult><ETag>"done"</ETag></CompleteMultipartUploadResult>`)
					default:
						t.Errorf("unexpected request: %s %s", r.Method, r.URL)
						w.WriteHeader(400)
					}
				}))
				defer server.Close()
				client := s3.NewFromConfig(aws.Config{
					Region: "us-east-1", Credentials: credentials.NewStaticCredentialsProvider("key", "secret", ""),
					RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
				}, func(o *s3.Options) { o.BaseEndpoint = aws.String(server.URL); o.UsePathStyle = true })
				var storage StorageClient = &S3Client{client: client, bucket: "backups"}
				if provider == "r2" {
					storage = &R2Client{client: client, bucket: "backups"}
				}
				var input io.Reader = io.MultiReader(io.LimitReader(zeroBackupReader{}, uploadPartSize), strings.NewReader("end"))
				if scenario == "empty" {
					input = bytes.NewBuffer(nil)
				}
				if scenario == "producer failure" {
					input = io.MultiReader(io.LimitReader(zeroBackupReader{}, uploadPartSize), failingBackupReader{})
				}
				key, err := storage.UploadBackup(ctx, time.Unix(0, 0), "job", "db", ".gz", "application/gzip", input)
				wantSuccess := scenario == "success" || scenario == "empty"
				if wantSuccess {
					if err != nil || key == "" || !completed || aborted {
						t.Fatalf("key=%q err=%v completed=%v aborted=%v", key, err, completed, aborted)
					}
					if scenario == "success" && (len(sizes) != 2 || sizes[0] != uploadPartSize || sizes[1] != 3) {
						t.Fatalf("part sizes: %v", sizes)
					}
					if scenario == "empty" && (len(sizes) != 1 || sizes[0] != 0) {
						t.Fatalf("empty part sizes: %v", sizes)
					}
				} else {
					if err == nil || key != "" || completed || !aborted {
						t.Fatalf("key=%q err=%v completed=%v aborted=%v", key, err, completed, aborted)
					}
					if scenario == "producer failure" && !strings.Contains(err.Error(), "dump failed") {
						t.Fatal(err)
					}
				}
			})
		}
	}
}

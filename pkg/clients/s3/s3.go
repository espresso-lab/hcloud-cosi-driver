package s3

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Client wraps a MinIO client for object-level operations.
type Client struct {
	mc *minio.Client
}

// New creates an S3 Client using the provided credentials and endpoint.
func New(endpoint, accessKey, secretKey string) (*Client, error) {
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: true,
	})
	if err != nil {
		return nil, fmt.Errorf("s3: new client: %w", err)
	}
	return &Client{mc: mc}, nil
}

// DrainBucket removes all objects from a bucket. Returns nil if the bucket does not exist.
func (c *Client) DrainBucket(ctx context.Context, bucket string) error {
	objects := c.mc.ListObjects(ctx, bucket, minio.ListObjectsOptions{Recursive: true})

	errCh := c.mc.RemoveObjects(ctx, bucket, objects, minio.RemoveObjectsOptions{})
	for e := range errCh {
		if e.Err != nil {
			return fmt.Errorf("s3: drain bucket %q: %w", bucket, e.Err)
		}
	}
	return nil
}

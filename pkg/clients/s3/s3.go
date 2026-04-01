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

// CreateBucket creates a new bucket in the given region.
func (c *Client) CreateBucket(ctx context.Context, bucket, region string) error {
	if err := c.mc.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: region}); err != nil {
		return fmt.Errorf("s3: create bucket %q: %w", bucket, err)
	}
	return nil
}

// DeleteBucket drains and removes a bucket. Returns nil if the bucket does not exist.
func (c *Client) DeleteBucket(ctx context.Context, bucket string) error {
	objects := c.mc.ListObjects(ctx, bucket, minio.ListObjectsOptions{Recursive: true})
	errCh := c.mc.RemoveObjects(ctx, bucket, objects, minio.RemoveObjectsOptions{})
	for e := range errCh {
		if e.Err != nil {
			return fmt.Errorf("s3: drain bucket %q: %w", bucket, e.Err)
		}
	}
	if err := c.mc.RemoveBucket(ctx, bucket); err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchBucket" {
			return nil
		}
		return fmt.Errorf("s3: remove bucket %q: %w", bucket, err)
	}
	return nil
}

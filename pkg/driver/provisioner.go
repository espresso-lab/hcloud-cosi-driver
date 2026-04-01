package driver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"

	cosi "sigs.k8s.io/container-object-storage-interface/proto"

	"github.com/espresso-lab/hcloud-cosi-driver/pkg/clients/s3"
)

// endpointForLocation constructs the Hetzner Object Storage S3 endpoint for a given location name.
func endpointForLocation(location string) string {
	return "https://" + location + ".your-objectstorage.com"
}

// s3HostForLocation returns the host (without scheme) for use with the MinIO client.
func s3HostForLocation(location string) string {
	return location + ".your-objectstorage.com"
}

// parseBucketID splits a "<location>:<bucket-name>" bucket ID into its parts.
func parseBucketID(bucketID string) (location, name string, err error) {
	parts := strings.SplitN(bucketID, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("expected <location>:<name>, got %q", bucketID)
	}
	return parts[0], parts[1], nil
}

// ProvisionerServer implements the COSI ProvisionerServer interface.
type ProvisionerServer struct {
	cosi.UnimplementedProvisionerServer

	AccessKey string
	SecretKey string
}

func (s *ProvisionerServer) s3ClientForLocation(location string) (*s3.Client, error) {
	return s3.New(s3HostForLocation(location), s.AccessKey, s.SecretKey)
}

func (s *ProvisionerServer) DriverCreateBucket(
	ctx context.Context,
	req *cosi.DriverCreateBucketRequest,
) (*cosi.DriverCreateBucketResponse, error) {
	name, err := bucketName(req.GetName())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate bucket name: %v", err)
	}

	location := "fsn1"
	if loc := strings.ToLower(req.GetParameters()["location"]); loc != "" {
		location = loc
	}

	s3Client, err := s.s3ClientForLocation(location)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create s3 client: %v", err)
	}

	if err := s3Client.CreateBucket(ctx, name, location); err != nil {
		klog.ErrorS(err, "Failed to create bucket", "bucket", name)
		return nil, status.Errorf(codes.Internal, "create bucket: %v", err)
	}

	klog.InfoS("Bucket created", "bucket", name, "location", location)
	return &cosi.DriverCreateBucketResponse{
		BucketId: location + ":" + name,
	}, nil
}

func (s *ProvisionerServer) DriverDeleteBucket(
	ctx context.Context,
	req *cosi.DriverDeleteBucketRequest,
) (*cosi.DriverDeleteBucketResponse, error) {
	location, name, err := parseBucketID(req.GetBucketId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid bucket id: %v", err)
	}

	s3Client, err := s.s3ClientForLocation(location)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create s3 client: %v", err)
	}

	if err := s3Client.DeleteBucket(ctx, name); err != nil {
		klog.ErrorS(err, "Failed to delete bucket", "bucket", name)
		return nil, status.Errorf(codes.Internal, "delete bucket: %v", err)
	}

	klog.InfoS("Bucket deleted", "bucket", name)
	return &cosi.DriverDeleteBucketResponse{}, nil
}

func (s *ProvisionerServer) DriverGrantBucketAccess(
	_ context.Context,
	req *cosi.DriverGrantBucketAccessRequest,
) (*cosi.DriverGrantBucketAccessResponse, error) {
	location, bucketName, err := parseBucketID(req.GetBucketId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid bucket id: %v", err)
	}

	return &cosi.DriverGrantBucketAccessResponse{
		AccountId: req.GetName(),
		Credentials: map[string]*cosi.CredentialDetails{
			"s3": {
				Secrets: map[string]string{
					"accessKeyID":     s.AccessKey,
					"accessSecretKey": s.SecretKey,
					"endpoint":        endpointForLocation(location),
					"bucketName":      bucketName,
				},
			},
		},
	}, nil
}

func (s *ProvisionerServer) DriverRevokeBucketAccess(
	_ context.Context,
	_ *cosi.DriverRevokeBucketAccessRequest,
) (*cosi.DriverRevokeBucketAccessResponse, error) {
	return &cosi.DriverRevokeBucketAccessResponse{}, nil
}

// bucketName generates "<requested>-<8 uppercase hex chars>".
func bucketName(requested string) (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	return requested + "-" + strings.ToUpper(hex.EncodeToString(b)), nil
}

package driver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"

	cosi "sigs.k8s.io/container-object-storage-interface/proto"

	"github.com/espresso-lab/hcloud-cosi-driver/pkg/clients/hcloud"
)

// locationIDs maps Hetzner location names to their numeric API IDs.
var locationIDs = map[string]int{
	"fsn1": 1,
	"nbg1": 2,
	"hel1": 3,
}

// endpointForLocation constructs the Hetzner Object Storage S3 endpoint for a given location name.
func endpointForLocation(location string) string {
	return "https://" + location + ".your-objectstorage.com"
}

// parseBucketID splits a "<location>:<numeric-id>:<bucket-name>" bucket ID into its parts.
func parseBucketID(bucketID string) (location string, id int, name string, err error) {
	parts := strings.SplitN(bucketID, ":", 3)
	if len(parts) != 3 {
		return "", 0, "", fmt.Errorf("expected <location>:<id>:<name>, got %q", bucketID)
	}
	id, err = strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, "", fmt.Errorf("invalid numeric id in %q: %w", bucketID, err)
	}
	return parts[0], id, parts[2], nil
}

// ProvisionerServer implements the COSI ProvisionerServer interface.
type ProvisionerServer struct {
	cosi.UnimplementedProvisionerServer

	HCloud    *hcloud.Client
	AccessKey string
	SecretKey string
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
		if _, ok := locationIDs[loc]; ok {
			location = loc
		}
	}

	id, bucketName, err := s.HCloud.CreateBucket(ctx, name, locationIDs[location])
	if err != nil {
		klog.ErrorS(err, "Failed to create bucket", "bucket", name)
		return nil, status.Errorf(codes.Internal, "create bucket: %v", err)
	}

	klog.InfoS("Bucket created", "bucket", bucketName, "id", id, "location", location)
	return &cosi.DriverCreateBucketResponse{
		BucketId: location + ":" + strconv.Itoa(id) + ":" + bucketName,
	}, nil
}

func (s *ProvisionerServer) DriverDeleteBucket(
	ctx context.Context,
	req *cosi.DriverDeleteBucketRequest,
) (*cosi.DriverDeleteBucketResponse, error) {
	_, id, _, err := parseBucketID(req.GetBucketId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid bucket id: %v", err)
	}

	if err := s.HCloud.DeleteBucket(ctx, id); err != nil {
		klog.ErrorS(err, "Failed to delete bucket", "id", id)
		return nil, status.Errorf(codes.Internal, "delete bucket: %v", err)
	}

	klog.InfoS("Bucket deleted", "id", id)
	return &cosi.DriverDeleteBucketResponse{}, nil
}

func (s *ProvisionerServer) DriverGrantBucketAccess(
	_ context.Context,
	req *cosi.DriverGrantBucketAccessRequest,
) (*cosi.DriverGrantBucketAccessResponse, error) {
	location, _, bucketName, err := parseBucketID(req.GetBucketId())
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

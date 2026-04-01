package hcloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"k8s.io/klog/v2"
)

const apiBase = "https://api.hetzner.cloud"

// Client calls the Hetzner Cloud API using a bearer token.
type Client struct {
	token      string
	httpClient *http.Client
}

// New returns a new Client.
func New(token string) *Client {
	return &Client{token: token, httpClient: &http.Client{}}
}

// CreateBucket creates an object storage bucket and returns its numeric ID and name.
func (c *Client) CreateBucket(ctx context.Context, name string, locationID int) (int, string, error) {
	body, _ := json.Marshal(map[string]any{
		"name":                name,
		"public":              false,
		"location":            locationID,
		"object_lock_enabled": false,
	})

	var resp struct {
		Bucket struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"bucket"`
	}
	if err := c.do(ctx, http.MethodPost, "/v1/_buckets", body, &resp); err != nil {
		return 0, "", err
	}
	return resp.Bucket.ID, resp.Bucket.Name, nil
}

// DeleteBucket deletes a bucket by its numeric ID. A 404 is treated as success.
func (c *Client) DeleteBucket(ctx context.Context, bucketID int) error {
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("/v1/_buckets/%d", bucketID), nil, nil)
}

// CreateObjectStorageCredentials creates a new credential pair and returns id, accessKey, secretKey.
func (c *Client) CreateObjectStorageCredentials(ctx context.Context, description string) (int, string, string, error) {
	body, _ := json.Marshal(map[string]string{"description": description})

	var resp struct {
		Credential struct {
			ID        int    `json:"id"`
			AccessKey string `json:"access_key"`
		} `json:"object_storage_credential"`
		SecretKey string `json:"secret_key"`
	}
	if err := c.do(ctx, http.MethodPost, "/v1/_object_storage_credentials", body, &resp); err != nil {
		return 0, "", "", err
	}
	if resp.Credential.AccessKey == "" || resp.SecretKey == "" {
		err := fmt.Errorf("hcloud API returned empty credentials")
		klog.ErrorS(err, "CreateObjectStorageCredentials returned empty access key or secret key")
		return 0, "", "", err
	}
	return resp.Credential.ID, resp.Credential.AccessKey, resp.SecretKey, nil
}

func (c *Client) do(ctx context.Context, method, path string, body []byte, out any) error {
	var bodyReader *bytes.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	} else {
		bodyReader = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, apiBase+path, bodyReader)
	if err != nil {
		klog.ErrorS(err, "Failed to build HCloud API request", "method", method, "path", path)
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		klog.ErrorS(err, "HCloud API request error", "method", method, "path", path)
		return err
	}
	defer res.Body.Close() //nolint:errcheck

	if res.StatusCode == http.StatusNotFound {
		return nil // treat 404 as success (idempotent deletes)
	}
	if res.StatusCode >= 300 {
		err := fmt.Errorf("hcloud API %s %s: status %d", method, path, res.StatusCode)
		klog.ErrorS(err, "HCloud API request failed", "method", method, "path", path, "status", res.StatusCode)
		return err
	}
	if out != nil {
		return json.NewDecoder(res.Body).Decode(out)
	}
	return nil
}

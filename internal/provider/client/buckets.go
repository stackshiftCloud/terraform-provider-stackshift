package client

import (
	"context"
	"errors"
	"net/http"
	"net/url"
)

// CreateBucket creates an S2 bucket and returns its one-time access credentials.
func (c *Client) CreateBucket(ctx context.Context, req CreateBucketRequest) (*Bucket, *BucketCredentials, error) {
	var data struct {
		Bucket      Bucket            `json:"bucket"`
		Credentials BucketCredentials `json:"credentials"`
	}
	err := c.do(ctx, http.MethodPost, "/buckets/", req, &data)
	return &data.Bucket, &data.Credentials, err
}

// GetBucket reads an S2 bucket by its control-plane UUID.
func (c *Client) GetBucket(ctx context.Context, id string) (*Bucket, error) {
	var data struct {
		Bucket Bucket `json:"bucket"`
	}
	err := c.do(ctx, http.MethodGet, "/buckets/"+url.PathEscape(id)+"/", nil, &data)
	return &data.Bucket, err
}

// DeleteBucket removes an S2 bucket. force permits deletion of contained objects.
func (c *Client) DeleteBucket(ctx context.Context, id string, force bool) error {
	path := "/buckets/" + url.PathEscape(id) + "/"
	if force {
		path += "?force=true"
	}
	err := c.do(ctx, http.MethodDelete, path, nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

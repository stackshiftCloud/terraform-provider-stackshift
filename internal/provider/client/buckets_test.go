package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateBucket(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/buckets/" {
			t.Fatalf("request = %s %s, want POST /api/v1/buckets/", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sspat_test" {
			t.Fatalf("authorization = %q", got)
		}
		var request CreateBucketRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Name != "customer-uploads" || request.ProjectID == nil || *request.ProjectID != "project-1" {
			t.Fatalf("unexpected request: %#v", request)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"bucket": map[string]any{
					"id": "bucket-1", "name": "customer-uploads", "region": "global",
					"visibility": "private", "project_id": "project-1", "endpoint": "https://storage.stackshift.cloud",
				},
				"credentials": map[string]any{
					"access_key_id": "access-1", "secret_access_key": "secret-1",
					"endpoint": "https://storage.stackshift.cloud", "region": "global",
				},
			},
		})
	}))
	defer server.Close()

	projectID := "project-1"
	bucket, credentials, err := New(server.URL, "sspat_test", server.Client()).CreateBucket(
		context.Background(),
		CreateBucketRequest{Name: "customer-uploads", Region: "global", Visibility: "private", ProjectID: &projectID},
	)
	if err != nil {
		t.Fatal(err)
	}
	if bucket.ID != "bucket-1" || credentials.AccessKeyID != "access-1" || credentials.SecretAccessKey != "secret-1" {
		t.Fatalf("bucket = %#v, credentials = %#v", bucket, credentials)
	}
}

func TestGetBucket(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/buckets/bucket-1/" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{"bucket": map[string]any{
				"id": "bucket-1", "name": "customer-uploads", "region": "global", "visibility": "private",
			}},
		})
	}))
	defer server.Close()

	bucket, err := New(server.URL, "sspat_test", server.Client()).GetBucket(context.Background(), "bucket-1")
	if err != nil {
		t.Fatal(err)
	}
	if bucket.ID != "bucket-1" || bucket.Name != "customer-uploads" {
		t.Fatalf("bucket = %#v", bucket)
	}
}

func TestDeleteBucket(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		force      bool
		rawQuery   string
		statusCode int
	}{
		{name: "empty bucket", statusCode: http.StatusOK},
		{name: "force destroy", force: true, rawQuery: "force=true", statusCode: http.StatusOK},
		{name: "already absent", statusCode: http.StatusNotFound},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/buckets/bucket-1/" || r.URL.RawQuery != test.rawQuery {
					t.Fatalf("request = %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
				}
				w.WriteHeader(test.statusCode)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"success": test.statusCode == http.StatusOK,
					"message": "bucket not found",
				})
			}))
			defer server.Close()

			if err := New(server.URL, "sspat_test", server.Client()).DeleteBucket(context.Background(), "bucket-1", test.force); err != nil {
				t.Fatal(err)
			}
		})
	}
}

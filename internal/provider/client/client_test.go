package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoSendsBearerAndDecodesEnvelope(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/projects" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sspat_test" {
			t.Fatalf("authorization = %q", got)
		}
		var body CreateProjectRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Name != "demo" || body.Runtime != "node" {
			t.Fatalf("unexpected body: %#v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":     true,
			"status":      "Created",
			"status_code": 201,
			"message":     "Resource created successfully",
			"data": map[string]any{
				"id":      "project-1",
				"name":    "demo",
				"runtime": "node",
				"region":  "us-east",
			},
		})
	}))
	defer server.Close()

	c := New(server.URL, "sspat_test", server.Client())
	project, err := c.CreateProject(context.Background(), CreateProjectRequest{Name: "demo", Runtime: "node", Region: "us-east"})
	if err != nil {
		t.Fatal(err)
	}
	if project.ID != "project-1" || project.Name != "demo" {
		t.Fatalf("decoded project = %#v", project)
	}
}

func TestDoMapsNotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":     false,
			"status":      "Not Found",
			"status_code": 404,
			"message":     "project not found",
			"error": map[string]any{
				"code": "Not Found",
			},
		})
	}))
	defer server.Close()

	c := New(server.URL, "sspat_test", server.Client())
	_, err := c.GetProject(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestDoReturnsAPIErrorMessage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":     false,
			"status":      "Payment Required",
			"status_code": 402,
			"message":     "database limit reached",
		})
	}))
	defer server.Close()

	c := New(server.URL, "sspat_test", server.Client())
	_, err := c.GetDatabase(context.Background(), "db-1")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %T, want APIError", err)
	}
	if apiErr.StatusCode != http.StatusPaymentRequired || apiErr.Message != "database limit reached" {
		t.Fatalf("api err = %#v", apiErr)
	}
}

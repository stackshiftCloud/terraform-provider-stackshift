package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestWorkloadEgressGrantSendsIdempotencyKey(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/projects/project-1/security/grants" {
			t.Fatalf("request = %s %s, want POST grant route", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "tf-grant-key" {
			t.Fatalf("Idempotency-Key = %q, want tf-grant-key", got)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"id": "grant-1", "project_id": "project-1", "environment": "production",
			"protocol": "tls", "port_start": 443, "port_end": 443,
			"purpose": "payment API", "expires_at": "2026-09-01T00:00:00Z",
			"status": "pending", "created_at": "2026-08-28T00:00:00Z",
			"updated_at": "2026-08-28T00:00:00Z",
		}); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	grant, err := New(server.URL, "sspat_test", server.Client()).RequestWorkloadEgressGrant(
		context.Background(), "project-1", "tf-grant-key", RequestWorkloadEgressGrant{
			Environment: "production", Protocol: "tls", PortStart: 443, PortEnd: 443,
			Purpose: "payment API", ExpiresAt: "2026-09-01T00:00:00Z",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if grant.ID != "grant-1" {
		t.Fatalf("grant ID = %q, want grant-1", grant.ID)
	}
}

func TestDeleteWorkloadSecurityResources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		call func(context.Context, *Client) error
	}{
		{
			name: "disable dependency",
			path: "/api/v1/projects/project-1/security/dependencies/dependency-1",
			call: func(ctx context.Context, client *Client) error {
				return client.DisableWorkloadDependency(ctx, "project-1", "dependency-1")
			},
		},
		{
			name: "revoke grant",
			path: "/api/v1/projects/project-1/security/grants/grant-1",
			call: func(ctx context.Context, client *Client) error {
				return client.RevokeWorkloadEgressGrant(ctx, "project-1", "grant-1")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete || r.URL.Path != test.path {
					t.Fatalf("request = %s %s, want DELETE %s", r.Method, r.URL.Path, test.path)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			if err := test.call(context.Background(), New(server.URL, "sspat_test", server.Client())); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDeleteWorkloadSecurityResourcesTreatsNotFoundAsSuccess(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := New(server.URL, "sspat_test", server.Client())
	if err := client.DisableWorkloadDependency(context.Background(), "project-1", "missing"); err != nil {
		t.Fatal(err)
	}
	if err := client.RevokeWorkloadEgressGrant(context.Background(), "project-1", "missing"); err != nil {
		t.Fatal(err)
	}
}

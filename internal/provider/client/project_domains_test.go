package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProjectDomainsUseCustomDomainEndpoints(t *testing.T) {
	t.Parallel()

	domain := map[string]any{"id": "pd-1", "project_id": "p-1", "hostname": "app.example.com", "verification_status": "verified", "ssl_status": "pending", "is_primary": true}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data any
		switch r.Method + " " + r.URL.Path {
		case "POST /api/v1/projects/p-1/custom-domains/external":
			var request CreateDomainRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Hostname != "app.example.com" || !request.IsPrimary || request.IncludeWWW {
				t.Fatalf("create request = %#v, %v", request, err)
			}
			data = domain
		case "POST /api/v1/projects/p-1/custom-domains/pd-1/verify":
			data = domain
		case "GET /api/v1/projects/p-1/custom-domains":
			data = map[string]any{"domains": []any{domain}}
		case "DELETE /api/v1/projects/p-1/custom-domains/pd-1":
			data = map[string]any{"message": "domain detached from project"}
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data})
	}))
	defer server.Close()

	c := New(server.URL, "sspat_test", server.Client())
	ctx := context.Background()
	created, err := c.CreateProjectDomain(ctx, "p-1", "app.example.com", true)
	if err != nil || created.Hostname != "app.example.com" || created.VerificationStatus != "verified" {
		t.Fatalf("create = %#v, %v", created, err)
	}
	if verified, err := c.VerifyProjectDomain(ctx, "p-1", "pd-1"); err != nil || verified.ID != "pd-1" {
		t.Fatalf("verify = %#v, %v", verified, err)
	}
	if listed, err := c.ListProjectDomains(ctx, "p-1"); err != nil || len(listed) != 1 || !listed[0].IsPrimary {
		t.Fatalf("list = %#v, %v", listed, err)
	}
	if err := c.DeleteProjectDomain(ctx, "p-1", "pd-1"); err != nil {
		t.Fatal(err)
	}
}

func TestDeletingAnAlreadyRemovedProjectDomainSucceeds(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "project domain not found"})
	}))
	defer server.Close()

	if err := New(server.URL, "sspat_test", server.Client()).DeleteProjectDomain(context.Background(), "p-1", "gone"); err != nil {
		t.Fatalf("delete of a removed domain = %v", err)
	}
}

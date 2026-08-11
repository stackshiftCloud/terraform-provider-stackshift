package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMailDomainLifecycleUsesCurrentAPIContract(t *testing.T) {
	t.Parallel()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch requests {
		case 1:
			if r.Method != http.MethodPost || r.URL.Path != "/api/v1/mail/domains" {
				t.Fatalf("create request = %s %s", r.Method, r.URL.Path)
			}
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["domain"] != "example.com" {
				t.Fatalf("create body = %#v", body)
			}
			writeMailTestData(t, w, map[string]any{"id": "domain-1", "domain": "example.com", "status": "pending"})
		case 2:
			if r.Method != http.MethodPost || r.URL.Path != "/api/v1/mail/domains/domain-1/verify" {
				t.Fatalf("verify request = %s %s", r.Method, r.URL.Path)
			}
			writeMailTestData(t, w, map[string]any{"id": "domain-1", "domain": "example.com", "status": "verified"})
		case 3:
			if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/mail/domains/domain-1" {
				t.Fatalf("delete request = %s %s", r.Method, r.URL.Path)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request %d", requests)
		}
	}))
	defer server.Close()

	client := New(server.URL, "sspat_test", server.Client())
	created, err := client.CreateMailDomain(context.Background(), "example.com", "")
	if err != nil {
		t.Fatal(err)
	}
	verified, err := client.VerifyMailDomain(context.Background(), created.ID)
	if err != nil || verified.Status != "verified" {
		t.Fatalf("verified = %#v, err = %v", verified, err)
	}
	if err := client.DeleteMailDomain(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
}

func TestMailWebhookUsesCamelCaseEventContract(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/mail/webhooks" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		var body struct {
			URL        string   `json:"url"`
			EventTypes []string `json:"eventTypes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.URL != "https://hooks.example.com/mail" || len(body.EventTypes) != 1 ||
			body.EventTypes[0] != "mail.delivery.delivered" {
			t.Fatalf("create body = %#v", body)
		}
		writeMailTestData(t, w, map[string]any{
			"id": "wh_1", "url": body.URL, "status": "active",
			"eventTypes": body.EventTypes, "secret": "secret",
		})
	}))
	defer server.Close()

	client := New(server.URL, "sspat_test", server.Client())
	webhook, err := client.CreateMailWebhook(
		context.Background(),
		"https://hooks.example.com/mail",
		nil,
		[]string{"mail.delivery.delivered"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if webhook.ID != "wh_1" || webhook.Secret != "secret" {
		t.Fatalf("webhook = %#v", webhook)
	}
}

func writeMailTestData(t *testing.T, w http.ResponseWriter, data any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data}); err != nil {
		t.Fatal(err)
	}
}

package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	for name, test := range map[string]struct {
		value string
		want  time.Duration
	}{
		"seconds":       {"7", 7 * time.Second},
		"padded":        {" 3 ", 3 * time.Second},
		"http date":     {now.Add(90 * time.Second).Format(http.TimeFormat), 90 * time.Second},
		"past date":     {now.Add(-time.Minute).Format(http.TimeFormat), 0},
		"negative":      {"-5", 0},
		"empty":         {"", 0},
		"unparseable":   {"soon", 0},
		"huge":          {"99999999999999999999", maxRetryAfter},
		"huge negative": {"-99999999999999999999", 0},
		"large":         {"9223372036854775807", maxRetryAfter},
		"far date":      {time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC).Format(http.TimeFormat), maxRetryAfter},
	} {
		if got := parseRetryAfter(test.value, now); got != test.want {
			t.Errorf("%s: parseRetryAfter(%q) = %v, want %v", name, test.value, got, test.want)
		}
	}
}

func TestDedicatedErrorCarriesRetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "4")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	_, err := New(server.URL, "sspat_test", nil).GetDedicatedDatabase(context.Background(), "db-1")
	var apiErr *DedicatedError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTooManyRequests || apiErr.RetryAfter != 4*time.Second {
		t.Fatalf("err = %#v", err)
	}
}

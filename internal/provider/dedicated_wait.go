package provider

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var dedicatedPollInterval = 5 * time.Second

const dedicatedWaitLimit = 45 * time.Minute

func tlsFailure(err error) bool {
	var unknownAuthority x509.UnknownAuthorityError
	var invalidCertificate x509.CertificateInvalidError
	var wrongHostname x509.HostnameError
	var recordHeader tls.RecordHeaderError
	var verification *tls.CertificateVerificationError
	return errors.As(err, &unknownAuthority) || errors.As(err, &invalidCertificate) || errors.As(err, &wrongHostname) ||
		errors.As(err, &recordHeader) || errors.As(err, &verification)
}

func retryablePollError(err error) bool {
	var apiErr *client.DedicatedError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode >= http.StatusInternalServerError || apiErr.StatusCode == http.StatusTooManyRequests || apiErr.StatusCode == http.StatusRequestTimeout
	}
	if tlsFailure(err) {
		return false
	}
	var transportErr *url.Error
	return errors.As(err, &transportErr)
}

func pollDelay(err error) time.Duration {
	var apiErr *client.DedicatedError
	if errors.As(err, &apiErr) && apiErr.RetryAfter > 0 {
		return apiErr.RetryAfter
	}
	return dedicatedPollInterval
}

func pollDedicated(ctx context.Context, timeout time.Duration, done func(context.Context) (bool, error)) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var lastErr error
	for {
		finished, err := done(ctx)
		if finished && err == nil {
			return nil
		}
		if err != nil && !retryablePollError(err) {
			return err
		}
		lastErr = err
		delay := pollDelay(err)
		if deadline, ok := ctx.Deadline(); ok {
			if remaining := time.Until(deadline); delay > remaining {
				delay = remaining
			}
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			if lastErr != nil {
				return fmt.Errorf("%w; the last attempt failed: %v", ctx.Err(), lastErr)
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func dedicatedTransitional(status string) bool {
	switch status {
	case "provisioning", "starting", "restarting", "restoring":
		return true
	}
	return false
}

func waitDedicatedRunning(ctx context.Context, api *client.Client, id string, timeout time.Duration) error {
	return pollDedicated(ctx, timeout, func(ctx context.Context) (bool, error) {
		db, err := api.GetDedicatedDatabase(ctx, id)
		if err != nil {
			return false, err
		}
		if db.Status == "running" {
			return true, nil
		}
		if dedicatedTransitional(db.Status) {
			return false, nil
		}
		return false, &client.DedicatedError{StatusCode: http.StatusConflict, Message: "The database is " + db.Status + ", so it can't take this change yet."}
	})
}

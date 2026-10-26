package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var dedicatedPollInterval = 5 * time.Second

const dedicatedWaitLimit = 45 * time.Minute

func retryablePollError(err error) bool {
	var apiErr *client.DedicatedError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode >= http.StatusInternalServerError
	}
	var transportErr *url.Error
	return errors.As(err, &transportErr)
}

func pollDedicated(ctx context.Context, timeout time.Duration, done func(context.Context) (bool, error)) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(dedicatedPollInterval)
	defer ticker.Stop()
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
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("%w; the last attempt failed: %v", ctx.Err(), lastErr)
			}
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func waitDedicatedRunning(ctx context.Context, api *client.Client, id string, timeout time.Duration) error {
	return pollDedicated(ctx, timeout, func(ctx context.Context) (bool, error) {
		db, err := api.GetDedicatedDatabase(ctx, id)
		if err != nil {
			return false, err
		}
		switch db.Status {
		case "running":
			return true, nil
		case "provisioning", "starting", "restarting", "restoring":
			return false, nil
		}
		return false, &client.DedicatedError{StatusCode: http.StatusConflict, Message: "The database is " + db.Status + ", so it can't take this change yet."}
	})
}

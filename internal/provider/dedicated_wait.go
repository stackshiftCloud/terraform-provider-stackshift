package provider

import (
	"context"
	"net/http"
	"time"

	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var dedicatedPollInterval = 5 * time.Second

const dedicatedWaitLimit = 45 * time.Minute

func pollDedicated(ctx context.Context, done func(context.Context) (bool, error)) error {
	ctx, cancel := context.WithTimeout(ctx, dedicatedWaitLimit)
	defer cancel()
	ticker := time.NewTicker(dedicatedPollInterval)
	defer ticker.Stop()
	for {
		finished, err := done(ctx)
		if err != nil || finished {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func waitDedicatedRunning(ctx context.Context, api *client.Client, id string) error {
	return pollDedicated(ctx, func(ctx context.Context) (bool, error) {
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

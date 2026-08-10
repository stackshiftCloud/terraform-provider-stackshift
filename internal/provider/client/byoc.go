package client

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"
)

func idempotencyHeader(key string) map[string]string {
	return map[string]string{"Idempotency-Key": key}
}

func (c *Client) WaitBYOCOperation(ctx context.Context, id string) (*BYOCOperation, error) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		operation, err := c.GetBYOCOperation(ctx, id)
		if err != nil {
			return nil, err
		}
		switch operation.Status {
		case "active", "completed", "deleted":
			return operation, nil
		case "failed":
			return operation, &APIError{StatusCode: http.StatusUnprocessableEntity, Code: operation.ErrorCode, Message: operation.ErrorSummary}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (c *Client) ListBYOCProviderConnections(ctx context.Context) ([]BYOCProviderConnection, error) {
	var result struct {
		Connections []BYOCProviderConnection `json:"connections"`
	}
	err := c.do(ctx, http.MethodGet, "/provider-connections/", nil, &result)
	return result.Connections, err
}

func (c *Client) GetBYOCProviderConnection(ctx context.Context, id string) (*BYOCProviderConnection, error) {
	connections, err := c.ListBYOCProviderConnections(ctx)
	if err != nil {
		return nil, err
	}
	for i := range connections {
		if connections[i].ID == id {
			return &connections[i], nil
		}
	}
	return nil, ErrNotFound
}

func (c *Client) CreateBYOCProviderConnection(ctx context.Context, request BYOCProviderConnectionRequest) (*BYOCProviderConnection, error) {
	var connection BYOCProviderConnection
	err := c.do(ctx, http.MethodPost, "/provider-connections/", request, &connection)
	return &connection, err
}

func (c *Client) UpdateBYOCProviderConnection(ctx context.Context, id string, request BYOCProviderConnectionRequest) (*BYOCProviderConnection, error) {
	var connection BYOCProviderConnection
	err := c.do(ctx, http.MethodPatch, "/provider-connections/"+url.PathEscape(id), request, &connection)
	return &connection, err
}

func (c *Client) ValidateBYOCProviderConnection(ctx context.Context, id string) (*BYOCProviderConnection, error) {
	var connection BYOCProviderConnection
	err := c.do(ctx, http.MethodPost, "/provider-connections/"+url.PathEscape(id)+"/validate", nil, &connection)
	return &connection, err
}

func (c *Client) DeleteBYOCProviderConnection(ctx context.Context, id string) error {
	err := c.do(ctx, http.MethodDelete, "/provider-connections/"+url.PathEscape(id), nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func (c *Client) ProvisionBYOCNode(ctx context.Context, request map[string]any, idempotencyKey string) (*BYOCNode, *BYOCOperation, error) {
	var result struct {
		Node      BYOCNode      `json:"node"`
		Operation BYOCOperation `json:"operation"`
	}
	err := c.doWithHeaders(ctx, http.MethodPost, "/byocloud/nodes", request, idempotencyHeader(idempotencyKey), &result)
	return &result.Node, &result.Operation, err
}

func (c *Client) GetBYOCNode(ctx context.Context, id string) (*BYOCNode, error) {
	var node BYOCNode
	err := c.do(ctx, http.MethodGet, "/nodes/"+url.PathEscape(id), nil, &node)
	return &node, err
}

func (c *Client) GetBYOCOperation(ctx context.Context, id string) (*BYOCOperation, error) {
	var result struct {
		Operation BYOCOperation `json:"operation"`
	}
	err := c.do(ctx, http.MethodGet, "/byocloud/operations/"+url.PathEscape(id), nil, &result)
	return &result.Operation, err
}

func (c *Client) DeleteBYOCNode(ctx context.Context, id, idempotencyKey string) (*BYOCOperation, error) {
	var result struct {
		Operation BYOCOperation `json:"operation"`
	}
	err := c.doWithHeaders(ctx, http.MethodDelete, "/byocloud/nodes/"+url.PathEscape(id), nil, idempotencyHeader(idempotencyKey), &result)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	return &result.Operation, err
}

func (c *Client) GetBYOCNodeResources(ctx context.Context, nodeID string) (*BYOCNodeResourceSummary, error) {
	var result BYOCNodeResourceSummary
	err := c.do(ctx, http.MethodGet, "/byocloud/nodes/"+url.PathEscape(nodeID)+"/resources", nil, &result)
	return &result, err
}

func (c *Client) CreateBYOCVolume(ctx context.Context, nodeID, idempotencyKey string, request map[string]any) (*BYOCVolume, error) {
	var volume BYOCVolume
	err := c.doWithHeaders(ctx, http.MethodPost, "/byocloud/nodes/"+url.PathEscape(nodeID)+"/volumes", request, idempotencyHeader(idempotencyKey), &volume)
	return &volume, err
}

func (c *Client) DeleteBYOCVolume(ctx context.Context, id, idempotencyKey string) error {
	err := c.doWithHeaders(ctx, http.MethodDelete, "/byocloud/volumes/"+url.PathEscape(id), nil, idempotencyHeader(idempotencyKey), nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func (c *Client) CreateBYOCSnapshot(ctx context.Context, volumeID, idempotencyKey, name string) (*BYOCSnapshot, error) {
	var snapshot BYOCSnapshot
	err := c.doWithHeaders(ctx, http.MethodPost, "/byocloud/volumes/"+url.PathEscape(volumeID)+"/snapshots", map[string]string{"name": name}, idempotencyHeader(idempotencyKey), &snapshot)
	return &snapshot, err
}

func (c *Client) DeleteBYOCSnapshot(ctx context.Context, id, idempotencyKey string) error {
	err := c.doWithHeaders(ctx, http.MethodDelete, "/byocloud/snapshots/"+url.PathEscape(id), nil, idempotencyHeader(idempotencyKey), nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func (c *Client) AllocateBYOCStaticIP(ctx context.Context, nodeID, idempotencyKey string) (*BYOCStaticIP, error) {
	var address BYOCStaticIP
	err := c.doWithHeaders(ctx, http.MethodPost, "/byocloud/nodes/"+url.PathEscape(nodeID)+"/static-ip", nil, idempotencyHeader(idempotencyKey), &address)
	return &address, err
}

func (c *Client) ReleaseBYOCStaticIP(ctx context.Context, nodeID, idempotencyKey string) error {
	err := c.doWithHeaders(ctx, http.MethodDelete, "/byocloud/nodes/"+url.PathEscape(nodeID)+"/static-ip", nil, idempotencyHeader(idempotencyKey), nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

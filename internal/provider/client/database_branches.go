package client

import (
	"context"
	"net/http"
	"net/url"
)

type DatabaseSleep struct {
	Enabled      bool   `json:"enabled"`
	AfterMinutes int64  `json:"after_minutes"`
	State        string `json:"state"`
}

type DatabaseBranchSource struct {
	Kind           string `json:"kind"`
	Time           string `json:"time"`
	RestorePointID string `json:"restore_point_id"`
	BackupID       string `json:"backup_id"`
}

type DatabaseBranchDatabase struct {
	PrivateHostname string         `json:"private_hostname"`
	Sleep           *DatabaseSleep `json:"sleep"`
}

type DatabaseBranch struct {
	ID               string                  `json:"id"`
	Name             string                  `json:"name"`
	ParentDatabaseID string                  `json:"parent_database_id"`
	RootDatabaseID   string                  `json:"root_database_id"`
	Mechanism        string                  `json:"mechanism"`
	Source           DatabaseBranchSource    `json:"source"`
	State            string                  `json:"state"`
	SourceRef        *string                 `json:"source_ref"`
	ExpiresAt        *string                 `json:"expires_at"`
	BranchedAt       string                  `json:"branched_at"`
	FailureMessage   *string                 `json:"failure_message"`
	Database         *DatabaseBranchDatabase `json:"database"`
}

type DatabaseBranchOperation struct {
	ID               string          `json:"id"`
	BranchDatabaseID string          `json:"branch_database_id"`
	ParentDatabaseID string          `json:"parent_database_id"`
	Kind             string          `json:"kind"`
	Status           string          `json:"status"`
	ErrorCode        string          `json:"error_code"`
	ErrorMessage     string          `json:"error_message"`
	Branch           *DatabaseBranch `json:"branch"`
}

type CreateDatabaseBranchRequest struct {
	Name              string `json:"name"`
	From              any    `json:"from"`
	TTL               string `json:"ttl,omitempty"`
	SleepAfterMinutes int64  `json:"sleep_after_minutes,omitempty"`
	SourceRef         string `json:"source_ref,omitempty"`
}

func branchPath(parentID, suffix string) string {
	return dedicatedPath(parentID, "/branches"+suffix)
}

func (c *Client) CreateDatabaseBranch(ctx context.Context, parentID string, req CreateDatabaseBranchRequest) (*DatabaseBranchOperation, error) {
	var op DatabaseBranchOperation
	err := c.dedicatedDo(ctx, http.MethodPost, branchPath(parentID, ""), req, &op)
	return &op, err
}

func (c *Client) GetDatabaseBranchOperation(ctx context.Context, parentID, operationID string) (*DatabaseBranchOperation, error) {
	var op DatabaseBranchOperation
	err := c.dedicatedDo(ctx, http.MethodGet, branchPath(parentID, "/operations/"+url.PathEscape(operationID)), nil, &op)
	return &op, err
}

func (c *Client) GetDatabaseBranch(ctx context.Context, parentID, branchID string) (*DatabaseBranch, error) {
	var branch DatabaseBranch
	err := c.dedicatedDo(ctx, http.MethodGet, branchPath(parentID, "/"+url.PathEscape(branchID)), nil, &branch)
	return &branch, err
}

func (c *Client) UpdateDatabaseBranch(ctx context.Context, parentID, branchID string, changes map[string]any) (*DatabaseBranch, error) {
	var branch DatabaseBranch
	err := c.dedicatedDo(ctx, http.MethodPatch, branchPath(parentID, "/"+url.PathEscape(branchID)), changes, &branch)
	return &branch, err
}

func (c *Client) DeleteDatabaseBranch(ctx context.Context, parentID, branchID string) (*DatabaseBranchOperation, error) {
	var op DatabaseBranchOperation
	err := c.dedicatedDo(ctx, http.MethodDelete, branchPath(parentID, "/"+url.PathEscape(branchID)), nil, &op)
	return &op, err
}

func (c *Client) SetDatabaseSleep(ctx context.Context, id string, afterMinutes int64) error {
	body := map[string]any{"enabled": afterMinutes > 0}
	if afterMinutes > 0 {
		body["after_minutes"] = afterMinutes
	}
	return c.dedicatedDo(ctx, http.MethodPut, dedicatedPath(id, "/sleep"), body, nil)
}

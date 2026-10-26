package client

import (
	"context"
	"errors"
	"net/http"
	"net/url"
)

type DedicatedParameterGroup struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Family      string            `json:"family"`
	Description string            `json:"description"`
	Version     int64             `json:"version"`
	Parameters  map[string]string `json:"parameters"`
	Databases   int64             `json:"databases"`
}

type DedicatedParameterDefinition struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Min         *float64 `json:"min"`
	Max         *float64 `json:"max"`
	Values      []string `json:"values"`
	Restart     bool     `json:"restart"`
	EngineValue string   `json:"engine_default"`
	Description string   `json:"description"`
}

type DedicatedParameterGroupRef struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version int64  `json:"version"`
}

type DedicatedParameters struct {
	Family         string                      `json:"family"`
	Group          *DedicatedParameterGroupRef `json:"group"`
	Applied        bool                        `json:"applied"`
	PendingRestart []string                    `json:"pending_restart"`
	Error          string                      `json:"error"`
}

type DedicatedBackupSet struct {
	ID             string  `json:"id"`
	Repo           string  `json:"repo"`
	Kind           string  `json:"kind"`
	Name           *string `json:"name"`
	Status         string  `json:"status"`
	StartedAt      *string `json:"started_at"`
	FinishedAt     *string `json:"finished_at"`
	SizeBytes      int64   `json:"size_bytes"`
	FailureMessage *string `json:"failure_message"`
	CreatedAt      string  `json:"created_at"`
}

type DedicatedRestorableWindow struct {
	Earliest *string `json:"earliest"`
	Latest   *string `json:"latest"`
}

type DedicatedBackups struct {
	RetentionDays int64                     `json:"retention_days"`
	Restorable    DedicatedRestorableWindow `json:"restorable"`
	Backups       []DedicatedBackupSet      `json:"backups"`
}

type DedicatedRestorePoint struct {
	ID             string  `json:"id"`
	Kind           string  `json:"kind"`
	Name           *string `json:"name"`
	Status         string  `json:"status"`
	SizeGB         int64   `json:"size_gb"`
	FailureMessage *string `json:"failure_message"`
	TakenAt        *string `json:"taken_at"`
}

type DedicatedRestoreSchedule struct {
	Kind    string `json:"kind"`
	Enabled bool   `json:"enabled"`
	Keep    int64  `json:"keep"`
}

type DedicatedRestorePoints struct {
	Eligibility struct {
		Available bool   `json:"available"`
		Reason    string `json:"reason"`
	} `json:"eligibility"`
	Schedules     []DedicatedRestoreSchedule `json:"schedules"`
	RestorePoints []DedicatedRestorePoint    `json:"restore_points"`
}

type DedicatedRestoreScheduleChange struct {
	Enabled bool  `json:"enabled"`
	Keep    int64 `json:"keep"`
}

type DedicatedPublicAccessRequest struct {
	Transport                 string   `json:"transport"`
	AccessMode                string   `json:"access_mode"`
	AllowedCIDRs              []string `json:"allowed_cidrs,omitempty"`
	ExpiresAt                 string   `json:"expires_at,omitempty"`
	AcknowledgePublicExposure bool     `json:"acknowledge_public_exposure,omitempty"`
}

type DedicatedPublicAccessPolicy struct {
	ID             string   `json:"id"`
	Transport      string   `json:"transport"`
	AccessMode     string   `json:"access_mode"`
	Status         string   `json:"status"`
	AllowedCIDRs   []string `json:"allowed_cidrs"`
	ExpiresAt      string   `json:"expires_at"`
	PublicHostname string   `json:"public_hostname"`
	PublicPort     int64    `json:"public_port"`
	TLSMode        string   `json:"tls_mode"`
	ErrorMessage   *string  `json:"error_message"`
}

type DedicatedInsights struct {
	RetentionDays int64 `json:"retention_days"`
	IncludedDays  int64 `json:"included_days"`
}

func (c *Client) CreateDedicatedParameterGroup(ctx context.Context, name, family, description string, parameters map[string]string) (*DedicatedParameterGroup, error) {
	var group DedicatedParameterGroup
	err := c.dedicatedDo(ctx, http.MethodPost, "/databases/parameter-groups", map[string]any{
		"name": name, "family": family, "description": description, "parameters": parameters,
	}, &group)
	return &group, err
}

func (c *Client) GetDedicatedParameterGroup(ctx context.Context, id string) (*DedicatedParameterGroup, error) {
	var group DedicatedParameterGroup
	err := c.dedicatedDo(ctx, http.MethodGet, "/databases/parameter-groups/"+url.PathEscape(id), nil, &group)
	return &group, err
}

func (c *Client) UpdateDedicatedParameterGroup(ctx context.Context, id, name, description string, parameters map[string]string) (*DedicatedParameterGroup, error) {
	var group DedicatedParameterGroup
	err := c.dedicatedDo(ctx, http.MethodPatch, "/databases/parameter-groups/"+url.PathEscape(id), map[string]any{
		"name": name, "description": description, "parameters": parameters,
	}, &group)
	return &group, err
}

func (c *Client) DeleteDedicatedParameterGroup(ctx context.Context, id string) error {
	err := c.dedicatedDo(ctx, http.MethodDelete, "/databases/parameter-groups/"+url.PathEscape(id), nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func (c *Client) ListDedicatedParameterDefinitions(ctx context.Context, family string) ([]DedicatedParameterDefinition, error) {
	var result struct {
		Parameters []DedicatedParameterDefinition `json:"parameters"`
	}
	err := c.dedicatedDo(ctx, http.MethodGet, "/databases/parameter-definitions?family="+url.QueryEscape(family), nil, &result)
	return result.Parameters, err
}

func (c *Client) GetDedicatedParameters(ctx context.Context, id string) (*DedicatedParameters, error) {
	var parameters DedicatedParameters
	err := c.dedicatedDo(ctx, http.MethodGet, dedicatedPath(id, "/parameters"), nil, &parameters)
	return &parameters, err
}

func (c *Client) SetDedicatedParameterGroup(ctx context.Context, id string, groupID *string) error {
	return c.dedicatedDo(ctx, http.MethodPut, dedicatedPath(id, "/parameter-group"), map[string]any{"group_id": groupID}, nil)
}

func (c *Client) ApplyDedicatedParameters(ctx context.Context, id string) error {
	return c.dedicatedDo(ctx, http.MethodPost, dedicatedPath(id, "/parameters/apply"), nil, nil)
}

func (c *Client) ListDedicatedBackups(ctx context.Context, id string) (*DedicatedBackups, error) {
	var backups DedicatedBackups
	err := c.dedicatedDo(ctx, http.MethodGet, dedicatedPath(id, "/backups"), nil, &backups)
	return &backups, err
}

func (c *Client) CreateDedicatedSnapshot(ctx context.Context, id, name string) (*DedicatedBackupSet, error) {
	var result struct {
		Backup DedicatedBackupSet `json:"backup"`
	}
	err := c.dedicatedDo(ctx, http.MethodPost, dedicatedPath(id, "/backups"), map[string]any{"name": name}, &result)
	return &result.Backup, err
}

func (c *Client) GetDedicatedSnapshot(ctx context.Context, id, backupID string) (*DedicatedBackupSet, error) {
	var backup DedicatedBackupSet
	err := c.dedicatedDo(ctx, http.MethodGet, dedicatedPath(id, "/backups/"+url.PathEscape(backupID)), nil, &backup)
	return &backup, err
}

func (c *Client) DeleteDedicatedSnapshot(ctx context.Context, id, backupID string) error {
	err := c.dedicatedDo(ctx, http.MethodDelete, dedicatedPath(id, "/backups/"+url.PathEscape(backupID)), nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func (c *Client) ListDedicatedRestorePoints(ctx context.Context, id string) (*DedicatedRestorePoints, error) {
	var points DedicatedRestorePoints
	err := c.dedicatedDo(ctx, http.MethodGet, dedicatedPath(id, "/restore-points"), nil, &points)
	return &points, err
}

func (c *Client) CreateDedicatedRestorePoint(ctx context.Context, id, name string) (*DedicatedRestorePoint, error) {
	var result struct {
		RestorePoint DedicatedRestorePoint `json:"restore_point"`
	}
	err := c.dedicatedDo(ctx, http.MethodPost, dedicatedPath(id, "/restore-points"), map[string]any{"name": name}, &result)
	return &result.RestorePoint, err
}

func (c *Client) DeleteDedicatedRestorePoint(ctx context.Context, id, pointID string) error {
	err := c.dedicatedDo(ctx, http.MethodDelete, dedicatedPath(id, "/restore-points/"+url.PathEscape(pointID)), nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func (c *Client) UpdateDedicatedRestoreSchedules(ctx context.Context, id string, changes map[string]DedicatedRestoreScheduleChange) ([]DedicatedRestoreSchedule, error) {
	var result struct {
		Schedules []DedicatedRestoreSchedule `json:"schedules"`
	}
	err := c.dedicatedDo(ctx, http.MethodPut, dedicatedPath(id, "/restore-points/schedules"), changes, &result)
	return result.Schedules, err
}

func (c *Client) CreateDedicatedPublicAccess(ctx context.Context, id string, req DedicatedPublicAccessRequest) (*DedicatedPublicAccessPolicy, error) {
	var result struct {
		Policy DedicatedPublicAccessPolicy `json:"policy"`
	}
	err := c.dedicatedDo(ctx, http.MethodPost, dedicatedPath(id, "/public-access"), req, &result)
	return &result.Policy, err
}

func (c *Client) GetDedicatedPublicAccess(ctx context.Context, id, policyID string) (*DedicatedPublicAccessPolicy, error) {
	var result struct {
		Policy DedicatedPublicAccessPolicy `json:"policy"`
	}
	err := c.dedicatedDo(ctx, http.MethodGet, dedicatedPath(id, "/public-access/"+url.PathEscape(policyID)), nil, &result)
	return &result.Policy, err
}

func (c *Client) RevokeDedicatedPublicAccess(ctx context.Context, id, policyID string) error {
	err := c.dedicatedDo(ctx, http.MethodPost, dedicatedPath(id, "/public-access/revoke"), map[string]any{"policy_id": policyID}, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func (c *Client) GetDedicatedInsights(ctx context.Context, id string) (*DedicatedInsights, error) {
	var insights DedicatedInsights
	err := c.dedicatedDo(ctx, http.MethodGet, dedicatedPath(id, "/insights"), nil, &insights)
	return &insights, err
}

func (c *Client) SetDedicatedInsights(ctx context.Context, id string, retentionDays int64, confirmChargeMinor *int64) error {
	return c.dedicatedDo(ctx, http.MethodPut, dedicatedPath(id, "/insights"), map[string]any{
		"retention_days": retentionDays, "confirm_charge_minor": confirmChargeMinor,
	}, nil)
}

package client

import (
	"context"
	"errors"
	"net/http"
	"net/url"
)

type WorkloadExternalDependency struct {
	ID                   string `json:"id"`
	ProjectID            string `json:"project_id"`
	Environment          string `json:"environment"`
	Hostname             string `json:"hostname"`
	Port                 int64  `json:"port"`
	Protocol             string `json:"protocol"`
	Purpose              string `json:"purpose"`
	TLSRequired          bool   `json:"tls_required"`
	PrivateRouteRequired bool   `json:"private_route_required"`
	Detected             bool   `json:"detected"`
	Status               string `json:"status"`
	CreatedAt            string `json:"created_at"`
	UpdatedAt            string `json:"updated_at"`
}

type DeclareWorkloadDependencyRequest struct {
	Environment string `json:"environment"`
	Hostname    string `json:"hostname"`
	Port        int64  `json:"port"`
	Protocol    string `json:"protocol"`
	Purpose     string `json:"purpose"`
	TLSRequired bool   `json:"tls_required"`
}

type WorkloadEgressGrant struct {
	ID                string  `json:"id"`
	ProjectID         string  `json:"project_id"`
	DependencyID      *string `json:"dependency_id"`
	Environment       string  `json:"environment"`
	Protocol          string  `json:"protocol"`
	Hostname          *string `json:"hostname"`
	CIDR              *string `json:"cidr"`
	PortStart         int64   `json:"port_start"`
	PortEnd           int64   `json:"port_end"`
	Purpose           string  `json:"purpose"`
	Status            string  `json:"status"`
	BroadPublicEgress bool    `json:"broad_public_egress"`
	ExpiresAt         string  `json:"expires_at"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}

type RequestWorkloadEgressGrant struct {
	DependencyID      *string `json:"dependency_id,omitempty"`
	Environment       string  `json:"environment"`
	Protocol          string  `json:"protocol"`
	Hostname          *string `json:"hostname,omitempty"`
	CIDR              *string `json:"cidr,omitempty"`
	PortStart         int64   `json:"port_start"`
	PortEnd           int64   `json:"port_end"`
	Purpose           string  `json:"purpose"`
	BroadPublicEgress bool    `json:"broad_public_egress"`
	ExpiresAt         string  `json:"expires_at"`
}

func (c *Client) DeclareWorkloadDependency(
	ctx context.Context,
	projectID string,
	req DeclareWorkloadDependencyRequest,
) ([]WorkloadExternalDependency, error) {
	var result struct {
		Dependencies []WorkloadExternalDependency `json:"dependencies"`
	}
	err := c.do(ctx, http.MethodPost, workloadSecurityPath(projectID)+"/dependencies", req, &result)
	return result.Dependencies, err
}

func (c *Client) ListWorkloadDependencies(
	ctx context.Context,
	projectID, environment string,
) ([]WorkloadExternalDependency, error) {
	var result struct {
		Dependencies []WorkloadExternalDependency `json:"dependencies"`
	}
	path := workloadSecurityPath(projectID) + "/dependencies?environment=" + url.QueryEscape(environment)
	err := c.do(ctx, http.MethodGet, path, nil, &result)
	return result.Dependencies, err
}

func (c *Client) DisableWorkloadDependency(ctx context.Context, projectID, dependencyID string) error {
	err := c.do(ctx, http.MethodDelete,
		workloadSecurityPath(projectID)+"/dependencies/"+url.PathEscape(dependencyID), nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func (c *Client) RequestWorkloadEgressGrant(
	ctx context.Context,
	projectID string,
	idempotencyKey string,
	req RequestWorkloadEgressGrant,
) (*WorkloadEgressGrant, error) {
	var grant WorkloadEgressGrant
	err := c.doWithHeaders(
		ctx,
		http.MethodPost,
		workloadSecurityPath(projectID)+"/grants",
		req,
		idempotencyHeader(idempotencyKey),
		&grant,
	)
	return &grant, err
}

func (c *Client) ListWorkloadEgressGrants(
	ctx context.Context,
	projectID, environment string,
) ([]WorkloadEgressGrant, error) {
	var result struct {
		Grants []WorkloadEgressGrant `json:"grants"`
	}
	path := workloadSecurityPath(projectID) + "/grants?environment=" + url.QueryEscape(environment)
	err := c.do(ctx, http.MethodGet, path, nil, &result)
	return result.Grants, err
}

func (c *Client) RevokeWorkloadEgressGrant(ctx context.Context, projectID, grantID string) error {
	err := c.do(ctx, http.MethodDelete,
		workloadSecurityPath(projectID)+"/grants/"+url.PathEscape(grantID), nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func workloadSecurityPath(projectID string) string {
	return "/projects/" + url.PathEscape(projectID) + "/security"
}

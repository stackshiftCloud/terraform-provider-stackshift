package client

import (
	"context"
	"errors"
	"net/http"
	"net/url"
)

func (c *Client) TriggerBuild(ctx context.Context, projectID string) (*Build, error) {
	var build Build
	err := c.do(ctx, http.MethodPost, "/projects/"+url.PathEscape(projectID)+"/builds", nil, &build)
	return &build, err
}

func (c *Client) DeploymentAction(ctx context.Context, projectID, action, deploymentID string) (*GenericActionResult, error) {
	path := "/projects/" + url.PathEscape(projectID) + "/deployments/" + url.PathEscape(action)
	if action == "rollback" && deploymentID != "" {
		path = "/projects/" + url.PathEscape(projectID) + "/deployments/" + url.PathEscape(deploymentID) + "/rollback"
	}
	var result GenericActionResult
	err := c.do(ctx, http.MethodPost, path, nil, &result)
	return &result, err
}

func (c *Client) CreateComputeInstance(ctx context.Context, req CreateComputeInstanceRequest) (*ComputeInstance, *GenericActionResult, error) {
	var data struct {
		Instance  ComputeInstance `json:"instance"`
		Operation struct {
			ID string `json:"id"`
		} `json:"operation"`
		Message          string `json:"message"`
		PaymentURL       string `json:"payment_url"`
		PaymentReference string `json:"payment_reference"`
	}
	err := c.do(ctx, http.MethodPost, "/compute/instances", req, &data)
	result := &GenericActionResult{Message: data.Message, OperationID: data.Operation.ID, PaymentURL: data.PaymentURL, PaymentRef: data.PaymentReference}
	return &data.Instance, result, err
}

func (c *Client) GetComputeInstance(ctx context.Context, id string) (*ComputeInstance, error) {
	var data struct {
		Instance ComputeInstance `json:"instance"`
	}
	err := c.do(ctx, http.MethodGet, "/compute/instances/"+url.PathEscape(id), nil, &data)
	return &data.Instance, err
}

func (c *Client) ComputeAction(ctx context.Context, instanceID string, req ComputeActionRequest) (*GenericActionResult, error) {
	var result GenericActionResult
	err := c.do(ctx, http.MethodPost, "/compute/instances/"+url.PathEscape(instanceID)+"/actions", req, &result)
	return &result, err
}

func (c *Client) CreateAgencyClient(ctx context.Context, req AgencyClientRequest) (*AgencyClient, error) {
	var out AgencyClient
	err := c.do(ctx, http.MethodPost, "/agency/clients", req, &out)
	return &out, err
}

func (c *Client) GetAgencyClient(ctx context.Context, id string) (*AgencyClient, error) {
	var out AgencyClient
	err := c.do(ctx, http.MethodGet, "/agency/clients/"+url.PathEscape(id), nil, &out)
	return &out, err
}

func (c *Client) UpdateAgencyClient(ctx context.Context, id string, req AgencyClientRequest) (*AgencyClient, error) {
	var out AgencyClient
	err := c.do(ctx, http.MethodPatch, "/agency/clients/"+url.PathEscape(id), req, &out)
	return &out, err
}

func (c *Client) DeleteAgencyClient(ctx context.Context, id string) error {
	err := c.do(ctx, http.MethodDelete, "/agency/clients/"+url.PathEscape(id), nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func (c *Client) AssignAgencyResource(ctx context.Context, clientID string, req AgencyResourceRequest) (*AgencyResourceAssignment, error) {
	var out AgencyResourceAssignment
	err := c.do(ctx, http.MethodPost, "/agency/clients/"+url.PathEscape(clientID)+"/resources", req, &out)
	return &out, err
}

func (c *Client) DeleteAgencyResource(ctx context.Context, clientID, assignmentID string) error {
	err := c.do(ctx, http.MethodDelete, "/agency/clients/"+url.PathEscape(clientID)+"/resources/"+url.PathEscape(assignmentID), nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func (c *Client) CreateRunbook(ctx context.Context, projectID string, req RunbookRequest) (*Runbook, error) {
	var out Runbook
	err := c.do(ctx, http.MethodPost, "/projects/"+url.PathEscape(projectID)+"/terminal/runbooks", req, &out)
	return &out, err
}

func (c *Client) CreateRunbookVersion(ctx context.Context, projectID, runbookID string, req RunbookVersionRequest) (*GenericActionResult, error) {
	var data struct {
		ID string `json:"id"`
	}
	err := c.do(ctx, http.MethodPost, "/projects/"+url.PathEscape(projectID)+"/terminal/runbooks/"+url.PathEscape(runbookID)+"/versions", req, &data)
	return &GenericActionResult{OperationID: data.ID}, err
}

func (c *Client) ExecuteRunbook(ctx context.Context, projectID, versionID string, req RunbookExecutionRequest) (*GenericActionResult, error) {
	var data struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	path := "/projects/" + url.PathEscape(projectID) + "/terminal/runbook-versions/" + url.PathEscape(versionID) + "/execute"
	err := c.do(ctx, http.MethodPost, path, req, &data)
	return &GenericActionResult{ExecutionID: data.ID, Status: data.Status}, err
}

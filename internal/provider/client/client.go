package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ErrNotFound = errors.New("stackshift resource not found")

type APIError struct {
	StatusCode int
	Message    string
	Code       string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("stackshift api returned status %d", e.StatusCode)
}

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func New(baseURL, token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: httpClient,
	}
}

type envelope struct {
	Success    bool            `json:"success"`
	Status     string          `json:"status"`
	StatusCode int             `json:"status_code"`
	Message    string          `json:"message"`
	Data       json.RawMessage `json:"data"`
	Error      *struct {
		Code string `json:"code"`
	} `json:"error"`
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var reqBody io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+"/api/v1"+path, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	var env envelope
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &env)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		msg := env.Message
		if msg == "" {
			msg = strings.TrimSpace(string(raw))
		}
		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("%w: %s", ErrNotFound, msg)
		}
		code := ""
		if env.Error != nil {
			code = env.Error.Code
		}
		return &APIError{StatusCode: resp.StatusCode, Message: msg, Code: code}
	}
	if out == nil {
		return nil
	}
	if len(env.Data) > 0 && string(env.Data) != "null" {
		return json.Unmarshal(env.Data, out)
	}
	return json.Unmarshal(raw, out)
}

func (c *Client) CreateProject(ctx context.Context, req CreateProjectRequest) (*Project, error) {
	var project Project
	err := c.do(ctx, http.MethodPost, "/projects", req, &project)
	return &project, err
}

func (c *Client) GetProject(ctx context.Context, id string) (*Project, error) {
	var project Project
	err := c.do(ctx, http.MethodGet, "/projects/"+url.PathEscape(id), nil, &project)
	return &project, err
}

func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	var data struct {
		Projects []Project `json:"projects"`
	}
	err := c.do(ctx, http.MethodGet, "/projects", nil, &data)
	return data.Projects, err
}

func (c *Client) UpdateProject(ctx context.Context, id string, req UpdateProjectRequest) (*Project, error) {
	var project Project
	err := c.do(ctx, http.MethodPatch, "/projects/"+url.PathEscape(id), req, &project)
	return &project, err
}

func (c *Client) DeleteProject(ctx context.Context, id string) error {
	err := c.do(ctx, http.MethodDelete, "/projects/"+url.PathEscape(id), nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func (c *Client) GetProjectEnv(ctx context.Context, projectID, environment string) ([]EnvVar, error) {
	var env []EnvVar
	path := "/projects/" + url.PathEscape(projectID) + "/env?reveal=true&environment=" + url.QueryEscape(environment)
	err := c.do(ctx, http.MethodGet, path, nil, &env)
	return env, err
}

func (c *Client) SetProjectEnv(ctx context.Context, projectID, environment string, inputs []EnvVarInput) error {
	return c.do(ctx, http.MethodPut, "/projects/"+url.PathEscape(projectID)+"/env", SetEnvVarsRequest{Environment: environment, Variables: inputs}, nil)
}

func (c *Client) CreateDatabase(ctx context.Context, projectID string, req CreateDatabaseRequest) (*Database, error) {
	var data struct {
		Database Database `json:"database"`
	}
	err := c.do(ctx, http.MethodPost, "/projects/"+url.PathEscape(projectID)+"/databases", req, &data)
	return &data.Database, err
}

func (c *Client) GetDatabase(ctx context.Context, id string) (*Database, error) {
	var db Database
	err := c.do(ctx, http.MethodGet, "/databases/"+url.PathEscape(id), nil, &db)
	return &db, err
}

func (c *Client) DeleteDatabase(ctx context.Context, projectID, id string) error {
	err := c.do(ctx, http.MethodDelete, "/projects/"+url.PathEscape(projectID)+"/databases/"+url.PathEscape(id), nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func (c *Client) GetDatabaseCredentials(ctx context.Context, id string) (*DatabaseCredentials, error) {
	var creds DatabaseCredentials
	err := c.do(ctx, http.MethodGet, "/databases/"+url.PathEscape(id)+"/credentials", nil, &creds)
	return &creds, err
}

func (c *Client) CreateProjectDomain(ctx context.Context, projectID, domain string) (*ProjectDomain, error) {
	var data struct {
		Domain ProjectDomain `json:"domain"`
	}
	err := c.do(ctx, http.MethodPost, "/projects/"+url.PathEscape(projectID)+"/domains", CreateDomainRequest{Domain: domain}, &data)
	return &data.Domain, err
}

func (c *Client) ListProjectDomains(ctx context.Context, projectID string) ([]ProjectDomain, error) {
	var data struct {
		Domains []ProjectDomain `json:"domains"`
	}
	err := c.do(ctx, http.MethodGet, "/projects/"+url.PathEscape(projectID)+"/domains", nil, &data)
	return data.Domains, err
}

func (c *Client) DeleteProjectDomain(ctx context.Context, projectID, domainID string) error {
	err := c.do(ctx, http.MethodDelete, "/projects/"+url.PathEscape(projectID)+"/domains/"+url.PathEscape(domainID), nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func (c *Client) VerifyProjectDomain(ctx context.Context, projectID, domainID string) error {
	return c.do(ctx, http.MethodPost, "/projects/"+url.PathEscape(projectID)+"/domains/"+url.PathEscape(domainID)+"/verify", nil, nil)
}

func (c *Client) GetProjectDNSConfig(ctx context.Context, projectID string) (*DNSConfig, error) {
	var cfg DNSConfig
	err := c.do(ctx, http.MethodGet, "/projects/"+url.PathEscape(projectID)+"/domains/dns-config", nil, &cfg)
	return &cfg, err
}

func (c *Client) CreateDNSRecord(ctx context.Context, domainID string, req DNSRecordRequest) (*DNSRecord, error) {
	var record DNSRecord
	err := c.do(ctx, http.MethodPost, "/domains/"+url.PathEscape(domainID)+"/dns", req, &record)
	return &record, err
}

func (c *Client) ListDNSRecords(ctx context.Context, domainID string) ([]DNSRecord, error) {
	var data struct {
		Records []DNSRecord `json:"records"`
	}
	err := c.do(ctx, http.MethodGet, "/domains/"+url.PathEscape(domainID)+"/dns", nil, &data)
	return data.Records, err
}

func (c *Client) UpdateDNSRecord(ctx context.Context, domainID, recordID string, req DNSRecordRequest) (*DNSRecord, error) {
	var record DNSRecord
	err := c.do(ctx, http.MethodPut, "/domains/"+url.PathEscape(domainID)+"/dns/"+url.PathEscape(recordID), req, &record)
	return &record, err
}

func (c *Client) DeleteDNSRecord(ctx context.Context, domainID, recordID string) error {
	err := c.do(ctx, http.MethodDelete, "/domains/"+url.PathEscape(domainID)+"/dns/"+url.PathEscape(recordID), nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

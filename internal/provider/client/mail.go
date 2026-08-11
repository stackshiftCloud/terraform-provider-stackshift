package client

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"
)

type MailDomain struct {
	ID               string     `json:"id"`
	Domain           string     `json:"domain"`
	Status           string     `json:"status"`
	SPFStatus        string     `json:"spfStatus"`
	DKIMStatus       string     `json:"dkimStatus"`
	DMARCStatus      string     `json:"dmarcStatus"`
	ReturnPathStatus string     `json:"returnPathStatus"`
	ManagedDNS       bool       `json:"managedDns"`
	CreatedAt        time.Time  `json:"createdAt"`
	VerifiedAt       *time.Time `json:"verifiedAt"`
	LastCheckedAt    *time.Time `json:"lastCheckedAt"`
}

type MailWebhook struct {
	ID            string     `json:"id"`
	URL           string     `json:"url"`
	Description   *string    `json:"description"`
	Status        string     `json:"status"`
	EventTypes    []string   `json:"eventTypes"`
	FailureCount  int64      `json:"failureCount"`
	LastSuccessAt *time.Time `json:"lastSuccessAt"`
	LastFailureAt *time.Time `json:"lastFailureAt"`
	CreatedAt     time.Time  `json:"createdAt"`
	Secret        string     `json:"secret"`
}

func (c *Client) CreateMailDomain(
	ctx context.Context,
	domain, registeredDomainID string,
) (*MailDomain, error) {
	request := map[string]string{}
	if domain != "" {
		request["domain"] = domain
	}
	if registeredDomainID != "" {
		request["registeredDomainId"] = registeredDomainID
	}
	var result MailDomain
	err := c.do(ctx, http.MethodPost, "/mail/domains", request, &result)
	return &result, err
}

func (c *Client) GetMailDomain(ctx context.Context, id string) (*MailDomain, error) {
	var result MailDomain
	err := c.do(ctx, http.MethodGet, "/mail/domains/"+url.PathEscape(id), nil, &result)
	return &result, err
}

func (c *Client) VerifyMailDomain(ctx context.Context, id string) (*MailDomain, error) {
	var result MailDomain
	err := c.do(ctx, http.MethodPost, "/mail/domains/"+url.PathEscape(id)+"/verify", nil, &result)
	return &result, err
}

func (c *Client) DeleteMailDomain(ctx context.Context, id string) error {
	err := c.do(ctx, http.MethodDelete, "/mail/domains/"+url.PathEscape(id), nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func (c *Client) CreateMailWebhook(
	ctx context.Context,
	endpoint string,
	description *string,
	eventTypes []string,
) (*MailWebhook, error) {
	var result MailWebhook
	err := c.do(ctx, http.MethodPost, "/mail/webhooks", map[string]any{
		"url": endpoint, "description": description, "eventTypes": eventTypes,
	}, &result)
	return &result, err
}

func (c *Client) GetMailWebhook(ctx context.Context, id string) (*MailWebhook, error) {
	var result MailWebhook
	err := c.do(ctx, http.MethodGet, "/mail/webhooks/"+url.PathEscape(id), nil, &result)
	return &result, err
}

func (c *Client) UpdateMailWebhook(
	ctx context.Context,
	id, endpoint string,
	description *string,
	eventTypes []string,
	status string,
) (*MailWebhook, error) {
	var result MailWebhook
	err := c.do(ctx, http.MethodPatch, "/mail/webhooks/"+url.PathEscape(id), map[string]any{
		"url": endpoint, "description": description, "eventTypes": eventTypes, "status": status,
	}, &result)
	return &result, err
}

func (c *Client) DeleteMailWebhook(ctx context.Context, id string) error {
	err := c.do(ctx, http.MethodDelete, "/mail/webhooks/"+url.PathEscape(id), nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

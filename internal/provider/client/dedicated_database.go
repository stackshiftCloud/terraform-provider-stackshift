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
)

type DatabaseInstanceClassRevision struct {
	ID                       string `json:"id"`
	Version                  int64  `json:"version"`
	Currency                 string `json:"currency"`
	MonthlyPriceMinor        int64  `json:"monthly_price_minor"`
	StorageGBMonthPriceMinor int64  `json:"storage_gb_month_price_minor"`
	BackupGBMonthPriceMinor  int64  `json:"backup_gb_month_price_minor"`
	EffectiveAt              string `json:"effective_at"`
}

type DatabaseInstanceClass struct {
	Slug                 string                        `json:"slug"`
	VCPU                 int64                         `json:"vcpu"`
	MemoryMB             int64                         `json:"memory_mb"`
	CPUPolicy            string                        `json:"cpu_policy"`
	MaxConnections       int64                         `json:"max_connections"`
	PooledMaxConnections int64                         `json:"pooled_max_connections"`
	DiskIOPS             int64                         `json:"disk_iops"`
	DiskBytesPerSec      int64                         `json:"disk_bytes_per_sec"`
	NetworkKBps          int64                         `json:"network_kbps"`
	IsActive             bool                          `json:"is_active"`
	Revision             DatabaseInstanceClassRevision `json:"revision"`
}

type DedicatedMaintenanceWindow struct {
	DayOfWeek   int64 `json:"day_of_week"`
	StartMinute int64 `json:"start_minute"`
}

type CreateDedicatedDatabaseRequest struct {
	Tier                string                      `json:"tier"`
	Type                string                      `json:"type"`
	Name                string                      `json:"name"`
	Version             string                      `json:"version,omitempty"`
	InstanceClass       string                      `json:"instance_class"`
	StorageGB           int64                       `json:"storage_gb"`
	StorageMaxGB        int64                       `json:"storage_max_gb,omitempty"`
	MasterUsername      string                      `json:"master_username,omitempty"`
	DatabaseName        string                      `json:"database_name,omitempty"`
	BackupRetentionDays int64                       `json:"backup_retention_days,omitempty"`
	DeletionProtection  bool                        `json:"deletion_protection,omitempty"`
	MaintenanceWindow   *DedicatedMaintenanceWindow `json:"maintenance_window,omitempty"`
	Billing             string                      `json:"billing,omitempty"`
	Standby             string                      `json:"standby,omitempty"`
	OnStandbyLoss       string                      `json:"on_standby_loss,omitempty"`
}

type DatabaseEndpoint struct {
	Protocol string `json:"protocol"`
	Network  string `json:"network"`
	Role     string `json:"role"`
	Host     string `json:"host"`
	Port     int64  `json:"port"`
	TLSMode  string `json:"tls_mode"`
	Primary  bool   `json:"primary"`
}

type DedicatedSettings struct {
	InstanceClass       string                     `json:"instance_class"`
	StorageGB           int64                      `json:"storage_gb"`
	StorageMaxGB        int64                      `json:"storage_max_gb"`
	BackupRetentionDays int64                      `json:"backup_retention_days"`
	DeletionProtection  bool                       `json:"deletion_protection"`
	MaintenanceWindow   DedicatedMaintenanceWindow `json:"maintenance_window"`
	Billing             string                     `json:"billing"`
	Standby             string                     `json:"standby"`
	OnStandbyLoss       string                     `json:"on_standby_loss"`
}

type DedicatedDatabase struct {
	ID           string             `json:"id"`
	Tier         string             `json:"tier"`
	Dedicated    *DedicatedSettings `json:"dedicated"`
	ProjectID    *string            `json:"project_id,omitempty"`
	Name         string             `json:"name"`
	Type         string             `json:"type"`
	Version      string             `json:"version"`
	Host         string             `json:"host"`
	Port         int64              `json:"port"`
	TLSMode      string             `json:"tls_mode"`
	Username     string             `json:"username"`
	DatabaseName string             `json:"database_name"`
	Status       string             `json:"status"`
	ErrorMessage *string            `json:"error_message,omitempty"`
	Endpoints    []DatabaseEndpoint `json:"endpoints"`
}

type DatabaseCheckout struct {
	Status      string `json:"status"`
	Message     string `json:"message"`
	PaymentURL  string `json:"payment_url"`
	Reference   string `json:"payment_reference"`
	AmountMinor int64  `json:"amount_minor"`
	WalletMinor int64  `json:"wallet_minor"`
	Currency    string `json:"currency"`
}

type DedicatedCreateResult struct {
	Database       *DedicatedDatabase `json:"database"`
	MasterPassword string             `json:"master_password"`
	Checkout       *DatabaseCheckout  `json:"checkout"`
	CheckoutURL    string             `json:"checkout_url"`
}

type DatabaseChangeQuote struct {
	CurrentMonthlyMinor int64  `json:"current_monthly_minor"`
	NewMonthlyMinor     int64  `json:"new_monthly_minor"`
	ChargeMinor         int64  `json:"charge_minor"`
	CreditMinor         int64  `json:"credit_minor"`
	Currency            string `json:"currency"`
	Billing             string `json:"billing"`
}

type DedicatedHAMember struct {
	ID            string `json:"id"`
	Role          string `json:"role"`
	InstanceClass string `json:"instance_class"`
	Status        string `json:"status"`
	Site          string `json:"site"`
}

type DedicatedSizeChange struct {
	ID             string `json:"id"`
	FromClass      string `json:"from_class"`
	ToClass        string `json:"to_class"`
	ApplyAfter     string `json:"apply_after"`
	Status         string `json:"status"`
	FailureMessage string `json:"failure_message"`
}

type DedicatedHA struct {
	StandbyMode      string               `json:"standby_mode"`
	OnStandbyLoss    string               `json:"on_standby_loss"`
	StandbyAvailable bool                 `json:"standby_available"`
	Members          []DedicatedHAMember  `json:"members"`
	SizeChange       *DedicatedSizeChange `json:"size_change"`
}

type DedicatedBilling struct {
	Mode              string `json:"mode"`
	Status            string `json:"status"`
	Currency          string `json:"currency"`
	MonthlyPriceMinor int64  `json:"monthly_price_minor"`
	CurrentPeriodEnd  string `json:"current_period_end"`
}

type DedicatedReplica struct {
	ID            string `json:"replica_id"`
	InstanceClass string `json:"instance_class"`
}

type DedicatedError struct {
	StatusCode  int
	Code        string
	Message     string
	CheckoutURL string
	AmountMinor int64
	Currency    string
	Quote       *DatabaseChangeQuote
}

func (e *DedicatedError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("stackshift api returned status %d", e.StatusCode)
}

func (e *DedicatedError) PriceConfirmationRequired() bool {
	return e.StatusCode == http.StatusConflict && e.Code == "price_confirmation_required" && e.Quote != nil
}

func (e *DedicatedError) PaymentRequired() bool {
	return e.StatusCode == http.StatusPaymentRequired
}

func AsDedicatedError(err error) (*DedicatedError, bool) {
	var dedicated *DedicatedError
	ok := errors.As(err, &dedicated)
	return dedicated, ok
}

type dedicatedBody struct {
	Message     string               `json:"message"`
	Error       json.RawMessage      `json:"error"`
	CheckoutURL string               `json:"checkout_url"`
	AmountMinor int64                `json:"amount_minor"`
	Currency    string               `json:"currency"`
	Quote       *DatabaseChangeQuote `json:"quote"`
	Data        json.RawMessage      `json:"data"`
}

func (b dedicatedBody) code() string {
	var code string
	if json.Unmarshal(b.Error, &code) == nil {
		return code
	}
	var detail struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(b.Error, &detail)
	return detail.Code
}

func (c *Client) dedicatedDo(ctx context.Context, method, path string, body any, out any) error {
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
	var parsed dedicatedBody
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &parsed)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		message := parsed.Message
		if message == "" {
			message = strings.TrimSpace(string(raw))
		}
		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("%w: %s", ErrNotFound, message)
		}
		if resp.StatusCode == http.StatusPaymentRequired && out != nil {
			_ = json.Unmarshal(raw, out)
		}
		return &DedicatedError{StatusCode: resp.StatusCode, Code: parsed.code(), Message: message,
			CheckoutURL: parsed.CheckoutURL, AmountMinor: parsed.AmountMinor, Currency: parsed.Currency, Quote: parsed.Quote}
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	if len(parsed.Data) > 0 && string(parsed.Data) != "null" {
		return json.Unmarshal(parsed.Data, out)
	}
	return json.Unmarshal(raw, out)
}

func dedicatedPath(id string, suffix string) string {
	return "/databases/" + url.PathEscape(id) + suffix
}

func (c *Client) ListDatabaseInstanceClasses(ctx context.Context) ([]DatabaseInstanceClass, error) {
	var result struct {
		Classes []DatabaseInstanceClass `json:"classes"`
	}
	err := c.dedicatedDo(ctx, http.MethodGet, "/databases/instance-classes", nil, &result)
	return result.Classes, err
}

func (c *Client) CreateDedicatedDatabase(ctx context.Context, projectID string, req CreateDedicatedDatabaseRequest) (*DedicatedCreateResult, error) {
	path := "/databases"
	if projectID != "" {
		path = "/projects/" + url.PathEscape(projectID) + "/databases"
	}
	req.Tier = "dedicated"
	req.Type = "postgres"
	var result DedicatedCreateResult
	err := c.dedicatedDo(ctx, http.MethodPost, path, req, &result)
	return &result, err
}

func (c *Client) GetDedicatedDatabase(ctx context.Context, id string) (*DedicatedDatabase, error) {
	var db DedicatedDatabase
	err := c.dedicatedDo(ctx, http.MethodGet, dedicatedPath(id, ""), nil, &db)
	return &db, err
}

func (c *Client) GetDedicatedHA(ctx context.Context, id string) (*DedicatedHA, error) {
	var ha DedicatedHA
	err := c.dedicatedDo(ctx, http.MethodGet, dedicatedPath(id, "/ha"), nil, &ha)
	return &ha, err
}

func (c *Client) GetDedicatedBilling(ctx context.Context, id string) (*DedicatedBilling, error) {
	var billing DedicatedBilling
	err := c.dedicatedDo(ctx, http.MethodGet, dedicatedPath(id, "/billing"), nil, &billing)
	return &billing, err
}

func (c *Client) UpdateDedicatedSettings(ctx context.Context, id string, changes map[string]any) (*DedicatedDatabase, error) {
	var db DedicatedDatabase
	err := c.dedicatedDo(ctx, http.MethodPatch, dedicatedPath(id, ""), changes, &db)
	return &db, err
}

func (c *Client) ChangeDedicatedClass(ctx context.Context, id, instanceClass, apply string, confirmChargeMinor *int64) error {
	return c.dedicatedDo(ctx, http.MethodPost, dedicatedPath(id, "/class"), map[string]any{
		"instance_class": instanceClass, "apply": apply, "confirm_charge_minor": confirmChargeMinor,
	}, nil)
}

func (c *Client) UpdateDedicatedStorage(ctx context.Context, id string, storageGB, storageMaxGB int64, confirmChargeMinor *int64) error {
	body := map[string]any{"confirm_charge_minor": confirmChargeMinor}
	if storageGB > 0 {
		body["storage_gb"] = storageGB
	}
	if storageMaxGB > 0 {
		body["storage_max_gb"] = storageMaxGB
	}
	return c.dedicatedDo(ctx, http.MethodPatch, dedicatedPath(id, "/storage"), body, nil)
}

func (c *Client) SetDedicatedStandby(ctx context.Context, id, mode, onStandbyLoss string, confirmChargeMinor *int64) error {
	return c.dedicatedDo(ctx, http.MethodPut, dedicatedPath(id, "/standby"), map[string]any{
		"mode": mode, "on_standby_loss": onStandbyLoss, "confirm_charge_minor": confirmChargeMinor,
	}, nil)
}

func (c *Client) AddDedicatedReplica(ctx context.Context, id, instanceClass string, confirmChargeMinor *int64) (*DedicatedReplica, error) {
	body := map[string]any{"confirm_charge_minor": confirmChargeMinor}
	if instanceClass != "" {
		body["instance_class"] = instanceClass
	}
	var replica DedicatedReplica
	err := c.dedicatedDo(ctx, http.MethodPost, dedicatedPath(id, "/replicas"), body, &replica)
	return &replica, err
}

func (c *Client) RemoveDedicatedReplica(ctx context.Context, id, replicaID string) error {
	err := c.dedicatedDo(ctx, http.MethodDelete, dedicatedPath(id, "/replicas/"+url.PathEscape(replicaID)), nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func (c *Client) DeleteDedicatedDatabase(ctx context.Context, id string, skipFinalSnapshot bool) error {
	path := dedicatedPath(id, "")
	if skipFinalSnapshot {
		path += "?skip_final_snapshot=true"
	}
	err := c.dedicatedDo(ctx, http.MethodDelete, path, nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

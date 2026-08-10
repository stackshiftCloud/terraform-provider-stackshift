package client

import "time"

// BYOCProviderConnectionRequest is deliberately a tagged union at the API
// boundary. Callers must populate exactly one provider-specific credential
// object; AWS access keys and Azure client secrets have no representation.
type BYOCProviderConnectionRequest struct {
	Provider    string                           `json:"provider"`
	DisplayName string                           `json:"display_name"`
	Credentials BYOCProviderConnectionCredential `json:"credentials"`
}

type BYOCProviderConnectionCredential struct {
	Token               string `json:"token,omitempty"`
	RoleARN             string `json:"role_arn,omitempty"`
	Region              string `json:"region,omitempty"`
	AzureTenantID       string `json:"azure_tenant_id,omitempty"`
	AzureSubscriptionID string `json:"azure_subscription_id,omitempty"`
	AzureClientID       string `json:"azure_client_id,omitempty"`
}

type BYOCProviderConnection struct {
	ID                string         `json:"id"`
	Provider          string         `json:"provider"`
	DisplayName       string         `json:"display_name"`
	AuthType          string         `json:"auth_type"`
	AuthConfig        map[string]any `json:"auth_config,omitempty"`
	ExternalID        string         `json:"external_id,omitempty"`
	Status            string         `json:"status"`
	ValidationError   string         `json:"validation_error,omitempty"`
	ValidationDetails struct {
		Code               string   `json:"code,omitempty"`
		Retryable          bool     `json:"retryable"`
		MissingPermissions []string `json:"missing_permissions,omitempty"`
	} `json:"validation_details,omitempty"`
}

type BYOCOperation struct {
	ID                string    `json:"id"`
	NodeID            string    `json:"node_id"`
	OperationType     string    `json:"operation_type"`
	Status            string    `json:"status"`
	Step              string    `json:"step"`
	Revision          int64     `json:"revision"`
	Attempts          int       `json:"attempts"`
	Progress          int       `json:"progress"`
	CleanupState      string    `json:"cleanup_state"`
	Retryable         bool      `json:"retryable"`
	ErrorCode         string    `json:"error_code,omitempty"`
	ErrorSummary      string    `json:"error_summary,omitempty"`
	ProviderRequestID string    `json:"provider_request_id,omitempty"`
	NextAttemptAt     time.Time `json:"next_attempt_at"`
}

type BYOCDeletionBlocker struct {
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id,omitempty"`
	Name         string `json:"name,omitempty"`
	State        string `json:"state,omitempty"`
	Count        int    `json:"count,omitempty"`
}

type BYOCNode struct {
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	Region               string    `json:"region"`
	Status               string    `json:"status"`
	NodeSource           string    `json:"node_source"`
	ProviderConnectionID string    `json:"provider_connection_id"`
	Provider             string    `json:"provider"`
	ExternalResourceID   string    `json:"external_resource_id"`
	TierSlug             string    `json:"tier_slug"`
	BootstrapStatus      string    `json:"bootstrap_status"`
	EnrollmentState      string    `json:"enrollment_state"`
	OverlayIP            string    `json:"overlay_ip"`
	PublicIP             string    `json:"public_ip"`
	FailureReason        string    `json:"failure_reason"`
	LastHeartbeatAt      time.Time `json:"last_heartbeat_at"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type BYOCResourceOperation struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Step         string `json:"step"`
	Progress     int64  `json:"progress"`
	Retryable    bool   `json:"retryable"`
	ErrorCode    string `json:"error_code"`
	ErrorSummary string `json:"error_summary"`
}

type BYOCVolume struct {
	ID               string                 `json:"id"`
	NodeID           string                 `json:"node_id"`
	Provider         string                 `json:"provider"`
	Name             string                 `json:"name"`
	ProviderVolumeID string                 `json:"provider_volume_id"`
	SizeGB           int64                  `json:"size_gb"`
	Region           string                 `json:"region"`
	MountPath        string                 `json:"mount_path"`
	Filesystem       string                 `json:"filesystem"`
	State            string                 `json:"state"`
	FailureReason    string                 `json:"failure_reason"`
	SnapshotCount    int64                  `json:"snapshot_count"`
	Operation        *BYOCResourceOperation `json:"operation,omitempty"`
}

type BYOCSnapshot struct {
	ID                 string                 `json:"id"`
	NodeID             string                 `json:"node_id"`
	VolumeID           string                 `json:"volume_id"`
	Provider           string                 `json:"provider"`
	Name               string                 `json:"name"`
	ProviderSnapshotID string                 `json:"provider_snapshot_id"`
	SizeGB             int64                  `json:"size_gb"`
	State              string                 `json:"state"`
	FailureReason      string                 `json:"failure_reason"`
	Operation          *BYOCResourceOperation `json:"operation,omitempty"`
}

type BYOCStaticIP struct {
	Supported          bool      `json:"supported"`
	ProviderResourceID string    `json:"provider_resource_id"`
	IPAddress          string    `json:"ip_address"`
	Status             string    `json:"status"`
	Assigned           bool      `json:"assigned"`
	LastSyncedAt       time.Time `json:"last_synced_at"`
}

type BYOCNodeResourceSummary struct {
	Volumes   []BYOCVolume   `json:"volumes"`
	Snapshots []BYOCSnapshot `json:"snapshots"`
	StaticIP  *BYOCStaticIP  `json:"static_ip"`
}

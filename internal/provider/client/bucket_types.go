package client

import "time"

type Bucket struct {
	ID                   string    `json:"id"`
	WorkspaceID          *string   `json:"workspace_id,omitempty"`
	ProjectID            *string   `json:"project_id,omitempty"`
	Name                 string    `json:"name"`
	Region               string    `json:"region"`
	Visibility           string    `json:"visibility"`
	VersioningEnabled    bool      `json:"versioning_enabled"`
	QuotaBytes           *int64    `json:"quota_bytes,omitempty"`
	DefaultRetentionDays int64     `json:"default_retention_days"`
	TierAfterDays        int64     `json:"tier_after_days"`
	WebsiteEnabled       bool      `json:"website_enabled"`
	WebsiteIndex         string    `json:"website_index"`
	WebsiteError         string    `json:"website_error,omitempty"`
	EncryptionMode       string    `json:"encryption_mode"`
	KMSKeyID             string    `json:"kms_key_id,omitempty"`
	CustomDomainID       *string   `json:"custom_domain_id,omitempty"`
	ObjectCount          int64     `json:"object_count"`
	SizeBytes            int64     `json:"size_bytes"`
	Endpoint             string    `json:"endpoint,omitempty"`
	CreatedAt            time.Time `json:"created_at,omitempty"`
	UpdatedAt            time.Time `json:"updated_at,omitempty"`
}

type BucketCredentials struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region"`
}

type CreateBucketRequest struct {
	Name       string  `json:"name"`
	Region     string  `json:"region"`
	Visibility string  `json:"visibility"`
	ProjectID  *string `json:"project_id,omitempty"`
	Label      string  `json:"label,omitempty"`
}

type UpdateBucketSettingsRequest struct {
	VersioningEnabled    bool    `json:"versioning_enabled"`
	QuotaBytes           *int64  `json:"quota_bytes"`
	DefaultRetentionDays int64   `json:"default_retention_days"`
	TierAfterDays        int64   `json:"tier_after_days"`
	WebsiteEnabled       bool    `json:"website_enabled"`
	WebsiteIndex         string  `json:"website_index"`
	WebsiteError         string  `json:"website_error"`
	EncryptionMode       string  `json:"encryption_mode"`
	CustomDomainID       *string `json:"custom_domain_id"`
}

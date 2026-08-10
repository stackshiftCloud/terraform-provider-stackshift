package client

import (
	"encoding/json"
	"time"
)

type GenericActionResult struct {
	Message      string          `json:"message,omitempty"`
	BuildID      string          `json:"build_id,omitempty"`
	Status       string          `json:"status,omitempty"`
	OperationID  string          `json:"operation_id,omitempty"`
	DeploymentID string          `json:"deployment_id,omitempty"`
	ExecutionID  string          `json:"execution_id,omitempty"`
	PaymentURL   string          `json:"payment_url,omitempty"`
	PaymentRef   string          `json:"payment_reference,omitempty"`
	Raw          json.RawMessage `json:"-"`
}

type AssetBucket struct {
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	DefaultVisibility string          `json:"default_visibility"`
	CacheControl      *string         `json:"cache_control,omitempty"`
	Versioning        bool            `json:"versioning"`
	RetentionDays     int64           `json:"retention_days"`
	MaxObjectBytes    *int64          `json:"max_object_bytes,omitempty"`
	AllowedMimeTypes  []string        `json:"allowed_mime_types"`
	HomeRegion        string          `json:"home_region"`
	ReplicationPolicy string          `json:"replication_policy"`
	CORSOrigins       []string        `json:"cors_origins"`
	AllowedOrigins    []string        `json:"allowed_origins"`
	LifecyclePolicy   json.RawMessage `json:"lifecycle_policy"`
	CustomDomainID    *string         `json:"custom_domain_id,omitempty"`
	Revision          int64           `json:"revision"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type AssetWebhook struct {
	ID            string     `json:"id"`
	URL           string     `json:"url"`
	EventTypes    []string   `json:"event_types"`
	Status        string     `json:"status"`
	FailureCount  int64      `json:"failure_count"`
	LastSuccessAt *time.Time `json:"last_success_at,omitempty"`
	LastFailureAt *time.Time `json:"last_failure_at,omitempty"`
	DisabledAt    *time.Time `json:"disabled_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type AssetLifecycleRule struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Prefix    string     `json:"prefix"`
	Action    string     `json:"action"`
	AgeDays   int64      `json:"age_days"`
	Enabled   bool       `json:"enabled"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	LastRunAt *time.Time `json:"last_run_at,omitempty"`
	LastError *string    `json:"last_error,omitempty"`
}

type AssetCustomDomain struct {
	ID                string     `json:"id"`
	Domain            string     `json:"domain"`
	Status            string     `json:"status"`
	VerificationName  string     `json:"verification_name"`
	VerificationValue string     `json:"verification_value"`
	LastError         *string    `json:"last_error,omitempty"`
	ProviderDomainID  *string    `json:"provider_domain_id,omitempty"`
	CertificateID     *string    `json:"certificate_id,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	VerifiedAt        *time.Time `json:"verified_at,omitempty"`
}

type AssetTransformation struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Spec           string    `json:"spec"`
	NormalizedSpec string    `json:"normalized_spec"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type AssetContentPolicy struct {
	AllowedMimeTypes []string  `json:"allowed_mime_types"`
	MaxImageBytes    int64     `json:"max_image_bytes"`
	MaxVideoBytes    int64     `json:"max_video_bytes"`
	MaxOtherBytes    int64     `json:"max_other_bytes"`
	RequireScan      bool      `json:"require_scan"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type AssetUsageSummary struct {
	TotalAssets             int64            `json:"total_assets"`
	TotalBytes              int64            `json:"total_bytes"`
	PeriodIngressBytes      int64            `json:"period_ingress_bytes"`
	PeriodEgressBytes       int64            `json:"period_egress_bytes"`
	PeriodOriginBytes       int64            `json:"period_origin_bytes"`
	PeriodVerificationBytes int64            `json:"period_verification_bytes"`
	PeriodTransformCount    int64            `json:"period_transform_count"`
	PeriodAIRequests        int64            `json:"period_ai_requests"`
	PeriodLogicalByteHours  int64            `json:"period_logical_storage_byte_hours"`
	PeriodPhysicalByteHours int64            `json:"period_physical_storage_byte_hours"`
	PeriodDerivedByteHours  int64            `json:"period_derived_storage_byte_hours"`
	PeriodTransformMillis   int64            `json:"period_transform_compute_ms"`
	PeriodVideoInputSeconds int64            `json:"period_video_input_seconds"`
	PeriodAICostMicros      int64            `json:"period_ai_cost_micros"`
	ByBucket                map[string]int64 `json:"by_bucket"`
	ByMimeFamily            map[string]int64 `json:"by_mime_family"`
	ByVisibility            map[string]int64 `json:"by_visibility"`
}

type Asset struct {
	ID                string `json:"id"`
	Bucket            string `json:"bucket"`
	Key               string `json:"key"`
	OriginalName      string `json:"original_name"`
	MimeType          string `json:"mime_type"`
	Size              int64  `json:"size"`
	ChecksumSHA256    string `json:"checksum_sha256"`
	Visibility        string `json:"visibility"`
	Status            string `json:"status"`
	ReplicationStatus string `json:"replication_status"`
	Generation        int64  `json:"generation"`
	Revision          int64  `json:"revision"`
	URL               string `json:"url"`
}

type AssetJob struct {
	ID              string          `json:"id"`
	JobName         string          `json:"job_name"`
	Status          string          `json:"status"`
	Result          json.RawMessage `json:"result"`
	Error           json.RawMessage `json:"error"`
	Attempts        int64           `json:"attempts"`
	MaxAttempts     int64           `json:"max_attempts"`
	ProgressPercent *int64          `json:"progress_percent,omitempty"`
	Phase           string          `json:"phase,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type Build struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

type Project struct {
	ID                   string          `json:"id"`
	UserID               string          `json:"user_id,omitempty"`
	TeamID               *string         `json:"team_id,omitempty"`
	Name                 string          `json:"name"`
	Slug                 string          `json:"slug,omitempty"`
	Description          *string         `json:"description,omitempty"`
	Runtime              string          `json:"runtime,omitempty"`
	Region               string          `json:"region,omitempty"`
	Port                 int             `json:"port,omitempty"`
	Status               string          `json:"status,omitempty"`
	DeploymentMode       string          `json:"deployment_mode,omitempty"`
	GitHubRepoURL        *string         `json:"github_repo_url,omitempty"`
	GitHubRepoID         int64           `json:"github_repo_id,omitempty"`
	GitHubInstallationID int64           `json:"github_installation_id,omitempty"`
	GitHubBranch         string          `json:"github_branch,omitempty"`
	BuildCommand         *string         `json:"build_command,omitempty"`
	StartCommand         *string         `json:"start_command,omitempty"`
	InstallCommand       *string         `json:"install_command,omitempty"`
	RootDirectory        *string         `json:"root_directory,omitempty"`
	OutputDirectory      *string         `json:"output_directory,omitempty"`
	SourceType           string          `json:"source_type,omitempty"`
	DockerImageURI       *string         `json:"docker_image_uri,omitempty"`
	TargetNodeID         *string         `json:"target_node_id,omitempty"`
	CreatedAt            time.Time       `json:"created_at,omitempty"`
	UpdatedAt            time.Time       `json:"updated_at,omitempty"`
	Raw                  json.RawMessage `json:"-"`
}

type CreateProjectRequest struct {
	Name                 string  `json:"name"`
	TeamID               *string `json:"team_id,omitempty"`
	Description          *string `json:"description,omitempty"`
	GitHubRepoURL        *string `json:"github_repo_url,omitempty"`
	GitHubRepoID         int64   `json:"github_repo_id,omitempty"`
	GitHubInstallationID int64   `json:"github_installation_id,omitempty"`
	GitHubBranch         string  `json:"github_branch,omitempty"`
	Runtime              string  `json:"runtime"`
	BuildCommand         *string `json:"build_command,omitempty"`
	StartCommand         *string `json:"start_command,omitempty"`
	RootDirectory        *string `json:"root_directory,omitempty"`
	OutputDirectory      *string `json:"output_directory,omitempty"`
	InstallCommand       *string `json:"install_command,omitempty"`
	Port                 int     `json:"port"`
	Region               string  `json:"region"`
	SourceType           string  `json:"source_type,omitempty"`
	DockerImageURI       *string `json:"docker_image_uri,omitempty"`
	TargetNodeID         *string `json:"target_node_id,omitempty"`
}

type UpdateProjectRequest struct {
	Name                 *string `json:"name,omitempty"`
	TeamID               *string `json:"team_id,omitempty"`
	Description          *string `json:"description,omitempty"`
	GitHubRepoURL        *string `json:"github_repo_url,omitempty"`
	GitHubRepoID         *int64  `json:"github_repo_id,omitempty"`
	GitHubInstallationID *int64  `json:"github_installation_id,omitempty"`
	GitHubBranch         *string `json:"github_branch,omitempty"`
	Runtime              *string `json:"runtime,omitempty"`
	BuildCommand         *string `json:"build_command,omitempty"`
	StartCommand         *string `json:"start_command,omitempty"`
	RootDirectory        *string `json:"root_directory,omitempty"`
	OutputDirectory      *string `json:"output_directory,omitempty"`
	InstallCommand       *string `json:"install_command,omitempty"`
	Port                 *int    `json:"port,omitempty"`
	Region               *string `json:"region,omitempty"`
	SourceType           *string `json:"source_type,omitempty"`
	DockerImageURI       *string `json:"docker_image_uri,omitempty"`
	TargetNodeID         *string `json:"target_node_id,omitempty"`
}

type EnvVar struct {
	Key             string `json:"key"`
	Value           string `json:"value,omitempty"`
	IsSecret        bool   `json:"is_secret"`
	Source          string `json:"source,omitempty"`
	NeedsRotation   bool   `json:"needs_rotation,omitempty"`
	EnvironmentName string `json:"environment_name,omitempty"`
	HasValue        bool   `json:"has_value"`
}

type SetEnvVarsRequest struct {
	Environment string        `json:"environment,omitempty"`
	Variables   []EnvVarInput `json:"variables"`
}

type EnvVarInput struct {
	Key                  string  `json:"key"`
	Value                *string `json:"value,omitempty"`
	IsSecret             bool    `json:"is_secret"`
	RotationReminderDays int     `json:"rotation_reminder_days,omitempty"`
}

type Database struct {
	ID             string    `json:"id"`
	ProjectID      *string   `json:"project_id,omitempty"`
	UserID         string    `json:"user_id,omitempty"`
	TargetNodeID   *string   `json:"target_node_id,omitempty"`
	Name           string    `json:"name"`
	Type           string    `json:"type"`
	Version        string    `json:"version"`
	Host           string    `json:"host,omitempty"`
	Port           int       `json:"port,omitempty"`
	TLSMode        string    `json:"tls_mode,omitempty"`
	DatabaseName   string    `json:"database_name,omitempty"`
	SizeGB         int       `json:"size_gb"`
	Status         string    `json:"status"`
	RuntimeKind    string    `json:"runtime_kind,omitempty"`
	MigrationState string    `json:"migration_status,omitempty"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
}

type CreateDatabaseRequest struct {
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Version      string  `json:"version"`
	SizeGB       int     `json:"size_gb"`
	TargetNodeID *string `json:"target_node_id,omitempty"`
}

type DatabaseCredentials struct {
	Username         string `json:"username"`
	Password         string `json:"password"`
	DatabaseName     string `json:"database_name"`
	Host             string `json:"host"`
	Port             int    `json:"port"`
	TLSMode          string `json:"tls_mode"`
	ConnectionString string `json:"connection_string"`
}

type ProjectDomain struct {
	ID                string    `json:"id"`
	ProjectID         string    `json:"project_id,omitempty"`
	Domain            string    `json:"domain"`
	VerificationToken *string   `json:"verification_token,omitempty"`
	Verified          bool      `json:"verified"`
	SSLStatus         string    `json:"ssl_status,omitempty"`
	IsPrimary         bool      `json:"is_primary,omitempty"`
	CreatedAt         time.Time `json:"created_at,omitempty"`
	UpdatedAt         time.Time `json:"updated_at,omitempty"`
}

type CreateDomainRequest struct {
	Domain string `json:"domain"`
}

type DNSConfig struct {
	CNAMETarget    string      `json:"cname_target"`
	LoadBalancerIP string      `json:"load_balancer_ip"`
	BaseDomain     string      `json:"base_domain"`
	ProjectURL     string      `json:"project_url"`
	Records        []DNSAdvice `json:"records"`
}

type DNSAdvice struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

type DNSRecord struct {
	ID         string    `json:"id"`
	DomainID   string    `json:"domain_id"`
	RecordType string    `json:"record_type"`
	Name       string    `json:"name"`
	Value      string    `json:"value"`
	Priority   *int      `json:"priority,omitempty"`
	TTL        int       `json:"ttl"`
	ManagedBy  string    `json:"managed_by,omitempty"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

type DNSRecordRequest struct {
	RecordType string `json:"record_type"`
	Name       string `json:"name"`
	Value      string `json:"value"`
	Priority   *int   `json:"priority,omitempty"`
	TTL        int    `json:"ttl,omitempty"`
}

type ComputeInstance struct {
	ID                     string    `json:"id"`
	PlanID                 string    `json:"plan_id"`
	Name                   string    `json:"name"`
	Hostname               string    `json:"hostname"`
	Status                 string    `json:"status"`
	Mode                   string    `json:"mode"`
	ImageSlug              string    `json:"image_slug"`
	IPv4Address            *string   `json:"ipv4_address,omitempty"`
	IPv6Address            *string   `json:"ipv6_address,omitempty"`
	InstallDocker          bool      `json:"install_docker"`
	InstallStackShiftAgent bool      `json:"install_stackshift_agent"`
	CreatedAt              time.Time `json:"created_at,omitempty"`
}

type CreateComputeInstanceRequest struct {
	PlanID                 *string `json:"plan_id,omitempty"`
	PlanSlug               string  `json:"plan_slug,omitempty"`
	Name                   string  `json:"name"`
	Hostname               string  `json:"hostname"`
	ImageSlug              string  `json:"image_slug"`
	SSHPublicKey           string  `json:"ssh_public_key"`
	Mode                   string  `json:"mode"`
	InstallDocker          bool    `json:"install_docker"`
	InstallStackShiftAgent bool    `json:"install_stackshift_agent"`
	IdempotencyKey         string  `json:"idempotency_key,omitempty"`
	PaymentProvider        string  `json:"payment_provider,omitempty"`
}

type ComputeActionRequest struct {
	Action       string `json:"action"`
	Confirmation string `json:"confirmation,omitempty"`
	ImageSlug    string `json:"image_slug,omitempty"`
	SSHPublicKey string `json:"ssh_public_key,omitempty"`
	Hostname     string `json:"hostname,omitempty"`
}

type AgencyClient struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Company         *string   `json:"company,omitempty"`
	Email           *string   `json:"email,omitempty"`
	Phone           *string   `json:"phone,omitempty"`
	Status          string    `json:"status"`
	BillingCurrency string    `json:"billing_currency"`
	Notes           *string   `json:"notes,omitempty"`
	CreatedAt       time.Time `json:"created_at,omitempty"`
}

type AgencyClientRequest struct {
	Name            string  `json:"name"`
	Company         *string `json:"company,omitempty"`
	Email           *string `json:"email,omitempty"`
	Phone           *string `json:"phone,omitempty"`
	Status          string  `json:"status,omitempty"`
	BillingCurrency string  `json:"billing_currency,omitempty"`
	Notes           *string `json:"notes,omitempty"`
	TeamID          *string `json:"team_id,omitempty"`
}

type AgencyResourceAssignment struct {
	ID           string    `json:"id"`
	ClientID     string    `json:"client_id"`
	ResourceType string    `json:"resource_type"`
	ResourceID   string    `json:"resource_id"`
	Label        *string   `json:"label,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
}

type AgencyResourceRequest struct {
	ResourceType string  `json:"resource_type"`
	ResourceID   string  `json:"resource_id"`
	Label        *string `json:"label,omitempty"`
	TeamID       *string `json:"team_id,omitempty"`
}

type Runbook struct {
	ID          string    `json:"id"`
	ProjectID   *string   `json:"project_id,omitempty"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Framework   *string   `json:"framework,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
}

type RunbookRequest struct {
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Framework   *string `json:"framework,omitempty"`
}

type RunbookVersionRequest struct {
	Command             string          `json:"command"`
	Parameters          json.RawMessage `json:"parameters"`
	Environments        []string        `json:"environments"`
	RequiredPermissions []string        `json:"required_permissions"`
	ApprovalRequired    bool            `json:"approval_required"`
	Published           bool            `json:"published"`
}

type RunbookExecutionRequest struct {
	Parameters        map[string]string `json:"parameters"`
	ApprovalRequestID *string           `json:"approval_request_id,omitempty"`
	Reason            string            `json:"reason,omitempty"`
}

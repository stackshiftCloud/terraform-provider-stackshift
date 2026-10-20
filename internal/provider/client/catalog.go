package client

import (
	"context"
	"net/http"
)

type RegionProduct struct {
	Available bool     `json:"available"`
	Tiers     []string `json:"tiers"`
}

type Region struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	City        string `json:"city"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	Default     bool   `json:"default"`
	Status      string `json:"status"`
	Products    struct {
		Apps      RegionProduct `json:"apps"`
		Databases RegionProduct `json:"databases"`
		VPS       RegionProduct `json:"vps"`
	} `json:"products"`
}

type RegionCatalog struct {
	DefaultRegion string   `json:"default_region"`
	Regions       []Region `json:"regions"`
}

type AppSize struct {
	Slug            string   `json:"slug"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	MemoryMB        int64    `json:"memory_mb"`
	CPUMillicores   int64    `json:"cpu_millicores"`
	Listed          bool     `json:"listed"`
	Recommended     bool     `json:"recommended"`
	PriceNGNMonthly *int64   `json:"price_ngn_monthly"`
	IncludedInPlan  bool     `json:"included_in_plan"`
	MinPlan         string   `json:"min_plan"`
	Plans           []string `json:"plans"`
}

type AppSizePlan struct {
	Slug             string `json:"slug"`
	Name             string `json:"name"`
	MaxMemoryMB      int64  `json:"max_memory_mb"`
	MaxCPUMillicores int64  `json:"max_cpu_millicores"`
	AppTier          string `json:"app_tier"`
	DefaultSize      string `json:"default_size"`
}

type AppSizeCatalog struct {
	Version  int64         `json:"version"`
	Currency string        `json:"currency"`
	Sizes    []AppSize     `json:"sizes"`
	Plans    []AppSizePlan `json:"plans"`
}

func (c *Client) GetRegions(ctx context.Context) (*RegionCatalog, error) {
	var catalog RegionCatalog
	err := c.do(ctx, http.MethodGet, "/regions", nil, &catalog)
	return &catalog, err
}

func (c *Client) GetAppSizes(ctx context.Context) (*AppSizeCatalog, error) {
	var catalog AppSizeCatalog
	err := c.do(ctx, http.MethodGet, "/app-sizes", nil, &catalog)
	return &catalog, err
}

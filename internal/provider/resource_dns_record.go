package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.Resource = (*dnsRecordResource)(nil)
var _ resource.ResourceWithConfigure = (*dnsRecordResource)(nil)
var _ resource.ResourceWithImportState = (*dnsRecordResource)(nil)

func NewDNSRecordResource() resource.Resource { return &dnsRecordResource{} }

type dnsRecordResource struct{ client *client.Client }

type dnsRecordModel struct {
	ID       types.String `tfsdk:"id"`
	DomainID types.String `tfsdk:"domain_id"`
	Type     types.String `tfsdk:"type"`
	Name     types.String `tfsdk:"name"`
	Value    types.String `tfsdk:"value"`
	TTL      types.Int64  `tfsdk:"ttl"`
	Priority types.Int64  `tfsdk:"priority"`
}

func (r *dnsRecordResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_record"
}

func (r *dnsRecordResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *dnsRecordResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":        schema.StringAttribute{Computed: true},
			"domain_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"type":      schema.StringAttribute{Required: true},
			"name":      schema.StringAttribute{Required: true},
			"value":     schema.StringAttribute{Required: true},
			"ttl":       schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(3600)},
			"priority":  schema.Int64Attribute{Optional: true},
		},
	}
}

func (r *dnsRecordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dnsRecordModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	record, err := r.client.CreateDNSRecord(ctx, plan.DomainID.ValueString(), plan.request())
	if err != nil {
		resp.Diagnostics.AddError("Create StackShift DNS record failed", err.Error())
		return
	}
	plan.applyRecord(record)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dnsRecordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dnsRecordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	records, err := r.client.ListDNSRecords(ctx, state.DomainID.ValueString())
	if notFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift DNS record failed", err.Error())
		return
	}
	for _, record := range records {
		if record.ID == state.ID.ValueString() {
			state.applyRecord(&record)
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *dnsRecordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan dnsRecordModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	record, err := r.client.UpdateDNSRecord(ctx, plan.DomainID.ValueString(), plan.ID.ValueString(), plan.request())
	if err != nil {
		resp.Diagnostics.AddError("Update StackShift DNS record failed", err.Error())
		return
	}
	plan.applyRecord(record)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dnsRecordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dnsRecordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteDNSRecord(ctx, state.DomainID.ValueString(), state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Delete StackShift DNS record failed", err.Error())
	}
}

func (r *dnsRecordResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := splitCompositeID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func (m dnsRecordModel) request() client.DNSRecordRequest {
	return client.DNSRecordRequest{
		RecordType: m.Type.ValueString(),
		Name:       m.Name.ValueString(),
		Value:      m.Value.ValueString(),
		TTL:        int(m.TTL.ValueInt64()),
		Priority:   intPtr(m.Priority),
	}
}

func (m *dnsRecordModel) applyRecord(record *client.DNSRecord) {
	m.ID = types.StringValue(record.ID)
	m.DomainID = types.StringValue(record.DomainID)
	m.Type = types.StringValue(record.RecordType)
	m.Name = types.StringValue(record.Name)
	m.Value = types.StringValue(record.Value)
	m.TTL = types.Int64Value(int64(record.TTL))
	if record.Priority != nil {
		m.Priority = types.Int64Value(int64(*record.Priority))
	} else {
		m.Priority = types.Int64Null()
	}
}

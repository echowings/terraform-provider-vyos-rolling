package provider

import (
	"context"
	"maps"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/echowings/terraform-provider-vyos-rolling/internal/client"
	"github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/helpers/tools"
	"github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/provider/data"
	"github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/resource/autogen"
	extra_firewall_zone_from "github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/resource/extra/firewall_zone_from"
	extra_system_image_upgrade "github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/resource/extra/system_image_upgrade"
)

// Ensure ScaffoldingProvider satisfies various provider interfaces.
var _ provider.Provider = &VyosProvider{}

// VyosProvider defines the provider implementation.
type VyosProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	Version string
}

// VyosProviderModel Contains all global configurations for the provider
type VyosProviderModel struct {
	Endpoint               types.String `tfsdk:"endpoint"`
	Key                    types.String `tfsdk:"api_key"`
	Certificate            types.Object `tfsdk:"certificate"`
	OverwriteExistingRes   types.Bool   `tfsdk:"overwrite_existing_resources_on_create"`
	IgnoreMissingParentRes types.Bool   `tfsdk:"ignore_missing_parent_resource_on_create"`
	IgnoreChildResOnDelete types.Bool   `tfsdk:"ignore_child_resource_on_delete"`
	RemoveMissingOnRefresh types.Bool   `tfsdk:"remove_missing_on_refresh"`
	DefaultTimeouts        types.Number `tfsdk:"default_timeouts"`
	HTTPRequestRetries     types.Int64  `tfsdk:"http_request_retries"`
	ManualBindingOverrides types.Map    `tfsdk:"manual_binding_overrides"`
}

// Metadata method to define the provider type name for inclusion in each data source and resource type name.
func (p *VyosProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "vyos"
	resp.Version = p.Version
}

// Schema method to define the schema for provider-level configuration.
func (p *VyosProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{

		// TODO Support dynamic changes in description to match rolling vs lts targets
		//  Change the notice and documentation link.
		//  milestone: 5
		MarkdownDescription: "" +
			"!> This is for the rolling release of VyOS, it will automatically update when the API schemas change\n\n" +
			"-> This provider's version number MIGHT follow `<MAJOR>.<MINOR>.<VYOS ROLLING RELEASE DATE><REVISION STARTING AT ZERO>`, so Version `1.3` of this provider, " +
			"revision / bugfix release nr 9, " +
			"built with the API schemas for VyOS rolling release built on 27th of November 1970 would be have the version number `1.3.197011278`." +
			"This allows for locking to a major version, or even a spessific release of rolling VyOS. This versioning scheme is not final and might change. " +
			"The REVISION is a single didgit number, and is likely going to stay at `0` most of the time, but is useful for hotfixes and extra releases when needed.\n\n" +
			"Use Terraform to configure your VyOS instances via API calls.\n\n" +
			"## Requirements\n" +
			"To use this provider you must enable the HTTP(S) API on the target instances. See [VyOS documentation](https://docs.vyos.io/en/latest/configuration/service/https.html) for more information.\n\n",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "VyOS API Endpoint",
				Required:            true,
			},
			"api_key": schema.StringAttribute{
				MarkdownDescription: "VyOS API Key",
				Required:            true,
			},
			"certificate": schema.SingleNestedAttribute{

				Optional: true,
				Attributes: map[string]schema.Attribute{
					"disable_verify": schema.BoolAttribute{
						MarkdownDescription: "Disable remote certificate verification, useful for selfsigned certs.",
						Optional:            true,
					},
				},
			},
			"default_timeouts": schema.NumberAttribute{
				MarkdownDescription: "Default Create/Read/Update/Destroy timeouts in minutes, can be overridden on a per resource basis. If not configured, defaults to 15.",
				Optional:            true,
			},
			"http_request_retries": schema.Int64Attribute{
				MarkdownDescription: "Number of additional attempts to retry HTTP API calls when a network error occurs. Defaults to 0 (no retries).",
				Optional:            true,
			},
			"overwrite_existing_resources_on_create": schema.BoolAttribute{
				MarkdownDescription: "Disables the check to see if the resource already exists on the target machine, " +
					"resulting in possibly overwriting configs without notice." +
					"This can be helpful when trying to avoid and change many resources at once.",
				Optional: true,
			},
			"ignore_missing_parent_resource_on_create": schema.BoolAttribute{
				MarkdownDescription: "Disables the check to see if the required parent resource exists on the target machine." +
					"This can be helpful when encountering a bug with the provider.",
				Optional: true,
			},
			"ignore_child_resource_on_delete": schema.BoolAttribute{
				MarkdownDescription: "Disables the check to see if the resource has any child resources." +
					"This can be useful when only a parent resource is configured via terraform." +
					"This has no effect on global resources." +
					"\n\n  !> **WARNING:** This is extremely destructive and will delete everything below the destroyed resource.",
				Optional: true,
			},
			"remove_missing_on_refresh": schema.BoolAttribute{
				MarkdownDescription: "When true, resources that are in state but missing from the VyOS API during refresh " +
					"will be removed from state so they can be recreated on the next plan/apply. " +
					"This is useful if the device lost config after reboot. Defaults to true.",
				Optional: true,
			},
			"manual_binding_overrides": schema.MapAttribute{
				MarkdownDescription: "Optional map where keys are VyOS path prefixes (joined by spaces) and values are binding identifiers. Resources whose paths match the same identifier are committed together, which is useful for manual batching overrides.",
				Optional:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

// Configure method to configure shared clients for data source and resource implementations.
func (p *VyosProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tools.Info(ctx, "Configuring vyos provider")

	var providerModel VyosProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &providerModel)...)

	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := providerModel.Endpoint.ValueString()
	apiKey := providerModel.Key.ValueString()

	// Configuration values validation
	if endpoint == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("endpoint"),
			"VyOS Endpoint",
			"A valid http(s) endpoint is required",
		)
	}

	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"VyOS API Key",
			"API Key required to connect",
		)
	}

	// SSL settings
	disableVerifyAttr := providerModel.Certificate.Attributes()["disable_verify"]

	var disableVerify bool
	if disableVerifyAttr != nil {
		disableVerify = disableVerifyAttr.(types.Bool).ValueBool()
	} else {
		disableVerify = false
	}

	var httpRequestRetries int
	if !providerModel.HTTPRequestRetries.IsNull() && !providerModel.HTTPRequestRetries.IsUnknown() {
		retries := providerModel.HTTPRequestRetries.ValueInt64()
		if retries < 0 {
			resp.Diagnostics.AddAttributeError(
				path.Root("http_request_retries"),
				"Negative retry attempts are not allowed",
				"Please specify a value greater than or equal to zero.",
			)
			return
		}
		httpRequestRetries = int(retries)
	}

	ctxMutilators := data.CtxMutilators(endpoint, apiKey)

	// Run ctx mutilators for client
	for _, fn := range ctxMutilators {
		ctx = fn(ctx)
	}

	// Client configuration for data sources and resources
	clientInstance := client.NewClientWithRetries(ctx, endpoint, apiKey, "TODO: add useragent with provider version", disableVerify, httpRequestRetries)

	combinedBindingOverrides := maps.Clone(defaultBindingOverrides())

	if !providerModel.ManualBindingOverrides.IsNull() && !providerModel.ManualBindingOverrides.IsUnknown() {
		var overrides map[string]string
		diags := providerModel.ManualBindingOverrides.ElementsAs(ctx, &overrides, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		if combinedBindingOverrides == nil {
			combinedBindingOverrides = map[string]string{}
		}
		for prefix, binding := range overrides {
			combinedBindingOverrides[prefix] = binding
		}
	}

	if len(combinedBindingOverrides) > 0 {
		clientInstance.SetBindingOverrides(combinedBindingOverrides)
	}

	config := data.NewProviderData(clientInstance)
	config.Config.ManualBindingOverrides = combinedBindingOverrides

	// Add ctx mutilators to provider data
	config.CtxMutilatorAdd(ctxMutilators...)

	// Default timeout
	var defaultTimeout float64
	if providerModel.DefaultTimeouts.IsNull() || providerModel.DefaultTimeouts.IsUnknown() {
		defaultTimeout = 15
	} else {
		defaultTimeout, _ = providerModel.DefaultTimeouts.ValueBigFloat().Float64()
	}

	if defaultTimeout == 0.0 {
		config.Config.CrudDefaultTimeouts = 15
	} else {
		config.Config.CrudDefaultTimeouts = defaultTimeout
	}

	// CRUD checks
	config.Config.CrudSkipExistingResourceCheck = providerModel.OverwriteExistingRes.ValueBool()
	config.Config.CrudSkipCheckParentBeforeCreate = providerModel.IgnoreMissingParentRes.ValueBool()
	config.Config.CrudSkipCheckChildBeforeDelete = providerModel.IgnoreChildResOnDelete.ValueBool()
	if providerModel.RemoveMissingOnRefresh.IsNull() || providerModel.RemoveMissingOnRefresh.IsUnknown() {
		// Keep default.
	} else {
		config.Config.ReadRemoveMissingOnRefresh = providerModel.RemoveMissingOnRefresh.ValueBool()
	}

	// Send provider data
	resp.DataSourceData = config
	resp.ResourceData = config
}

func defaultBindingOverrides() map[string]string {
	return map[string]string{
		"firewall": "firewall",
	}
}

// Resources method to define the provider's resources.
func (p *VyosProvider) Resources(ctx context.Context) []func() resource.Resource {
	return append(
		autogen.GetResources(),
		extra_firewall_zone_from.New,
		extra_system_image_upgrade.New,
	)
}

// DataSources method to define the provider's data sources.
func (p *VyosProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		//NewDataSource,
	}
}

// New method to return the provider generator function
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &VyosProvider{
			Version: version,
		}
	}
}

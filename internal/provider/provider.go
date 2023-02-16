package provider

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure CockroachGKEProvider satisfies various provider interfaces.
var _ provider.Provider = &CockroachGKEProvider{}

// New is a helper function to simplify provider server and testing implementation.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &CockroachGKEProvider{
			version: version,
		}
	}
}

// CockroachGKEProvider defines the provider implementation.
type CockroachGKEProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// CockroachGKEProviderModel describes the provider data model.
type CockroachGKEProviderModel struct {
	Host     types.String `tfsdk:"host"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
	CertPath types.String `tfsdk:"certpath"`
}

func (p *CockroachGKEProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "cockroachgke"
	resp.Version = p.version
}

func (p *CockroachGKEProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Interact with Cockroach.",
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Description: "Host for the Cockroach database.",
				Optional:    false,
			},
			"username": schema.StringAttribute{
				Description: "Username for the Cockroach user with cluster admin permissions.",
				Optional:    false,
			},
			"password": schema.StringAttribute{
				Description: "Password for the Cockroach user with cluster admin permissions.",
				Optional:    false,
				Sensitive:   true,
			},
			"certpath": schema.StringAttribute{
				Description: "Path to certificate authority for Cockroach cluster.",
				Optional:    false,
			},
		},
	}
}

func (p *CockroachGKEProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data CockroachGKEProviderModel

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Configuration values are now available.
	// if data.Endpoint.IsNull() { /* ... */ }
	if data.Host.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("host"),
			"Unknown Cockroach database host",
			"The provider cannot create a Cockroach database connection because there is an unknown configuration value for the Cockroach host.",
		)
	}

	if data.Username.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("username"),
			"Unknown Cockroach database username",
			"The provider cannot create a Cockroach database connection because there is an unknown configuration value for the Cockroach username.",
		)
	}

	if data.Password.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("password"),
			"Unknown Cockroach database password",
			"The provider cannot create a Cockroach database connection because there is an unknown configuration value for the Cockroach password.",
		)
	}

	if data.CertPath.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("certpath"),
			"Unknown Cockroach database cert path",
			"The provider cannot create a Cockroach database connection because there is an unknown configuration value for the path to the Cockroach certificate authority.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// if data.Host.ValueString() == "" {
	// 	resp.Diagnostics.AddAttributeError(
	// 		path.Root("host"),
	// 		"Missing Cockroach database host",
	// 		"The provider cannot create a Cockroach database connection because there is a missing configuration value for the Cockroach host.",
	// 	)
	// }

	// if data.Username.ValueString() == "" {
	// 	resp.Diagnostics.AddAttributeError(
	// 		path.Root("username"),
	// 		"Missing Cockroach database username",
	// 		"The provider cannot create a Cockroach database connection because there is a missing configuration value for the Cockroach username.",
	// 	)
	// }

	// if data.Password.ValueString() == "" {
	// 	resp.Diagnostics.AddAttributeError(
	// 		path.Root("password"),
	// 		"Missing Cockroach database password",
	// 		"The provider cannot create a Cockroach database connection because there is a missing configuration value for the Cockroach password.",
	// 	)
	// }

	// if data.CertPath.ValueString() == "" {
	// 	resp.Diagnostics.AddAttributeError(
	// 		path.Root("certpath"),
	// 		"Missing Cockroach database cert path",
	// 		"The provider cannot create a Cockroach database connection because there is a missing configuration value for the path to the Cockroach certificate authority.",
	// 	)
	// }

	// if resp.Diagnostics.HasError() {
	// 	return
	// }

	// Create connection to cockroach cluster
	cnx := generateConnectionString(data)
	client, err := connectToCockroach(cnx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to connect to cockroach",
			err.Error(),
		)
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *CockroachGKEProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewExampleDataSource,
	}
}

func (p *CockroachGKEProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewExampleResource,
	}
}

// Helper functions for cockroach connection
func generateConnectionString(model CockroachGKEProviderModel) string {
	cnxStr := fmt.Sprintf("postgres://%s:%s@%s:26257?sslmode=verify-full&sslrootcert=%s",
		model.Username,
		model.Password,
		model.Host,
		model.CertPath,
	)
	return cnxStr
}

func connectToCockroach(cnx string) (*sql.DB, error) {
	db, err := sql.Open("postgres", cnx)
	if err != nil {
		return nil, err
	}
	return db, nil
}

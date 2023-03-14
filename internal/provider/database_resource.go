package provider

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	// "github.com/hashicorp/terraform-plugin-log/tflog"
	_ "github.com/lib/pq"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &DatabaseResource{}
var _ resource.ResourceWithImportState = &DatabaseResource{}

func NewDatabaseResource() resource.Resource {
	return &DatabaseResource{}
}

// DatabaseResource defines the resource implementation. Contains the cockroach client connection string.
type DatabaseResource struct {
	db *CockroachClient
}

// DatabaseResourceModel describes the resource data model.
type DatabaseResourceModel struct {
	Name              types.String `tfsdk:"name"`
	DisableProtection types.Bool   `tfsdk:"disable_protection"`
}

// Metadata appends the resource name to the provider name
func (r *DatabaseResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database"
}

// Schema is the shape of the resource - what you need to supply
func (r *DatabaseResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Database resource",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the database",
				Required:            true,
			},
			"disable_protection": schema.BoolAttribute{
				MarkdownDescription: "Optional disable delete protection for tables",
				Optional:            true,
			},
		},
	}
}

// Configure adds the provider configured client to the resource
func (r *DatabaseResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	r.db = req.ProviderData.(*CockroachClient)
}

// Create is for creating the database resource
func (r *DatabaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *DatabaseResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := r.db.Connect()
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to connect to cockroach",
			err.Error(),
		)
		return
	}
	defer client.Close()

	sql := fmt.Sprintf("CREATE DATABASE %s", data.Name.String())
	_, err = client.Exec(sql)
	if err != nil {
		resp.Diagnostics.AddError("Create db error", fmt.Sprintf("Unable to create database, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created a database")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read is called first each time - reads the cockroach internals for existing databases
func (r *DatabaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *DatabaseResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := r.db.Connect()
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to connect to cockroach",
			err.Error(),
		)
		return
	}

	queryName := strings.Replace(data.Name.String(), "\"", "", -1)
	var name string

	q := fmt.Sprintf("SELECT name FROM crdb_internal.databases WHERE name = '%s'", queryName)
	err = client.QueryRow(q).Scan(&name)

	if err == sql.ErrNoRows {
		data.Name = types.StringValue(name)
		resp.State.RemoveResource(ctx)
	}

	if types.StringValue(name) != data.Name {
		data.Name = types.StringValue(name)
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}

	defer client.Close()

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatabaseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *DatabaseResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete resource from crdb
func (r *DatabaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *DatabaseResourceModel
	req.State.Get(ctx, &data)

	client, err := r.db.Connect()
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to connect to cockroach",
			err.Error(),
		)
		return
	}
	defer client.Close()

	sql := ""
	disabled := data.DisableProtection.ValueBool()

	if disabled {
		sql = fmt.Sprintf("DROP DATABASE %s CASCADE", data.Name.String())
	} else {
		sql = fmt.Sprintf("DROP DATABASE %s RESTRICT", data.Name.String())
	}

	_, err = client.Exec(sql)
	if err != nil {
		resp.Diagnostics.AddError("Delete db error", fmt.Sprintf("Unable to delete database, got error: %s", err))
		return
	}
	tflog.Trace(ctx, "deleted a database")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatabaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

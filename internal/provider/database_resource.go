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
	Name types.String `tfsdk:"name"`
}

// Metadata appends the resource name to the provider name
func (r *DatabaseResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database"
}

// Schema is the shape of the resource - what you need to supply
func (r *DatabaseResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Database resource",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the database",
				Required:            true,
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

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create cockroach connection, defer close
	client, err := r.db.Connect()
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to connect to cockroach",
			err.Error(),
		)
		return
	}
	defer client.Close()

	// Call the actual SQL for db creation
	sql := fmt.Sprintf("CREATE DATABASE %s", data.Name.String())
	_, err = client.Exec(sql)
	if err != nil {
		resp.Diagnostics.AddError("Create db error", fmt.Sprintf("Unable to create database, got error: %s", err))
		return
	}

	// For the purposes of this example code, hardcoding a response value to
	// save into the Terraform state.
	// data.Name = types.StringValue(data.Name.String())

	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	tflog.Trace(ctx, "created a database")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read is called first each time - reads the cockroach internals for existing databases
func (r *DatabaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *DatabaseResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Connect to crdb
	client, err := r.db.Connect()
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to connect to cockroach",
			err.Error(),
		)
		return
	}

	// Get the name of the database in state
	queryName := strings.Replace(data.Name.String(), "\"", "", -1)
	var name string

	// Query crdb for that database name
	q := fmt.Sprintf("SELECT name FROM crdb_internal.databases WHERE name = '%s'", queryName)
	err = client.QueryRow(q).Scan(&name)
	//resp.Diagnostics.AddError("Read db error", fmt.Sprintf("Unable to read database, got error: %s", queryName))
	// If no rows come back, remove the resource from state because it shouldn't be there
	if err == sql.ErrNoRows {
		data.Name = types.StringValue(name)
		resp.State.RemoveResource(ctx)
		//resp.Diagnostics.AddError("Read db error", fmt.Sprintf("Unable to read database, got error: %s", err))
		//return
	}

	// This might not be doing anything lol
	if types.StringValue(name) != data.Name {
		data.Name = types.StringValue(name)
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}

	defer client.Close()

	// Save updated data into Terraform state
	//resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatabaseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *DatabaseResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete resource from crdb
func (r *DatabaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *DatabaseResourceModel
	req.State.Get(ctx, &data)

	// db connection
	client, err := r.db.Connect()
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to connect to cockroach",
			err.Error(),
		)
		return
	}
	defer client.Close()

	// sql for db deletion
	sql := fmt.Sprintf("DROP DATABASE %s", data.Name.String())
	_, err = client.Exec(sql)
	if err != nil {
		resp.Diagnostics.AddError("Delete db error", fmt.Sprintf("Unable to delete database, got error: %s", err))
		return
	}

	// For the purposes of this example code, hardcoding a response value to
	// save into the Terraform state.
	// data.Name = types.StringValue(data.Name.String())

	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	tflog.Trace(ctx, "created a database")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatabaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

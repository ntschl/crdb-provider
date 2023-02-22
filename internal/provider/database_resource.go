package provider

import (
	"context"
	"database/sql"
	"fmt"

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

// DatabaseResource defines the resource implementation.
type DatabaseResource struct {
	db *sql.DB
}

// DatabaseResourceModel describes the resource data model.
type DatabaseResourceModel struct {
	Name types.String `tfsdk:"name"`
}

func (r *DatabaseResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database"
}

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

func (r *DatabaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *DatabaseResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cnx := "postgresql://root@localhost:26257/defaultdb?sslmode=disable"
	client, err := connectToCockroach(cnx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to connect to cockroach",
			err.Error(),
		)
		return
	}

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

func (r *DatabaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *DatabaseResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
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

func (r *DatabaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *DatabaseResourceModel
	req.State.Get(ctx, &data)

	cnx := "postgresql://root@localhost:26257/defaultdb?sslmode=disable"
	client, err := connectToCockroach(cnx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to connect to cockroach",
			err.Error(),
		)
		return
	}
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

func connectToCockroach(cnx string) (*sql.DB, error) {
	db, err := sql.Open("postgres", cnx)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func generateConnectionString(model CockroachGKEProviderModel) string {
	cnxStr := fmt.Sprintf("postgres://%s:%s@%s:26257?sslmode=verify-full&sslrootcert=%s",
		model.Username,
		model.Password,
		model.Host,
		model.CertPath,
	)
	return cnxStr
}

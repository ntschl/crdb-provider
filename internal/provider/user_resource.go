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
var _ resource.Resource = &UserResource{}
var _ resource.ResourceWithImportState = &UserResource{}

func NewUserResource() resource.Resource {
	return &UserResource{}
}

// UserResource defines the resource implementation. Contains the cockroach client connection string.
type UserResource struct {
	db *CockroachClient
}

// UserResourceModel describes the resource data model.
type UserResourceModel struct {
	Username   types.String `tfsdk:"username"`
	Password   types.String `tfsdk:"password"`
	Database   types.String `tfsdk:"database"`
	Privileges types.String `tfsdk:"privileges"`
}

// var privileges = map[string]bool{
// 	"select": false,
// 	"insert": false,
// 	"update": false,
// 	"delete": false,
// }

// Metadata appends the resource name to the provider name
func (r *UserResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

// Schema is the shape of the resource - what you need to supply
func (r *UserResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "User resource",
		Attributes: map[string]schema.Attribute{
			"username": schema.StringAttribute{
				MarkdownDescription: "Name of the user",
				Required:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Password of the user",
				Required:            true,
			},
			"database": schema.StringAttribute{
				MarkdownDescription: "Database to which the user belongs",
				Required:            true,
			},
			"privileges": schema.StringAttribute{
				MarkdownDescription: "Privileges of the user",
				Optional:            true,
			},
		},
	}
}

// Configure adds the provider configured client to the resource
func (r *UserResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	r.db = req.ProviderData.(*CockroachClient)
}

// Create is for creating the user resource
func (r *UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *UserResourceModel
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

	pw := strings.Replace(data.Password.String(), "\"", "", -1)

	query := fmt.Sprintf("SET DATABASE=%s; CREATE USER %s WITH PASSWORD '%s';", data.Database, data.Username, pw)
	_, err = client.Exec(query)
	if err != nil {
		resp.Diagnostics.AddError("Create user error", fmt.Sprintf("Unable to create user, got error: %s", err))
		return
	}

	var tables string
	alter := fmt.Sprintf("ALTER DEFAULT PRIVILEGES FOR ALL ROLES GRANT %s ON TABLES TO %s;", data.Privileges, data.Username)
	grant := fmt.Sprintf("GRANT SELECT ON * TO %s;", data.Username)
	err = client.QueryRow("SHOW TABLES;").Scan(&tables)
	if err == sql.ErrNoRows {
		client.Exec(alter)
	} else {
		client.Exec(grant)
		client.Exec(alter)
	}
	//resp.Diagnostics.AddError("Set db error", fmt.Sprintf("Unable to set db, got error: %s", err.Error()))

	tflog.Trace(ctx, "created a user")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *UserResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *UserResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *UserResourceModel
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
	defer client.Close()

	alter := fmt.Sprintf("SET DATABASE=%s; ALTER DEFAULT PRIVILEGES FOR ALL ROLES REVOKE ALL ON TABLES FROM %s; ", data.Database, data.Username)
	revoke := fmt.Sprintf("REVOKE %s ON * FROM %s; ", data.Privileges, data.Username)
	delete := fmt.Sprintf("DROP USER %s;", data.Username)

	var tables string
	err = client.QueryRow(fmt.Sprintf("SET DATABASE=%s; SHOW TABLES;", data.Database)).Scan(&tables)
	if err == sql.ErrNoRows {
		_, err = client.Exec(alter + delete)
		if err != nil {
			resp.Diagnostics.AddError("Delete user error (no tables)", fmt.Sprintf("Unable to delete user, got error: %s", err))
			return
		}
	} else {
		_, err = client.Exec(alter + revoke + delete)
		if err != nil {
			resp.Diagnostics.AddError("Delete user error (tables)", fmt.Sprintf("Unable to delete user, got error: %s", err))
			return
		}
	}

	tflog.Trace(ctx, "deleted a user")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

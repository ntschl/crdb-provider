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
	"golang.org/x/exp/slices"

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
	Privileges types.List   `tfsdk:"privileges"`
}

var privilegeSlice = []string{"select", "update", "insert", "delete"}

// Metadata appends the resource name to the provider name
func (r *UserResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

// Schema is the shape of the resource - what you need to supply
func (r *UserResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
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
			"privileges": schema.ListAttribute{
				ElementType:         types.StringType,
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
	privString := ""
	privList := data.Privileges.Elements()
	last := len(privList) - 1
	for i, s := range privList {
		if !slices.Contains(privilegeSlice, strings.Replace(s.String(), "\"", "", -1)) {
			resp.Diagnostics.AddError("Invalid privilege", fmt.Sprintf("Unable to set invalid privilege: %s", s))
			return
		}
		if i < last {
			privString = privString + s.String() + ", "
		} else {
			privString = privString + s.String()
		}
	}
	privileges := strings.Replace(privString, "\"", "", -1)

	query := fmt.Sprintf("SET DATABASE=%s; CREATE USER %s WITH PASSWORD '%s';", data.Database, data.Username, pw)
	_, err = client.Exec(query)
	if err != nil {
		resp.Diagnostics.AddError("Create user error", fmt.Sprintf("Unable to create user, got error: %s", err))
		return
	}

	var tables string
	alter := fmt.Sprintf("ALTER DEFAULT PRIVILEGES FOR ALL ROLES GRANT %s ON TABLES TO %s;", privileges, data.Username)
	grant := fmt.Sprintf("GRANT %s ON * TO %s;", privileges, data.Username)
	err = client.QueryRow("SHOW TABLES;").Scan(&tables)
	if err == sql.ErrNoRows {
		client.Exec(alter)
	} else {
		client.Exec(grant)
		client.Exec(alter)
	}

	tflog.Trace(ctx, "created a user")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
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

	queryName := strings.Replace(data.Username.String(), "\"", "", -1)
	type rowData struct {
		db        string
		schema    string
		relation  string
		grantee   string
		privilege string
		grantable string
	}
	privilegeReadSlice := []string{}

	q := fmt.Sprintf("SET DATABASE=%s; SHOW GRANTS FOR %s", data.Database, queryName)

	rows, err := client.Query(q)
	if err != nil {
		resp.State.RemoveResource(ctx)
		return
	} else {
		for rows.Next() {
			rowDataStruct := rowData{}
			rows.Scan(&rowDataStruct.db, &rowDataStruct.schema, &rowDataStruct.relation, &rowDataStruct.grantee, &rowDataStruct.privilege, &rowDataStruct.grantable)
			if !slices.Contains(privilegeReadSlice, rowDataStruct.privilege) {
				privilegeReadSlice = append(privilegeReadSlice, rowDataStruct.privilege)
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	defer client.Close()
}

func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *UserResourceModel
	var state *UserResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

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

	alter := ""
	revoke := ""
	delete := ""

	// Check for username change
	if state.Username != data.Username {
		alter = fmt.Sprintf("SET DATABASE=%s; ALTER DEFAULT PRIVILEGES FOR ALL ROLES REVOKE ALL ON TABLES FROM %s; ", data.Database, state.Username)
		revoke = fmt.Sprintf("REVOKE ALL ON * FROM %s; ", state.Username)
		delete = fmt.Sprintf("DROP USER %s;", state.Username)
	} else {
		// DELETE THE USER - CAN WE JUST CALL DELETE INSTEAD OF REPEATING THE CODE?
		alter = fmt.Sprintf("SET DATABASE=%s; ALTER DEFAULT PRIVILEGES FOR ALL ROLES REVOKE ALL ON TABLES FROM %s; ", data.Database, data.Username)
		revoke = fmt.Sprintf("REVOKE ALL ON * FROM %s; ", data.Username)
		delete = fmt.Sprintf("DROP USER %s;", data.Username)
	}

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

	// CREATE THE USER AGAIN - CAN WE CALL CREATE INSTEAD OF REPEATING THE CODE
	pw := strings.Replace(data.Password.String(), "\"", "", -1)
	privString := ""
	privList := data.Privileges.Elements()
	last := len(privList) - 1
	for i, s := range privList {
		if !slices.Contains(privilegeSlice, strings.Replace(s.String(), "\"", "", -1)) {
			resp.Diagnostics.AddError("Invalid privilege", fmt.Sprintf("Unable to set invalid privilege: %s", s))
			return
		}
		if i < last {
			privString = privString + s.String() + ", "
		} else {
			privString = privString + s.String()
		}
	}
	privileges := strings.Replace(privString, "\"", "", -1)

	query := fmt.Sprintf("SET DATABASE=%s; CREATE USER %s WITH PASSWORD '%s';", data.Database, data.Username, pw)
	_, err = client.Exec(query)
	if err != nil {
		resp.Diagnostics.AddError("Create user error", fmt.Sprintf("Unable to create user, got error: %s", err))
		return
	}

	var tables2 string
	alter = fmt.Sprintf("ALTER DEFAULT PRIVILEGES FOR ALL ROLES GRANT %s ON TABLES TO %s;", privileges, data.Username)
	grant := fmt.Sprintf("GRANT %s ON * TO %s;", privileges, data.Username)
	err = client.QueryRow("SHOW TABLES;").Scan(&tables2)
	if err == sql.ErrNoRows {
		client.Exec(alter)
	} else {
		client.Exec(grant)
		client.Exec(alter)
	}

	tflog.Trace(ctx, "created a user")
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
	revoke := fmt.Sprintf("REVOKE ALL ON * FROM %s; ", data.Username)
	delete := fmt.Sprintf("DROP USER %s;", data.Username)

	var delTables string
	err = client.QueryRow(fmt.Sprintf("SET DATABASE=%s; SHOW TABLES;", data.Database)).Scan(&delTables)
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
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

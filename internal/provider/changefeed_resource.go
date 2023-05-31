package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func NewChangefeedResource() resource.Resource {
	return &ChangefeedResource{}
}

// ChangefeedResource defines the resource implementation. Contains the cockroach client connection string.
type ChangefeedResource struct {
	db *CockroachClient
}

// ChangefeedResourceModel describes the resource data model.
type ChangefeedResourceModel struct {
	TableName  types.String `tfsdk:"table"`
	BucketName types.String `tfsdk:"bucket"`
	Token      types.String `tfsdk:"token"`
	Database   types.String `tfsdk:"database"`
	JobID      types.String `tfsdk:"job_id"`
}

// Metadata appends the resource name to the provider name
func (r *ChangefeedResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_changefeed"
}

// Schema is the shape of the resource - what you need to supply
func (r *ChangefeedResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Changefeed resource",
		Attributes: map[string]schema.Attribute{
			"table": schema.StringAttribute{
				MarkdownDescription: "Name of the table receiving the changefeed",
				Required:            true,
			},
			"bucket": schema.StringAttribute{
				MarkdownDescription: "Bucket to send the changefeed to",
				Required:            true,
			},
			"token": schema.StringAttribute{
				MarkdownDescription: "Optional disable delete protection for tables",
				Required:            true,
			},
			"database": schema.StringAttribute{
				MarkdownDescription: "Database for the tables receiving a changefeed",
				Required:            true,
			},
			"job_id": schema.StringAttribute{
				MarkdownDescription: "ID returned for the changefeed",
				Computed:            true,
			},
		},
	}
}

// Configure adds the provider configured client to the resource
func (r *ChangefeedResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	r.db = req.ProviderData.(*CockroachClient)
}

func (r *ChangefeedResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *ChangefeedResourceModel

	// Read Terraform plan data into the model
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

	database := strings.Replace(data.Database.String(), "\"", "", -1)
	table := strings.Replace(data.TableName.String(), "\"", "", -1)
	bucket := strings.Replace(data.BucketName.String(), "\"", "", -1)
	token := strings.Replace(data.Token.String(), "\"", "", -1)
	query := fmt.Sprintf("SET DATABASE=%s; CREATE CHANGEFEED FOR TABLE %s INTO 'gs://%s?AUTH=specified&CREDENTIALS=%s';", database, table, bucket, token)

	var id string
	err = client.QueryRow(query).Scan(&id)
	if err != nil {
		resp.Diagnostics.AddError("Create changefeed error", fmt.Sprintf("Unable to create changefeed, got error: %s", err))
		return
	}
	data.JobID = types.StringValue(id)

	// Documentation: https://terraform.io/plugin/log
	tflog.Trace(ctx, "created a changefeed")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ChangefeedResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *ChangefeedResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// httpResp, err := r.client.Do(httpReq)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read example, got error: %s", err))
	//     return
	// }

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ChangefeedResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *ChangefeedResourceModel
	var data2 *ChangefeedResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &data2)...)

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

	db := strings.Replace(data.Database.String(), "\"", "", -1)
	id := strings.Replace(data2.JobID.String(), "\"", "", -1)

	deleteQuery := fmt.Sprintf("SET DATABASE=%s; CANCEL JOB %s;", db, id)
	_, err = client.Exec(deleteQuery)
	if err != nil {
		resp.Diagnostics.AddError("Update changefeed error (cancel)", fmt.Sprintf("Unable to update changefeed, got error: %s %s %s", err, db, id))
		return
	}

	database := strings.Replace(data.Database.String(), "\"", "", -1)
	table := strings.Replace(data.TableName.String(), "\"", "", -1)
	bucket := strings.Replace(data.BucketName.String(), "\"", "", -1)
	token := strings.Replace(data.Token.String(), "\"", "", -1)
	query := fmt.Sprintf("SET DATABASE=%s; CREATE CHANGEFEED FOR TABLE %s INTO 'gs://%s?AUTH=specified&CREDENTIALS=%s';", database, table, bucket, token)

	id = ""
	err = client.QueryRow(query).Scan(&id)
	if err != nil {
		resp.Diagnostics.AddError("Update changefeed error (create)", fmt.Sprintf("Unable to update changefeed, got error: %s", err))
		return
	}
	data.JobID = types.StringValue(id)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ChangefeedResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *ChangefeedResourceModel

	// Read Terraform prior state data into the model
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

	db := strings.Replace(data.Database.String(), "\"", "", -1)
	id := strings.Replace(data.JobID.String(), "\"", "", -1)

	query := fmt.Sprintf("SET DATABASE=%s; CANCEL JOB %s;", db, id)
	_, err = client.Exec(query)
	if err != nil {
		resp.Diagnostics.AddError("Delete changefeed error", fmt.Sprintf("Unable to delete changefeed, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "deleted a changefeed")
}

func (r *ChangefeedResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

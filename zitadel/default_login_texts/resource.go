package default_login_texts

import (
	"context"
	"fmt"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/admin"
	textpb "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/text"
	"google.golang.org/protobuf/encoding/protojson"

	generatedtext "github.com/zitadel/terraform-provider-zitadel/v2/gen/github.com/zitadel/zitadel/pkg/grpc/text"
	"github.com/zitadel/terraform-provider-zitadel/v2/zitadel/helper"
)

const (
	LanguageVar = "language"
)

var (
	_ resource.Resource = &defaultLoginTextsResource{}
)

func New() resource.Resource {
	return &defaultLoginTextsResource{}
}

type defaultLoginTextsResource struct {
	clientInfo *helper.ClientInfo
}

func (r *defaultLoginTextsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_default_login_texts"
}

func (r *defaultLoginTextsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	// Get the generated schema - this returns a provider schema, so we need to convert it
	providerSchema, d := generatedtext.GenSchemaLoginCustomText(ctx)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert provider schema to resource schema
	resourceAttrs := make(map[string]schema.Attribute)
	for name, attr := range providerSchema.Attributes {
		if name != "org_id" { // Skip org_id attribute
			resourceAttrs[name] = convertProviderAttrToResourceAttr(attr)
		}
	}

	resp.Schema = schema.Schema{
		Attributes:          resourceAttrs,
		Description:         providerSchema.Description,
		MarkdownDescription: providerSchema.MarkdownDescription,
		DeprecationMessage:  providerSchema.DeprecationMessage,
	}
}

func (r *defaultLoginTextsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	clientInfo, ok := req.ProviderData.(*helper.ClientInfo)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Configure Provider Data Type",
			fmt.Sprintf("Expected *helper.ClientInfo, got: %T", req.ProviderData),
		)
		return
	}
	r.clientInfo = clientInfo
}

func (r *defaultLoginTextsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	language := getPlanAttrs(ctx, req.Plan, resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan types.Object
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	obj := textpb.LoginCustomText{}
	resp.Diagnostics.Append(generatedtext.CopyLoginCustomTextFromTerraform(ctx, plan, &obj)...)
	if resp.Diagnostics.HasError() {
		return
	}

	jsonpb := &runtime.JSONPb{
		UnmarshalOptions: protojson.UnmarshalOptions{
			DiscardUnknown: true,
		},
	}
	data, err := jsonpb.Marshal(&obj)
	if err != nil {
		resp.Diagnostics.AddError("failed to marshal protobuf object", err.Error())
		return
	}
	zReq := &admin.SetCustomLoginTextsRequest{}
	if err := jsonpb.Unmarshal(data, zReq); err != nil {
		resp.Diagnostics.AddError("failed to unmarshal into ZITADEL request", err.Error())
		return
	}
	zReq.Language = language

	client, err := helper.GetAdminClient(ctx, r.clientInfo)
	if err != nil {
		resp.Diagnostics.AddError("failed to get client", err.Error())
		return
	}

	_, err = client.SetCustomLoginText(ctx, zReq)
	if err != nil {
		resp.Diagnostics.AddError("failed to create default login texts", err.Error())
		return
	}

	setID(&plan, language)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *defaultLoginTextsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state types.Object
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	language := getID(ctx, state)

	client, err := helper.GetAdminClient(ctx, r.clientInfo)
	if err != nil {
		resp.Diagnostics.AddError("failed to get client", err.Error())
		return
	}

	zResp, err := client.GetCustomLoginTexts(ctx, &admin.GetCustomLoginTextsRequest{Language: language})
	if err != nil {
		// Handle resource not found - check if error indicates resource doesn't exist
		if isResourceNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("failed to read default login texts", err.Error())
		return
	}

	if zResp.GetCustomText().GetIsDefault() {
		return
	}

	resp.Diagnostics.Append(generatedtext.CopyLoginCustomTextToTerraform(ctx, zResp.GetCustomText(), &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	setID(&state, language)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *defaultLoginTextsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	language := getPlanAttrs(ctx, req.Plan, resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan types.Object
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	obj := textpb.LoginCustomText{}
	resp.Diagnostics.Append(generatedtext.CopyLoginCustomTextFromTerraform(ctx, plan, &obj)...)
	if resp.Diagnostics.HasError() {
		return
	}

	jsonpb := &runtime.JSONPb{
		UnmarshalOptions: protojson.UnmarshalOptions{
			DiscardUnknown: true,
		},
	}
	data, err := jsonpb.Marshal(&obj)
	if err != nil {
		resp.Diagnostics.AddError("failed to marshal protobuf object", err.Error())
		return
	}
	zReq := &admin.SetCustomLoginTextsRequest{}
	if err := jsonpb.Unmarshal(data, zReq); err != nil {
		resp.Diagnostics.AddError("failed to unmarshal into ZITADEL request", err.Error())
		return
	}
	zReq.Language = language

	client, err := helper.GetAdminClient(ctx, r.clientInfo)
	if err != nil {
		resp.Diagnostics.AddError("failed to get client", err.Error())
		return
	}

	_, err = client.SetCustomLoginText(ctx, zReq)
	if err != nil {
		resp.Diagnostics.AddError("failed to update default login texts", err.Error())
		return
	}

	setID(&plan, language)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *defaultLoginTextsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state types.Object
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	language := getStateAttrsFromObject(ctx, state, resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := helper.GetAdminClient(ctx, r.clientInfo)
	if err != nil {
		resp.Diagnostics.AddError("failed to get client", err.Error())
		return
	}

	_, err = client.ResetCustomLoginTextToDefault(ctx, &admin.ResetCustomLoginTextsToDefaultRequest{Language: language})
	if err != nil {
		if isResourceNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("failed to delete default login texts", err.Error())
		return
	}
}

func setID(obj *types.Object, language string) {
	if obj.IsNull() || obj.IsUnknown() {
		return
	}
	attrs := obj.Attributes()
	if attrs == nil {
		return
	}
	attrs["id"] = types.StringValue(language)
	attrs[LanguageVar] = types.StringValue(language)
}

func getID(ctx context.Context, obj types.Object) string {
	if obj.IsNull() || obj.IsUnknown() {
		return ""
	}
	return helper.GetStringFromAttr(ctx, obj.Attributes(), "id")
}

func getPlanAttrs(ctx context.Context, plan tfsdk.Plan, diags diag.Diagnostics) string {
	var language types.String
	diags.Append(plan.GetAttribute(ctx, path.Root(LanguageVar), &language)...)
	if diags.HasError() || language.IsNull() || language.IsUnknown() {
		return ""
	}
	return language.ValueString()
}

func getStateAttrsFromObject(ctx context.Context, obj types.Object, diags diag.Diagnostics) string {
	if obj.IsNull() || obj.IsUnknown() {
		return ""
	}

	attrs := obj.Attributes()
	if attrs == nil {
		return ""
	}

	if langAttr, exists := attrs[LanguageVar]; exists {
		if langStr, ok := langAttr.(types.String); ok && !langStr.IsNull() && !langStr.IsUnknown() {
			return langStr.ValueString()
		}
	}

	return ""
}

// Helper function to convert provider schema attributes to resource schema attributes
func convertProviderAttrToResourceAttr(attr providerschema.Attribute) schema.Attribute {
	switch v := attr.(type) {
	case providerschema.StringAttribute:
		return schema.StringAttribute{
			Description:         v.Description,
			MarkdownDescription: v.MarkdownDescription,
			Required:            v.Required,
			Optional:            v.Optional,
			Computed:            false,
			Sensitive:           v.Sensitive,
			DeprecationMessage:  v.DeprecationMessage,
		}
	case providerschema.BoolAttribute:
		return schema.BoolAttribute{
			Description:         v.Description,
			MarkdownDescription: v.MarkdownDescription,
			Required:            v.Required,
			Optional:            v.Optional,
			Computed:            false,
			Sensitive:           v.Sensitive,
			DeprecationMessage:  v.DeprecationMessage,
		}
	case providerschema.Int64Attribute:
		return schema.Int64Attribute{
			Description:         v.Description,
			MarkdownDescription: v.MarkdownDescription,
			Required:            v.Required,
			Optional:            v.Optional,
			Computed:            false,
			Sensitive:           v.Sensitive,
			DeprecationMessage:  v.DeprecationMessage,
		}
	case providerschema.SingleNestedAttribute:
		nestedAttrs := make(map[string]schema.Attribute)
		for name, nestedAttr := range v.Attributes {
			nestedAttrs[name] = convertProviderAttrToResourceAttr(nestedAttr)
		}
		return schema.SingleNestedAttribute{
			Description:         v.Description,
			MarkdownDescription: v.MarkdownDescription,
			Required:            v.Required,
			Optional:            v.Optional,
			Computed:            false,
			Sensitive:           v.Sensitive,
			DeprecationMessage:  v.DeprecationMessage,
			Attributes:          nestedAttrs,
		}
	case providerschema.ListNestedAttribute:
		nestedAttrs := make(map[string]schema.Attribute)
		for name, nestedAttr := range v.NestedObject.Attributes {
			nestedAttrs[name] = convertProviderAttrToResourceAttr(nestedAttr)
		}
		return schema.ListNestedAttribute{
			Description:         v.Description,
			MarkdownDescription: v.MarkdownDescription,
			Required:            v.Required,
			Optional:            v.Optional,
			Computed:            false,
			Sensitive:           v.Sensitive,
			DeprecationMessage:  v.DeprecationMessage,
			NestedObject: schema.NestedAttributeObject{
				Attributes: nestedAttrs,
			},
		}
	default:
		return schema.StringAttribute{
			Optional: true,
		}
	}
}

// Helper function to check if an error indicates a resource was not found
func isResourceNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "not_found") ||
		strings.Contains(errStr, "does not exist")
}
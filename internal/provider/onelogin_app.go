package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &oneloginAppResource{}
	_ resource.ResourceWithConfigure   = &oneloginAppResource{}
	_ resource.ResourceWithImportState = &oneloginAppResource{}
)

type oneloginAppResource struct {
	client *client
}

type oneloginApp struct {
	// Included in all apps
	ID                    types.Int64  `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	ConnectorID           types.Int64  `tfsdk:"connector_id"`
	IconURL               types.String `tfsdk:"icon_url"`
	Visible               types.Bool   `tfsdk:"visible"`
	AuthMethod            types.Int64  `tfsdk:"auth_method"`
	AuthMethodDescription types.String `tfsdk:"auth_method_description"`
	AllowAssumedSignin    types.Bool   `tfsdk:"allow_assumed_signin"`
	CreatedAt             types.String `tfsdk:"created_at"` // timestamp string
	UpdatedAt             types.String `tfsdk:"updated_at"` // timestmp string
	ProvisioningEnabled   types.Bool   `tfsdk:"provisioning_enabled"`

	// Included in some apps (can be null)
	Description types.String `tfsdk:"description"`
	TabID       types.Int64  `tfsdk:"tab_id"`
	BrandID     types.Int64  `tfsdk:"brand_id"`
	Notes       types.String `tfsdk:"notes"`
	PolicyID    types.Int64  `tfsdk:"policy_id"`

	SSO           types.Object `tfsdk:"sso"`
	Configuration types.Map    `tfsdk:"configuration"`

	Parameters types.Map `tfsdk:"parameters"`
}

type oneloginNativeApp struct {
	ID                    int64  `json:"id"`
	Name                  string `json:"name"`
	ConnectorID           int64  `json:"connector_id"`
	IconURL               string `json:"icon_url"`
	Visible               bool   `json:"visible"`
	AuthMethod            int64  `json:"auth_method"`
	AuthMethodDescription string `json:"auth_method_description"`
	AllowAssumedSignin    bool   `json:"allow_assumed_signin"`
	CreatedAt             string `json:"created_at"` // timestamp string
	UpdatedAt             string `json:"updated_at"` // timestmp string

	// Can be null
	Description *string `json:"description,omitempty"`
	TabID       *int64  `json:"tab_id,omitempty"`
	BrandID     *int64  `json:"brand_id,omitempty"`
	Notes       *string `json:"notes,omitempty"`
	PolicyID    *int64  `json:"policy_id,omitempty"`

	Provisioning *oneloginNativeAppProvisioning `json:"provisioning,omitempty"`
	SSO          *oneloginNativeAppSSO          `json:"sso,omitempty"`

	// Different for every connector
	Configuration map[string]interface{} `json:"configuration,omitempty"`

	Parameters map[string]oneloginNativeAppParameter `json:"parameters,omitempty"`
}

type oneloginNativeAppProvisioning struct {
	Enabled bool `json:"enabled"`
}

type oneloginAppParameter struct {
	ID                      types.Int64  `tfsdk:"id"`
	Label                   types.String `tfsdk:"label"`
	ProvisionedEntitlements types.Bool   `tfsdk:"provisioned_entitlements"`
	SkipIfBlank             types.Bool   `tfsdk:"skip_if_blank"`

	DefaultValues             types.String `tfsdk:"default_values"`
	UserAttributeMappings     types.String `tfsdk:"user_attribute_mappings"`
	UserAttributeMacros       types.String `tfsdk:"user_attribute_macros"`
	AttributesTransformations types.String `tfsdk:"attributes_transformations"`
	Values                    types.String `tfsdk:"values"`
	IncludeInSamlAssertion    types.Bool   `tfsdk:"include_in_saml_assertion"`
}

type oneloginNativeAppParameter struct {
	ID                      int64  `json:"id,omitempty"`
	Label                   string `json:"label,omitempty"`
	ProvisionedEntitlements bool   `json:"provisioned_entitlements,omitempty"`
	SkipIfBlank             bool   `json:"skip_if_blank,omitempty"`

	// can be nil
	DefaultValues             *string `json:"default_values,omitempty"`
	UserAttributeMappings     *string `json:"user_attribute_mappings,omitempty"`
	UserAttributeMacros       *string `json:"user_attribute_macros,omitempty"`
	AttributesTransformations *string `json:"attributes_transformations,omitempty"`
	Values                    *string `json:"values,omitempty"`
	IncludeInSAMLAssertion    *bool   `json:"include_in_saml_assertion,omitempty"`
}

func oneloginAppParameterTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":                         types.Int64Type,
		"label":                      types.StringType,
		"provisioned_entitlements":   types.BoolType,
		"skip_if_blank":              types.BoolType,
		"default_values":             types.StringType,
		"user_attribute_mappings":    types.StringType,
		"user_attribute_macros":      types.StringType,
		"attributes_transformations": types.StringType,
		"values":                     types.StringType,
		"include_in_saml_assertion":  types.BoolType,
	}
}

// SSO types
type oneloginAppSSO struct {
	ClientID         types.String `tfsdk:"client_id"`
	ClientSecret     types.String `tfsdk:"client_secret"`
	MetadataURL      types.String `tfsdk:"metadata_url"`
	ACSURL           types.String `tfsdk:"acs_url"`
	SLSURL           types.String `tfsdk:"sls_url"`
	Issuer           types.String `tfsdk:"issuer"`
	WSFED_SSO_URL    types.String `tfsdk:"wsfed_sso_url"`
	CertificateID    types.Int64  `tfsdk:"certificate_id"`
	CertificateValue types.String `tfsdk:"certificate_value"`
	CertificateName  types.String `tfsdk:"certificate_name"`
}

type oneloginNativeAppSSO struct {
	ClientID      *string                      `json:"client_id,omitempty"`
	ClientSecret  *string                      `json:"client_secret,omitempty"`
	MetadataURL   *string                      `json:"metadata_url,omitempty"`
	ACSURL        *string                      `json:"acs_url,omitempty"`
	SLSURL        *string                      `json:"sls_url,omitempty"`
	Issuer        *string                      `json:"issuer,omitempty"`
	WSFED_SSO_URL *string                      `json:"wsfed_sso_url,omitempty"`
	Certificate   oneloginNativeAppCertificate `json:"certificate,omitempty"`
}

type oneloginNativeAppCertificate struct {
	CertificateID    *int64  `json:"certificate_id,omitempty"`
	CertificateValue *string `json:"certificate_value,omitempty"`
	CertificateName  *string `json:"certificate_name,omitempty"`
}

// can probably accomplish the same with oneloginAppSSO and reflection
func oneloginAppSSOTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"client_id":         types.StringType,
		"client_secret":     types.StringType,
		"metadata_url":      types.StringType,
		"acs_url":           types.StringType,
		"sls_url":           types.StringType,
		"issuer":            types.StringType,
		"wsfed_sso_url":     types.StringType,
		"certificate_id":    types.Int64Type,
		"certificate_value": types.StringType,
		"certificate_name":  types.StringType,
	}
}

func NewOneLoginAppResource(client *client) newResourceFunc {
	return func() resource.Resource {
		return &oneloginAppResource{client}
	}
}

func (d *oneloginAppResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app"
}

func (d *oneloginAppResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
}

func (d *oneloginAppResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			"connector_id": schema.Int64Attribute{
				Required: true,
			},

			// might not be able to update for most connectors, check connector list
			"icon_url": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"visible": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},

			// might not be able to set for most connectors, check connector list
			"auth_method": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"auth_method_description": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"allow_assumed_signin": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},

			"created_at": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Computed: true,
			},

			// May be null, optional attribute
			"description": schema.StringAttribute{
				Optional: true,
			},
			"tab_id": schema.Int64Attribute{
				Optional: true,
			},
			"notes": schema.StringAttribute{
				Optional: true,
			},
			"policy_id": schema.Int64Attribute{
				Optional: true,
			},
			"brand_id": schema.Int64Attribute{
				Optional: true,
			},

			"provisioning_enabled": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},

			"sso": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"client_id": schema.StringAttribute{
						Computed: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"client_secret": schema.StringAttribute{
						Sensitive: true,
						Computed:  true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"metadata_url": schema.StringAttribute{
						Computed: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"acs_url": schema.StringAttribute{
						Computed: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"sls_url": schema.StringAttribute{
						Computed: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"issuer": schema.StringAttribute{
						Computed: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"wsfed_sso_url": schema.StringAttribute{
						Computed: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"certificate_id": schema.Int64Attribute{
						Computed: true,
						PlanModifiers: []planmodifier.Int64{
							int64planmodifier.UseStateForUnknown(),
						},
					},
					"certificate_value": schema.StringAttribute{
						Computed: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"certificate_name": schema.StringAttribute{
						Computed: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
				},
				Computed: true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
			},

			"configuration": schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.UseStateForUnknown(),
				},
			},
			"parameters": schema.MapNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.Int64Attribute{
							Computed: true,
							Optional: true,
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
						},
						"label": schema.StringAttribute{
							Computed: true,
							Optional: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"provisioned_entitlements": schema.BoolAttribute{
							Computed: true,
							Optional: true,
							PlanModifiers: []planmodifier.Bool{
								boolplanmodifier.UseStateForUnknown(),
							},
						},
						"skip_if_blank": schema.BoolAttribute{
							Computed: true,
							Optional: true,
							PlanModifiers: []planmodifier.Bool{
								boolplanmodifier.UseStateForUnknown(),
							},
						},

						"default_values": schema.StringAttribute{
							Optional: true,
						},
						"user_attribute_mappings": schema.StringAttribute{
							Optional: true,
						},
						"user_attribute_macros": schema.StringAttribute{
							Optional: true,
						},
						"attributes_transformations": schema.StringAttribute{
							Optional: true,
						},
						"values": schema.StringAttribute{
							Optional: true,
						},

						"include_in_saml_assertion": schema.BoolAttribute{
							Optional: true,
						},
					},
				},
				Optional: true,
				Computed: true,
			},
		},
	}
}

func (d *oneloginAppResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var state oneloginApp
	diags := req.Plan.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	app := state.toNativApp(ctx)
	var appResp oneloginNativeApp
	err := d.client.execRequest(&oneloginRequest{
		method:    methodPost,
		path:      pathApps,
		body:      app,
		respModel: &appResp,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating app",
			"Could not create app: "+err.Error(),
		)
		return
	}

	newState, diags := appResp.toState(ctx)
	if diags.HasError() {
		return
	}

	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (d *oneloginAppResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state oneloginApp
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	d.read(ctx, &state, &resp.State, &resp.Diagnostics)
}

func (d *oneloginAppResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state oneloginApp
	diags := req.Plan.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var appResp oneloginNativeApp
	err := d.client.execRequest(&oneloginRequest{
		method:    methodPut,
		path:      fmt.Sprintf("%s/%v", pathApps, state.ID.ValueInt64()),
		body:      state.toNativApp(ctx),
		respModel: &appResp,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating app",
			"Could not update app: "+err.Error(),
		)
		return
	}

	newState, diags := appResp.toState(ctx)
	if diags.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &newState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (d *oneloginAppResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state oneloginApp
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := d.client.execRequest(&oneloginRequest{
		method: methodDelete,
		path:   fmt.Sprintf("%s/%v", pathApps, state.ID.ValueInt64()),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting app",
			"Could not delete app with id="+strconv.Itoa(int(state.ID.ValueInt64()))+": "+err.Error(),
		)
		return
	}
}

func (d *oneloginAppResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.Atoi(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing ID for import app",
			"Could not parse ID "+req.ID+": "+err.Error(),
		)
		return
	}

	state := oneloginApp{
		ID: types.Int64Value(int64(id)),
	}

	d.read(ctx, &state, &resp.State, &resp.Diagnostics)
}

func (r *oneloginAppResource) read(ctx context.Context, state *oneloginApp, respState *tfsdk.State, d *diag.Diagnostics) {
	var app oneloginNativeApp

	id := state.ID.ValueInt64()

	err := r.client.execRequest(&oneloginRequest{
		method:    methodGet,
		path:      fmt.Sprintf("%s/%v", pathApps, id),
		respModel: &app,
	})
	if err != nil {
		d.AddError(
			"client error",
			fmt.Sprintf("Unable to read app %v, got error: %s", id, err),
		)
		return
	}

	newState, diags := app.toState(ctx)
	if diags.HasError() {
		return
	}

	// Update state
	diags = respState.Set(ctx, newState)
	d.Append(diags...)
}

func (state *oneloginApp) toNativApp(ctx context.Context) *oneloginNativeApp {
	app := &oneloginNativeApp{
		ID:                    state.ID.ValueInt64(),
		Name:                  state.Name.ValueString(),
		ConnectorID:           state.ConnectorID.ValueInt64(),
		IconURL:               state.IconURL.ValueString(),
		Visible:               state.Visible.ValueBool(),
		AuthMethod:            state.AuthMethod.ValueInt64(),
		AuthMethodDescription: state.AuthMethodDescription.ValueString(),
		AllowAssumedSignin:    state.AllowAssumedSignin.ValueBool(),
		CreatedAt:             state.CreatedAt.ValueString(),
		UpdatedAt:             state.UpdatedAt.ValueString(),
		Description:           state.Description.ValueStringPointer(),
		TabID:                 state.TabID.ValueInt64Pointer(),
		BrandID:               state.BrandID.ValueInt64Pointer(),
		Notes:                 state.Notes.ValueStringPointer(),
		PolicyID:              state.PolicyID.ValueInt64Pointer(),
	}

	if !state.ProvisioningEnabled.IsUnknown() && !state.ProvisioningEnabled.IsNull() {
		app.Provisioning = &oneloginNativeAppProvisioning{
			Enabled: state.ProvisioningEnabled.ValueBool(),
		}
	}

	if !state.SSO.IsUnknown() && !state.SSO.IsNull() {
		app.SSO = &oneloginNativeAppSSO{}
		for k, v := range state.SSO.Attributes() {
			switch k {
			case "client_id":
				app.SSO.ClientID = v.(types.String).ValueStringPointer()
			case "client_secret":
				app.SSO.ClientSecret = v.(types.String).ValueStringPointer()
			case "metadata_url":
				app.SSO.MetadataURL = v.(types.String).ValueStringPointer()
			case "acs_url":
				app.SSO.ACSURL = v.(types.String).ValueStringPointer()
			case "sls_url":
				app.SSO.SLSURL = v.(types.String).ValueStringPointer()
			case "issuer":
				app.SSO.Issuer = v.(types.String).ValueStringPointer()
			case "wsfed_sso_url":
				app.SSO.WSFED_SSO_URL = v.(types.String).ValueStringPointer()
			case "certificate_id":
				app.SSO.Certificate.CertificateID = v.(types.Int64).ValueInt64Pointer()
			case "certificate_value":
				app.SSO.Certificate.CertificateValue = v.(types.String).ValueStringPointer()
			case "certificate_name":
				app.SSO.Certificate.CertificateName = v.(types.String).ValueStringPointer()
			}
		}
	}

	if !state.Parameters.IsNull() && !state.Parameters.IsUnknown() {
		app.Parameters = map[string]oneloginNativeAppParameter{}
		params := map[string]oneloginAppParameter{}
		diags := state.Parameters.ElementsAs(ctx, &params, false)
		if diags.HasError() {
			panic(diags.Errors())
		}
		for k, v := range params {
			app.Parameters[k] = oneloginNativeAppParameter{
				ID:                        v.ID.ValueInt64(),
				Label:                     v.Label.ValueString(),
				ProvisionedEntitlements:   v.ProvisionedEntitlements.ValueBool(),
				SkipIfBlank:               v.SkipIfBlank.ValueBool(),
				DefaultValues:             v.DefaultValues.ValueStringPointer(),
				UserAttributeMappings:     v.UserAttributeMappings.ValueStringPointer(),
				UserAttributeMacros:       v.UserAttributeMacros.ValueStringPointer(),
				AttributesTransformations: v.AttributesTransformations.ValueStringPointer(),
				Values:                    v.Values.ValueStringPointer(),
				IncludeInSAMLAssertion:    v.IncludeInSamlAssertion.ValueBoolPointer(),
			}
		}
	}

	return app
}

func (app *oneloginNativeApp) toState(ctx context.Context) (*oneloginApp, diag.Diagnostics) {
	state := &oneloginApp{
		ID:                    types.Int64Value(app.ID),
		Name:                  types.StringValue(app.Name),
		ConnectorID:           types.Int64Value(app.ConnectorID),
		IconURL:               types.StringValue(app.IconURL),
		Visible:               types.BoolValue(app.Visible),
		AuthMethod:            types.Int64Value(app.AuthMethod),
		AuthMethodDescription: types.StringValue(app.AuthMethodDescription),
		AllowAssumedSignin:    types.BoolValue(app.AllowAssumedSignin),
		CreatedAt:             types.StringValue(app.CreatedAt),
		UpdatedAt:             types.StringValue(app.UpdatedAt),

		Description: types.StringPointerValue(app.Description),
		TabID:       types.Int64PointerValue(app.TabID),
		BrandID:     types.Int64PointerValue(app.BrandID),
		Notes:       types.StringPointerValue(app.Notes),
		PolicyID:    types.Int64PointerValue(app.PolicyID),
	}

	if app.Provisioning != nil {
		state.ProvisioningEnabled = types.BoolValue(app.Provisioning.Enabled)
	}

	diags := diag.Diagnostics{}
	if app.SSO == nil {
		state.SSO = types.ObjectNull(oneloginAppSSOTypes())
	} else {
		sso := oneloginAppSSO{
			ClientID:         types.StringPointerValue(app.SSO.ClientID),
			ClientSecret:     types.StringPointerValue(app.SSO.ClientSecret),
			MetadataURL:      types.StringPointerValue(app.SSO.MetadataURL),
			ACSURL:           types.StringPointerValue(app.SSO.ACSURL),
			SLSURL:           types.StringPointerValue(app.SSO.SLSURL),
			Issuer:           types.StringPointerValue(app.SSO.Issuer),
			WSFED_SSO_URL:    types.StringPointerValue(app.SSO.WSFED_SSO_URL),
			CertificateID:    types.Int64PointerValue(app.SSO.Certificate.CertificateID),
			CertificateValue: types.StringPointerValue(app.SSO.Certificate.CertificateValue),
			CertificateName:  types.StringPointerValue(app.SSO.Certificate.CertificateName),
		}
		tmp, diags := types.ObjectValueFrom(ctx, oneloginAppSSOTypes(), sso)
		if diags.HasError() {
			state.SSO = types.ObjectNull(oneloginAppSSOTypes())
		} else {
			state.SSO = tmp
		}
	}

	if app.Configuration == nil {
		state.Configuration = types.MapNull(types.StringType)
	} else {
		newConfig := map[string]interface{}{}
		for k, v := range app.Configuration {
			if v != nil {
				newConfig[k] = fmt.Sprintf("%v", v)
			}
		}

		tmp, diags := types.MapValueFrom(ctx, types.StringType, newConfig)
		if diags.HasError() {
			state.Configuration = types.MapNull(types.StringType)
		} else {
			state.Configuration = tmp
		}
	}

	if app.Parameters == nil {
		state.Parameters = types.MapNull(types.ObjectNull(oneloginAppParameterTypes()).Type(ctx))
	} else {
		diags := diag.Diagnostics{}
		tmpParams := map[string]attr.Value{}
		for k, v := range app.Parameters {
			param := oneloginAppParameter{
				ID:                        types.Int64Value(v.ID),
				Label:                     types.StringValue(v.Label),
				ProvisionedEntitlements:   types.BoolValue(v.ProvisionedEntitlements),
				SkipIfBlank:               types.BoolValue(v.SkipIfBlank),
				DefaultValues:             types.StringPointerValue(v.DefaultValues),
				UserAttributeMappings:     types.StringPointerValue(v.UserAttributeMappings),
				UserAttributeMacros:       types.StringPointerValue(v.UserAttributeMacros),
				AttributesTransformations: types.StringPointerValue(v.AttributesTransformations),
				Values:                    types.StringPointerValue(v.Values),
				IncludeInSamlAssertion:    types.BoolPointerValue(v.IncludeInSAMLAssertion),
			}

			tmp, newDiags := types.ObjectValueFrom(ctx, oneloginAppParameterTypes(), param)
			diags.Append(newDiags...)
			tmpParams[k] = tmp
		}
		if diags.HasError() {
			state.Parameters = types.MapNull(types.ObjectNull(oneloginAppParameterTypes()).Type(ctx))
		} else {
			state.Parameters, diags = types.MapValue(types.ObjectNull(oneloginAppParameterTypes()).Type(ctx), tmpParams)
		}

	}

	return state, diags
}

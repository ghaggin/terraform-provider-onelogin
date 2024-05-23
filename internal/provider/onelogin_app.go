package provider

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"github.com/ghaggin/terraform-provider-onelogin/onelogin"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/dynamicplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &oneloginAppResource{}
	_ resource.ResourceWithConfigure   = &oneloginAppResource{}
	_ resource.ResourceWithImportState = &oneloginAppResource{}
)

type oneloginAppResource struct {
	client *onelogin.Client
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

	SSO           types.Object  `tfsdk:"sso"`
	Configuration types.Dynamic `tfsdk:"configuration"`

	Parameters types.Map `tfsdk:"parameters"`
}

type oneloginAppParameter struct {
	ID                      types.Int64  `tfsdk:"id"`
	Label                   types.String `tfsdk:"label"`
	ProvisionedEntitlements types.Bool   `tfsdk:"provisioned_entitlements"`
	SkipIfBlank             types.Bool   `tfsdk:"skip_if_blank"`

	// DefaultValues             types.String `tfsdk:"default_values"`
	UserAttributeMappings     types.String `tfsdk:"user_attribute_mappings"`
	UserAttributeMacros       types.String `tfsdk:"user_attribute_macros"`
	AttributesTransformations types.String `tfsdk:"attributes_transformations"`
	Values                    types.String `tfsdk:"values"`
	IncludeInSamlAssertion    types.Bool   `tfsdk:"include_in_saml_assertion"`
}

func oneloginAppParameterTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":                       types.Int64Type,
		"label":                    types.StringType,
		"provisioned_entitlements": types.BoolType,
		"skip_if_blank":            types.BoolType,
		// "default_values":             types.StringType,
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

func NewOneLoginAppResource(client *onelogin.Client) newResourceFunc {
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

			"configuration": schema.DynamicAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Dynamic{
					dynamicplanmodifier.UseStateForUnknown(),
				},
				// TODO: describe all the intricacies of app configurations
				Description:         "see documentation for specific values",
				MarkdownDescription: onelogin.ConfigurationMarkdownDescription,
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

						// "default_values": schema.StringAttribute{
						// 	Optional: true,
						// },
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

	app, diags := state.toNativApp(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var appResp onelogin.Application
	err := d.client.ExecRequest(&onelogin.Request{
		Context:   ctx,
		Method:    onelogin.MethodPost,
		Path:      onelogin.PathApps,
		Body:      app,
		RespModel: &appResp,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating app",
			"Could not create app: "+err.Error(),
		)
		return
	}

	newState, diags := appToState(ctx, &appResp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
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

	nativeApp, diags := state.toNativApp(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var appResp onelogin.Application
	err := d.client.ExecRequest(&onelogin.Request{
		Context:   ctx,
		Method:    onelogin.MethodPut,
		Path:      fmt.Sprintf("%s/%v", onelogin.PathApps, state.ID.ValueInt64()),
		Body:      nativeApp,
		RespModel: &appResp,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating app",
			"Could not update app: "+err.Error(),
		)
		return
	}

	newState, diags := appToState(ctx, &appResp)
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

	err := d.client.ExecRequest(&onelogin.Request{
		Context: ctx,
		Method:  onelogin.MethodDelete,
		Path:    fmt.Sprintf("%s/%v", onelogin.PathApps, state.ID.ValueInt64()),
	})

	// consider NotFound a success
	if err == onelogin.ErrNotFound {
		tflog.Warn(ctx, "app to delete not found", map[string]interface{}{
			"name": state.Name.ValueString(),
			"id":   state.ID.ValueInt64(),
		})
		return
	}

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
	var app onelogin.Application

	id := state.ID.ValueInt64()

	err := r.client.ExecRequest(&onelogin.Request{
		Context:   ctx,
		Method:    onelogin.MethodGet,
		Path:      fmt.Sprintf("%s/%v", onelogin.PathApps, id),
		RespModel: &app,
	})
	if err != nil {
		d.AddError(
			"client error",
			fmt.Sprintf("Unable to read app %v, got error: %s", id, err),
		)
		return
	}

	// Sanitize the name
	// Onelogin returns some characters encoded for xml
	app.Name = regexp.MustCompile(`&amp;`).ReplaceAllString(app.Name, "&")
	app.Name = regexp.MustCompile(`&#39;`).ReplaceAllString(app.Name, "'")

	newState, diags := appToState(ctx, &app)
	d.Append(diags...)
	if d.HasError() {
		return
	}

	// Update state
	diags = respState.Set(ctx, newState)
	d.Append(diags...)
}

func (state *oneloginApp) toNativApp(ctx context.Context) (*onelogin.Application, diag.Diagnostics) {
	app := &onelogin.Application{
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
		app.Provisioning = &onelogin.ApplicationProvisioning{
			Enabled: state.ProvisioningEnabled.ValueBool(),
		}
	}

	if !state.SSO.IsUnknown() && !state.SSO.IsNull() {
		app.SSO = &onelogin.ApplicationSSO{}
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
		app.Parameters = map[string]onelogin.ApplicationParameter{}
		params := map[string]oneloginAppParameter{}
		diags := state.Parameters.ElementsAs(ctx, &params, false)
		if diags.HasError() {
			panic(diags.Errors())
		}
		for k, v := range params {
			app.Parameters[k] = onelogin.ApplicationParameter{
				ID:                      v.ID.ValueInt64(),
				Label:                   v.Label.ValueString(),
				ProvisionedEntitlements: v.ProvisionedEntitlements.ValueBool(),
				SkipIfBlank:             v.SkipIfBlank.ValueBool(),
				// DefaultValues:             v.DefaultValues.ValueStringPointer(),
				UserAttributeMappings:     v.UserAttributeMappings.ValueStringPointer(),
				UserAttributeMacros:       v.UserAttributeMacros.ValueStringPointer(),
				AttributesTransformations: v.AttributesTransformations.ValueStringPointer(),
				Values:                    v.Values.ValueStringPointer(),
				IncludeInSAMLAssertion:    v.IncludeInSamlAssertion.ValueBoolPointer(),
			}
		}
	}

	diags := diag.Diagnostics{}
	if !state.Configuration.IsNull() && !state.Configuration.IsUnknown() {
		switch value := state.Configuration.UnderlyingValue().(type) {
		case types.Object:
			app.Configuration = map[string]interface{}{}
			for k, v := range value.Attributes() {
				switch vtyped := v.(type) {
				case types.String:
					app.Configuration[k] = vtyped.ValueString()
				case types.Int64:
					app.Configuration[k] = vtyped.ValueInt64()
				}
			}
		default:
			diags.AddError("unknown type for configuration block", "should be object")
			return nil, diags
		}
	}

	return app, nil
}

func appToState(ctx context.Context, app *onelogin.Application) (*oneloginApp, diag.Diagnostics) {
	diags := diag.Diagnostics{}

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
		tmp, newDiags := types.ObjectValueFrom(ctx, oneloginAppSSOTypes(), sso)
		diags.Append(newDiags...)
		if diags.HasError() {
			return nil, diags
		}
		state.SSO = tmp
	}

	if app.Configuration == nil {
		state.Configuration = types.DynamicNull()
	} else {
		objecttypes, objectvalues, err := getTypesAndValuesForConnector(app.ConnectorID, app.Configuration)
		if err != nil {
			diags.AddError(err.Error(), fmt.Sprintf("app_name: %s\t\tapp_id: %d\t\tconnector_id: %d", app.Name, app.ID, app.ConnectorID))
			return nil, diags
		}
		tmp, newDiags := types.ObjectValue(objecttypes, objectvalues)
		diags.Append(newDiags...)
		if diags.HasError() {
			return nil, diags
		}
		state.Configuration = types.DynamicValue(tmp)
	}

	if app.Parameters == nil {
		state.Parameters = types.MapNull(types.ObjectNull(oneloginAppParameterTypes()).Type(ctx))
	} else {
		paramdiags := diag.Diagnostics{}
		tmpParams := map[string]attr.Value{}
		for k, v := range app.Parameters {
			param := oneloginAppParameter{
				ID:                      types.Int64Value(v.ID),
				Label:                   types.StringValue(v.Label),
				ProvisionedEntitlements: types.BoolValue(v.ProvisionedEntitlements),
				SkipIfBlank:             types.BoolValue(v.SkipIfBlank),
				// DefaultValues:             types.StringPointerValue(v.DefaultValues),
				UserAttributeMappings:     types.StringPointerValue(v.UserAttributeMappings),
				UserAttributeMacros:       types.StringPointerValue(v.UserAttributeMacros),
				AttributesTransformations: types.StringPointerValue(v.AttributesTransformations),
				Values:                    types.StringPointerValue(v.Values),
				IncludeInSamlAssertion:    types.BoolPointerValue(v.IncludeInSAMLAssertion),
			}

			tmp, newDiags := types.ObjectValueFrom(ctx, oneloginAppParameterTypes(), param)
			paramdiags.Append(newDiags...)
			tmpParams[k] = tmp
		}
		diags.Append(paramdiags...)
		if diags.HasError() {
			return nil, diags
		} else {
			tmp, newDiags := types.MapValue(types.ObjectNull(oneloginAppParameterTypes()).Type(ctx), tmpParams)
			diags.Append(newDiags...)
			if diags.HasError() {
				return nil, diags
			}
			state.Parameters = tmp
		}
	}

	return state, diags
}

func getTypesAndValuesForConnector(connectorID int64, m map[string]interface{}) (map[string]attr.Type, map[string]attr.Value, error) {
	var configtypes map[string]attr.Type
	switch connectorID {
	case 110016:
		// SAML Custom Connector (Advanced)
		configtypes = map[string]attr.Type{
			"audience":                      types.StringType,
			"certificate_id":                types.Int64Type,
			"consumer_url":                  types.StringType,
			"encrypt_assertion":             types.StringType,
			"generate_attribute_value_tags": types.StringType,
			"login":                         types.StringType,
			"logout_url":                    types.StringType,
			"recipient":                     types.StringType,
			"relaystate":                    types.StringType,
			"saml_encryption_method_id":     types.StringType,
			"saml_initiater_id":             types.StringType,
			"saml_issuer_type":              types.StringType,
			"saml_nameid_format_id":         types.StringType,
			"saml_nameid_format_id_slo":     types.StringType,
			"saml_notbefore":                types.StringType,
			"saml_notonorafter":             types.StringType,
			"saml_sessionnotonorafter":      types.StringType,
			"saml_sign_element":             types.StringType,
			"sign_slo_request":              types.StringType,
			"sign_slo_response":             types.StringType,
			"signature_algorithm":           types.StringType,
			"validator":                     types.StringType,
		}
	case 14571:
		// Shortcut
		configtypes = map[string]attr.Type{
			"url":                 types.StringType,
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
		}
	case 31697:
		// Salesforce Sandbox
		configtypes = map[string]attr.Type{
			"provisioning_version": types.StringType,
			"url":                  types.StringType,
			"subdomain":            types.StringType,
			"update_entitlements":  types.StringType,
			"signature_algorithm":  types.StringType,
			"certificate_id":       types.Int64Type,
		}
	case 42405:
		// SAML Test Connector (IdP)
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"validator":           types.StringType,
			"logout_url":          types.StringType,
			"relaystate":          types.StringType,
			"audience":            types.StringType,
			"recipient":           types.StringType,
			"signature_algorithm": types.StringType,
			"consumer_url":        types.StringType,
		}
	case 42657:
		// -not found-
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"consumer_url":        types.StringType,
			"signature_algorithm": types.StringType,
			"audience":            types.StringType,
			"validator":           types.StringType,
			"recipient":           types.StringType,
			"logout_url":          types.StringType,
			"relaystate":          types.StringType,
		}
	case 29255:
		// Salesforce
		configtypes = map[string]attr.Type{
			"provisioning_version": types.StringType,
			"signature_algorithm":  types.StringType,
			"url":                  types.StringType,
			"update_entitlements":  types.StringType,
			"certificate_id":       types.Int64Type,
		}
	case 43457:
		// -not found-
		configtypes = map[string]attr.Type{
			"logout_url":          types.StringType,
			"audience":            types.StringType,
			"signature_algorithm": types.StringType,
			"consumer_url":        types.StringType,
			"validator":           types.StringType,
			"recipient":           types.StringType,
			"relaystate":          types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 49734:
		// ServiceNow Multi Tenant
		configtypes = map[string]attr.Type{
			"login":               types.StringType,
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
			"url":                 types.StringType,
			"relaystate":          types.StringType,
		}
	case 65663:
		// -not found-
		configtypes = map[string]attr.Type{
			"relaystate":          types.StringType,
			"audience":            types.StringType,
			"certificate_id":      types.Int64Type,
			"consumer_url":        types.StringType,
			"validator":           types.StringType,
			"recipient":           types.StringType,
			"logout_url":          types.StringType,
			"signature_algorithm": types.StringType,
			"login_url":           types.StringType,
		}
	case 43753:
		// Wordpress
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
			"relaystate":          types.StringType,
			"consumer_url":        types.StringType,
			"slo":                 types.StringType,
		}
	case 56723:
		// Quicklink SP (GET)
		configtypes = map[string]attr.Type{
			"login_url":           types.StringType,
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
		}
	case 159123:
		// DocuSign Admin API (Prod)
		configtypes = map[string]attr.Type{
			"account_id":          types.StringType,
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
			"idpid":               types.StringType,
			"organization_id":     types.StringType,
		}
	case 4513:
		// Zendesk
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
			"url":                 types.StringType,
			"subdomain":           types.StringType,
		}
	case 45504:
		// SCIM Provisioner with SAML (SCIM v2 Core)
		configtypes = map[string]attr.Type{
			"audience":            types.StringType,
			"signature_algorithm": types.StringType,
			"consumer":            types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 130179:
		// SCIM Provisioner with SAML (SCIM v2 Enterprise)
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
			"consumer":            types.StringType,
			"audience":            types.StringType,
		}
	case 7170:
		// G Suite (Shared Accounts)
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"api_email":           types.StringType,
			"domain":              types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 150771:
		// DocuSign Admin API (Demo)
		configtypes = map[string]attr.Type{
			"account_id":          types.StringType,
			"certificate_id":      types.Int64Type,
			"organization_id":     types.StringType,
			"signature_algorithm": types.StringType,
			"idpid":               types.StringType,
		}
	case 140809:
		// -not found-
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"docusignEntityId":    types.StringType,
			"signature_algorithm": types.StringType,
			"login_url":           types.StringType,
		}
	case 49677:
		// Tableau Server(Signed Response)
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"prefix":              types.StringType,
			"audience":            types.StringType,
			"logout_url":          types.StringType,
			"server":              types.StringType,
			"signature_algorithm": types.StringType,
		}
	case 2885:
		// Workday
		configtypes = map[string]attr.Type{
			"audience":            types.StringType,
			"signature_algorithm": types.StringType,
			"url":                 types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 907:
		// G Suite
		configtypes = map[string]attr.Type{
			"certificate_id":         types.Int64Type,
			"signature_algorithm":    types.StringType,
			"api_email":              types.StringType,
			"domain":                 types.StringType,
			"provision_entitlements": types.StringType,
		}
	case 55873:
		// OpenDNS
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
		}
	case 15452:
		// SpringCM - UAT
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 133809:
		// Intercom
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
			"id":                  types.StringType,
		}
	case 68859:
		// -not found-
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"domain":              types.StringType,
			"Organization ID":     types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 156557:
		// SCIM Provisioner with SAML (SCIM v2 Enterprise, full SAML)
		configtypes = map[string]attr.Type{
			"login":                         types.StringType,
			"saml_notonorafter":             types.StringType,
			"encrypt_assertion":             types.StringType,
			"consumer_url":                  types.StringType,
			"validator":                     types.StringType,
			"saml_encryption_method_id":     types.StringType,
			"saml_notbefore":                types.StringType,
			"saml_issuer_type":              types.StringType,
			"sign_slo_request":              types.StringType,
			"sign_slo_response":             types.StringType,
			"signature_algorithm":           types.StringType,
			"saml_nameid_format_id_slo":     types.StringType,
			"audience":                      types.StringType,
			"recipient":                     types.StringType,
			"relaystate":                    types.StringType,
			"saml_sign_element":             types.StringType,
			"saml_sessionnotonorafter":      types.StringType,
			"generate_attribute_value_tags": types.StringType,
			"saml_initiater_id":             types.StringType,
			"logout_url":                    types.StringType,
			"saml_nameid_format_id":         types.StringType,
			"certificate_id":                types.Int64Type,
		}
	case 72003:
		// Frontify
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
			"domain":              types.StringType,
		}
	case 77999:
		// Slack
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"subdomain":           types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 105598:
		// SAML Custom Connector (SP Shibboleth)
		configtypes = map[string]attr.Type{
			"consumer_url":        types.StringType,
			"relay":               types.StringType,
			"recipient":           types.StringType,
			"login_url":           types.StringType,
			"validator":           types.StringType,
			"slo_url":             types.StringType,
			"audience":            types.StringType,
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 84355:
		// GitHub Cloud Organizations
		configtypes = map[string]attr.Type{
			"signature_algorithm":      types.StringType,
			"saml_sessionnotonorafter": types.StringType,
			"scim_base_url":            types.StringType,
			"certificate_id":           types.Int64Type,
			"org":                      types.StringType,
		}
	case 134485:
		// Adobe Creative Cloud (SP initiated SAML)
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"adobeid":             types.StringType,
			"signature_algorithm": types.StringType,
		}
	case 154612:
		// Oracle Fusion Gen2
		configtypes = map[string]attr.Type{
			"logout_url":                types.StringType,
			"consumer":                  types.StringType,
			"certificate_id":            types.Int64Type,
			"relaystate":                types.StringType,
			"loginURL":                  types.StringType,
			"encrypt_assertion":         types.StringType,
			"saml_encryption_method_id": types.StringType,
			"audience":                  types.StringType,
			"signature_algorithm":       types.StringType,
		}
	case 125281:
		// Egencia Direct
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"region":              types.StringType,
			"login_url":           types.StringType,
			"signature_algorithm": types.StringType,
		}
	case 11989:
		// SpringCM
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
			"scim_base_url":       types.StringType,
		}
	case 112879:
		// -not found-
		configtypes = map[string]attr.Type{
			"consumer_url":        types.StringType,
			"certificate_id":      types.Int64Type,
			"validator":           types.StringType,
			"audience":            types.StringType,
			"signature_algorithm": types.StringType,
			"slo_url":             types.StringType,
			"recipient":           types.StringType,
		}
	case 120559:
		// Figma
		configtypes = map[string]attr.Type{
			"tenantid":            types.StringType,
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 90941:
		// Formstack
		configtypes = map[string]attr.Type{
			"acs":                 types.StringType,
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
			"domain":              types.StringType,
		}
	case 65767:
		// Looker
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
			"subdomain":           types.StringType,
		}
	case 127174:
		// SentinelOne
		configtypes = map[string]attr.Type{
			"accountid":           types.StringType,
			"certificate_id":      types.Int64Type,
			"server":              types.StringType,
			"signature_algorithm": types.StringType,
		}
	case 130413:
		// AWS IAM Identity Center (AWS Single Sign-on)
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
			"consumer":            types.StringType,
			"audience":            types.StringType,
		}
	case 49886:
		// Artifactory
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"domain":              types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 60489:
		// HackerOne
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
		}
	case 11019:
		// Smartsheet
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
			"domain":              types.StringType,
		}
	case 37918:
		// -not found-
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
			"domain":              types.StringType,
		}
	case 121632:
		// AirWatch (Multi ACS URL support)
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"audience":            types.StringType,
			"consumer_url":        types.StringType,
			"signature_algorithm": types.StringType,
			"validator":           types.StringType,
		}
	case 70856:
		// -not found-
		configtypes = map[string]attr.Type{
			"validator":           types.StringType,
			"audience":            types.StringType,
			"relaystate":          types.StringType,
			"consumer_url":        types.StringType,
			"recipient":           types.StringType,
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
			"logout_url":          types.StringType,
		}
	case 2479:
		// -not found-
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"slo_url":             types.StringType,
			"certificate_id":      types.Int64Type,
			"audience":            types.StringType,
			"consumer_url":        types.StringType,
			"recipient":           types.StringType,
			"validator":           types.StringType,
		}
	case 95219:
		// -not found-
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"consumer":            types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 76671:
		// EHS Insight
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"subdomain":           types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 47292:
		// -not found-
		configtypes = map[string]attr.Type{
			"recipient":           types.StringType,
			"certificate_id":      types.Int64Type,
			"audience":            types.StringType,
			"signature_algorithm": types.StringType,
			"validator":           types.StringType,
			"login_url":           types.StringType,
			"consumer_url":        types.StringType,
			"slo_url":             types.StringType,
		}
	case 160658:
		// KnowBe4
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
			"consumer":            types.StringType,
			"audience":            types.StringType,
		}
	case 105466:
		// Lessonly
		configtypes = map[string]attr.Type{
			"subdomain":           types.StringType,
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
		}
	case 77901:
		// MobileIron
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"audience":            types.StringType,
			"signature_algorithm": types.StringType,
			"domain":              types.StringType,
		}
	case 741:
		// Google Mail
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
			"domain":              types.StringType,
		}
	case 4860:
		// -not found-
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 31802:
		// Tableau Server
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 71476:
		// Collective Health
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 127704:
		// Notion
		configtypes = map[string]attr.Type{
			"consumer":            types.StringType,
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 108990:
		// Asana
		configtypes = map[string]attr.Type{
			"certificate_id":           types.Int64Type,
			"saml_sessionnotonorafter": types.StringType,
			"signature_algorithm":      types.StringType,
		}
	case 70450:
		// DigiCert
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
		}
	case 121995:
		// SiRequest(SP Initiated)
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
		}
	case 77614:
		// -not found-
		configtypes = map[string]attr.Type{
			"login_url":           types.StringType,
			"signature_algorithm": types.StringType,
			"recipient":           types.StringType,
			"consumer_url":        types.StringType,
			"certificate_id":      types.Int64Type,
			"validator":           types.StringType,
			"audience":            types.StringType,
			"slo_url":             types.StringType,
		}
	case 114048:
		// Procore
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 28712:
		// Google Drive
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
			"domain":              types.StringType,
		}
	case 82579:
		// Periscope Data
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 124615:
		// Appspace Cloud
		configtypes = map[string]attr.Type{
			"account_id":          types.StringType,
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 50159:
		// -not found-
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
		}
	case 126186:
		// Mixpanel
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
			"postback_url":        types.StringType,
		}
	case 99186:
		// Memsource
		configtypes = map[string]attr.Type{
			"ORG_ID":              types.StringType,
			"signature_algorithm": types.StringType,
			"domain":              types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 154781:
		// Uber
		configtypes = map[string]attr.Type{
			"org_id":              types.StringType,
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 106670:
		// SimpleLegal
		configtypes = map[string]attr.Type{
			"domain":              types.StringType,
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
		}
	case 152501:
		// MeetingSelect
		configtypes = map[string]attr.Type{
			"subdomain":           types.StringType,
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 95426:
		// RFPIO
		configtypes = map[string]attr.Type{
			"relaystate":          types.StringType,
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 149688:
		// Workplace by Facebook Provisioning
		configtypes = map[string]attr.Type{
			"consumer":             types.StringType,
			"recipient":            types.StringType,
			"audience":             types.StringType,
			"signature_algorithm":  types.StringType,
			"webhook_verify_token": types.StringType,
			"certificate_id":       types.Int64Type,
		}
	case 40314:
		// Google Calendar
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"domain":              types.StringType,
			"signature_algorithm": types.StringType,
		}
	case 40570:
		// Absorb LMS
		configtypes = map[string]attr.Type{
			"consumer_url":        types.StringType,
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
			"relaystate":          types.StringType,
			"recipient":           types.StringType,
		}
	case 30118:
		// -not found-
		configtypes = map[string]attr.Type{
			"url":                 types.StringType,
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 122063:
		// Beamery Grow
		configtypes = map[string]attr.Type{
			"connectionname":      types.StringType,
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 148084:
		// Segment
		configtypes = map[string]attr.Type{
			"acs":                 types.StringType,
			"audience":            types.StringType,
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 47441:
		// Sprinklr
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"number":              types.StringType,
			"subdomain":           types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 95071:
		// Onetrust
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 56217:
		// Degreed
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
		}
	case 30117:
		// -not found-
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
			"url":                 types.StringType,
		}
	case 45714:
		// Github Enterprise Server
		configtypes = map[string]attr.Type{
			"signature_algorithm":      types.StringType,
			"domain":                   types.StringType,
			"saml_sessionnotonorafter": types.StringType,
			"certificate_id":           types.Int64Type,
		}
	case 26753:
		// Zoom
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
			"subdomain":           types.StringType,
		}
	case 65151:
		// GitLab (Self-managed)
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"domain":              types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 77106:
		// Slido
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
			"login":               types.StringType,
		}
	case 25034:
		// ExactTarget (Salesforce Marketing Cloud)(deprecated)
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"relay":               types.StringType,
			"signature_algorithm": types.StringType,
		}
	case 94803:
		// -not found-
		configtypes = map[string]attr.Type{
			"consumer_url":        types.StringType,
			"relaystate":          types.StringType,
			"certificate_id":      types.Int64Type,
			"recipient":           types.StringType,
			"validator":           types.StringType,
			"logout_url":          types.StringType,
			"audience":            types.StringType,
			"signature_algorithm": types.StringType,
		}
	case 95886:
		// -not found-
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"company":             types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 94329:
		// Valimail
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 30005:
		// -not found-
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
			"url":                 types.StringType,
		}
	case 38071:
		// ThousandEyes
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
			"audience":            types.StringType,
			"consumer":            types.StringType,
		}
	case 133783:
		// BrowserStack SSO
		configtypes = map[string]attr.Type{
			"acs":                 types.StringType,
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
		}
	case 42995:
		// -not found-
		configtypes = map[string]attr.Type{
			"accountid":           types.StringType,
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 142151:
		// Autodesk SSO
		configtypes = map[string]attr.Type{
			"audience":            types.StringType,
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
			"customerID":          types.StringType,
		}
	case 9772:
		// Marketo
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
			"munchkin_account_id": types.StringType,
		}
	case 96712:
		// Sentry
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"slug":                types.StringType,
			"signature_algorithm": types.StringType,
		}
	case 162077:
		// Own{backup}
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"region":              types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 116169:
		// Datadog
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
			"shard":               types.StringType,
			"login_url":           types.StringType,
		}
	case 30002:
		// -not found-
		configtypes = map[string]attr.Type{
			"url":                 types.StringType,
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
		}
	case 16066:
		// New Relic by Account
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"account_id":          types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 158861:
		// Shortcut
		configtypes = map[string]attr.Type{
			"org":                 types.StringType,
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 110542:
		// -not found-
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
			"subdomain":           types.StringType,
			"relay":               types.StringType,
		}
	case 107446:
		// Tenable.io
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
			"domain":              types.StringType,
		}
	case 30003:
		// -not found-
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"url":                 types.StringType,
			"signature_algorithm": types.StringType,
		}
	case 106368:
		// Oracle Identity Cloud Service
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"slo":                 types.StringType,
			"acs":                 types.StringType,
			"certificate_id":      types.Int64Type,
			"provider":            types.StringType,
		}
	case 121781:
		// SIRequest QA(SP initiated)
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
		}
	case 90344:
		// Relativity
		configtypes = map[string]attr.Type{
			"consumer_url":        types.StringType,
			"validator":           types.StringType,
			"certificate_id":      types.Int64Type,
			"audience":            types.StringType,
			"signature_algorithm": types.StringType,
		}
	case 46171:
		// Meraki
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"consumer":            types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 127691:
		// CodeSignal
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
		}
	case 82403:
		// SiQ (formerly SpaceIQ)
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
			"consumer":            types.StringType,
			"audience":            types.StringType,
		}
	case 84822:
		// Zscaler Admin
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"cloudname":           types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 165862:
		//  Zscaler ZDX
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 76260:
		// -not found-
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
			"account":             types.StringType,
		}
	case 53161:
		// Expensify
		configtypes = map[string]attr.Type{
			"domain":              types.StringType,
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 137483:
		// Airbnb for Work
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
			"companyid":           types.StringType,
		}
	case 88596:
		// AlertMedia
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"domain":              types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 2801:
		// Box
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
			"alias":               types.StringType,
		}
	case 48193:
		// Shareworks Employee
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"relay":               types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 140003:
		// Ally.io
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
			"uuid":                types.StringType,
		}
	case 163938:
		// SCIM Provisioner with SAML (SCIM v2 Enterprise, SCIM2 PATCH for Groups)
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
			"consumer":            types.StringType,
			"audience":            types.StringType,
		}
	case 115252:
		// ZScaler (All Tenants)
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"tenant":              types.StringType,
			"signature_algorithm": types.StringType,
			"relay":               types.StringType,
		}
	case 30004:
		// -not found-
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
			"url":                 types.StringType,
		}
	case 30001:
		// -not found-
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"url":                 types.StringType,
			"signature_algorithm": types.StringType,
		}
	case 89335:
		// -not found-
		configtypes = map[string]attr.Type{
			"account":             types.StringType,
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
			"audience":            types.StringType,
		}
	case 136206:
		// iboss User SSO
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"domain":              types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 121386:
		// SCIM Provisioner w/SAML (SCIM v2 w/OAuth)
		configtypes = map[string]attr.Type{
			"custom_headers":      types.StringType,
			"consumer":            types.StringType,
			"scim_base_url":       types.StringType,
			"auth_url":            types.StringType,
			"site":                types.StringType,
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
			"token_uri":           types.StringType,
			"audience":            types.StringType,
		}
	case 95668:
		// Buildkite
		configtypes = map[string]attr.Type{
			"domain":              types.StringType,
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
		}
	case 55785:
		// Splunk
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"audience":            types.StringType,
			"signature_algorithm": types.StringType,
			"logout_url":          types.StringType,
			"consumer_url":        types.StringType,
		}
	case 42338:
		// Coupa
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
			"domain":              types.StringType,
			"url":                 types.StringType,
		}
	case 78887:
		// Amazon Web Services (AWS) Multi Account
		configtypes = map[string]attr.Type{
			"external_role":       types.StringType,
			"certificate_id":      types.Int64Type,
			"external_id":         types.StringType,
			"signature_algorithm": types.StringType,
			"idp_list":            types.StringType,
		}
	case 107404:
		// TextExpander
		configtypes = map[string]attr.Type{
			"domain":              types.StringType,
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 154478:
		// Lucid
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
		}
	case 56178:
		// SurveyMonkey
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
		}
	case 64290:
		// Thomsons Online Benefits
		configtypes = map[string]attr.Type{
			"company_id":          types.StringType,
			"subdomain":           types.StringType,
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
		}
	case 37247:
		// -not found-
		configtypes = map[string]attr.Type{
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
			"accountid":           types.StringType,
		}
	case 126029:
		// Mapbox
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
		}
	case 85731:
		// Amplitude
		configtypes = map[string]attr.Type{
			"orgid":               types.StringType,
			"signature_algorithm": types.StringType,
			"certificate_id":      types.Int64Type,
		}
	case 76209:
		// Heroku
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"OrgID":               types.StringType,
			"signature_algorithm": types.StringType,
		}
	case 95842:
		// Oracle Planning &amp; Budgeting
		configtypes = map[string]attr.Type{
			"environment":         types.StringType,
			"signature_algorithm": types.StringType,
			"data_center":         types.StringType,
			"certificate_id":      types.Int64Type,
			"identity_domain":     types.StringType,
		}
	case 141102:
		// Tableau Online SSO
		configtypes = map[string]attr.Type{
			"audience":            types.StringType,
			"consumer":            types.StringType,
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
		}
	case 50323:
		// Uber Bon Appétit Staging
		configtypes = map[string]attr.Type{
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
		}
	case 131770:
		// NS1
		configtypes = map[string]attr.Type{
			"sso_id":              types.StringType,
			"certificate_id":      types.Int64Type,
			"signature_algorithm": types.StringType,
		}
	default:
		configtypes = map[string]attr.Type{}
		for k, v := range m {
			if v == nil {
				continue
			}

			switch v.(type) {
			case float64:
				configtypes[k] = types.Int64Type
			case string:
				configtypes[k] = types.StringType
			default:
				return nil, nil, fmt.Errorf("unrecognized type for: %v", v)
			}
		}
	}

	configvalues := map[string]attr.Value{}
	for k, t := range configtypes {
		if mapvalue, ok := m[k]; ok {
			switch t {
			case types.StringType:
				if mapvalue == nil {
					delete(configtypes, k)
				} else {
					configvalues[k] = types.StringValue(mapvalue.(string))
				}
			case types.Int64Type:
				// json -> map converts ints to floats
				if mapvalue == nil {
					delete(configtypes, k)
				} else {
					configvalues[k] = types.Int64Value(int64(mapvalue.(float64)))
				}
			}
		}
	}

	return configtypes, configvalues, nil
}

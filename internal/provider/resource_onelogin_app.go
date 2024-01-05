package provider

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/ghaggin/terraform-provider-onelogin/internal/util"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/onelogin/onelogin-go-sdk/pkg/client"
	"github.com/onelogin/onelogin-go-sdk/pkg/services/apps"
)

var (
	_ resource.Resource                = &oneLoginApp{}
	_ resource.ResourceWithConfigure   = &oneLoginApp{}
	_ resource.ResourceWithImportState = &oneLoginApp{}
)

type oneLoginApp struct {
	client *client.APIClient
}

type oneLoginAppModel struct {
	ID          types.Int64  `tfsdk:"id"`
	ConnectorID types.Int64  `tfsdk:"connector_id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Notes       types.String `tfsdk:"notes"`
	PolicyID    types.Int64  `tfsdk:"policy_id"`
	BrandID     types.Int64  `tfsdk:"brand_id"`
	IconURL     types.String `tfsdk:"icon_url"`
	Visible     types.Bool   `tfsdk:"visible"`
	AuthMethod  types.Int64  `tfsdk:"auth_method"`
	TabID       types.Int64  `tfsdk:"tab_id"`

	// Sring values of time in format
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`

	// Warning: Including role_ids here creates bad interplay
	// with the the role resource.
	//
	// RoleIDs            types.List                     `tfsdk:"role_ids"`

	AllowAssumedSignin types.Bool                     `tfsdk:"allow_assumed_signin"`
	Provisioning       *oneLoginAppProvisioningModel  `tfsdk:"provisioning"`
	SSO                types.Object                   `tfsdk:"sso"`
	Configuration      *oneLoginAppConfigurationModel `tfsdk:"configuration"`
	Parameters         types.Map                      `tfsdk:"parameters"`

	// Note: attribute local to terraform objects
	LastUpdated types.String `tfsdk:"last_updated"`
}

type oneLoginAppProvisioningModel struct {
	Enabled types.Bool `tfsdk:"enabled"`
}

type oneLoginAppConfigurationModel struct {
	RedirectURI                   types.String `tfsdk:"redirect_uri"`
	RefreshTokenExpirationMinutes types.Int64  `tfsdk:"refresh_token_expiration_minutes"`
	LoginURL                      types.String `tfsdk:"login_url"`
	OidcApplicationType           types.Int64  `tfsdk:"oidc_application_type"`
	TokenEndpointAuthMethod       types.Int64  `tfsdk:"token_endpoint_auth_method"`
	AccessTokenExpirationMinutes  types.Int64  `tfsdk:"access_token_expiration_minutes"`
	ProviderArn                   types.String `tfsdk:"provider_arn"`
	IdpList                       types.String `tfsdk:"idp_list"`
	SignatureAlgorithm            types.String `tfsdk:"signature_algorithm"`
	LogoutURL                     types.String `tfsdk:"logout_url"`
	PostLogoutRedirectURI         types.String `tfsdk:"post_logout_redirect_uri"`
	Audience                      types.String `tfsdk:"audience"`
	ConsumerURL                   types.String `tfsdk:"consumer_url"`
	Login                         types.String `tfsdk:"login"`
	Recipient                     types.String `tfsdk:"recipient"`
	Validator                     types.String `tfsdk:"validator"`
	RelayState                    types.String `tfsdk:"relaystate"`
	Relay                         types.String `tfsdk:"relay"`
	SAMLNotValidOnOrAafter        types.String `tfsdk:"saml_notonorafter"`
	GenerateAttributeValueTags    types.String `tfsdk:"generate_attribute_value_tags"`
	SAMLInitiaterID               types.String `tfsdk:"saml_initiater_id"`
	SAMLNotValidBefore            types.String `tfsdk:"saml_notbefore"`
	SAMLIssuerType                types.String `tfsdk:"saml_issuer_type"`
	SAMLSignElement               types.String `tfsdk:"saml_sign_element"`
	EncryptAssertion              types.String `tfsdk:"encrypt_assertion"`
	SAMLSessionNotValidOnOrAfter  types.String `tfsdk:"saml_sessionnotonorafter"`
	SAMLEncryptionMethodID        types.String `tfsdk:"saml_encryption_method_id"`
	SAMLNameIDFormatID            types.String `tfsdk:"saml_nameid_format_id"`
}

type oneLoginAppParameterModel struct {
	ID                        types.Int64  `tfsdk:"id"`
	Label                     types.String `tfsdk:"label"`
	UserAttributeMappings     types.String `tfsdk:"user_attribute_mappings"`
	UserAttributeMacros       types.String `tfsdk:"user_attribute_macros"`
	AttributesTransformations types.String `tfsdk:"attributes_transformations"`
	SkipIfBlank               types.Bool   `tfsdk:"skip_if_blank"`
	Values                    types.String `tfsdk:"values"`
	DefaultValues             types.String `tfsdk:"default_values"`
	ParamKeyName              types.String `tfsdk:"param_key_name"`
	ProvisionedEntitlements   types.Bool   `tfsdk:"provisioned_entitlements"`
	SafeEntitlementsEnabled   types.Bool   `tfsdk:"safe_entitlements_enabled"`
	IncludeInSamlAssertion    types.Bool   `tfsdk:"include_in_saml_assertion"`
}

// SSO types
type oneLoginAppSSOModel struct {
	ClientID         types.String `tfsdk:"client_id"`
	ClientSecret     types.String `tfsdk:"client_secret"`
	MetadataURL      types.String `tfsdk:"metadata_url"`
	ACSURL           types.String `tfsdk:"acs_url"`
	SLSURL           types.String `tfsdk:"sls_url"`
	IssuerURL        types.String `tfsdk:"issuer_url"`
	CertificateID    types.Int64  `tfsdk:"certificate_id"`
	CertificateValue types.String `tfsdk:"certificate_value"`
	CertificateName  types.String `tfsdk:"certificate_name"`
}

const (
	ssoClientIDKey         = "client_id"
	ssoCliendSecretKey     = "client_secret"
	ssoMetadataURLKey      = "metadata_url"
	ssoAcsURLKey           = "acs_url"
	ssoSlsURLKey           = "sls_url"
	ssoIssuerURLKey        = "issuer_url"
	ssoCertificateIDKey    = "certificate_id"
	ssoCertificateValueKey = "certificate_value"
	ssoCertificateNameKey  = "certificate_name"
)

func (d *oneLoginApp) ssoAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		ssoClientIDKey:         types.StringType,
		ssoCliendSecretKey:     types.StringType,
		ssoMetadataURLKey:      types.StringType,
		ssoAcsURLKey:           types.StringType,
		ssoSlsURLKey:           types.StringType,
		ssoIssuerURLKey:        types.StringType,
		ssoCertificateIDKey:    types.Int64Type,
		ssoCertificateValueKey: types.StringType,
		ssoCertificateNameKey:  types.StringType,
	}
}

// SSO types [end]

func NewOneLoginApp() resource.Resource {
	return &oneLoginApp{}
}

func (d *oneLoginApp) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app"
}

func (d *oneLoginApp) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.APIClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.APIClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *oneLoginApp) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"connector_id": schema.Int64Attribute{
				Required: true,
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			"description": schema.StringAttribute{
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
			"icon_url": schema.StringAttribute{
				Optional: true,
			},
			"visible": schema.BoolAttribute{
				Optional: true,
			},
			"auth_method": schema.Int64Attribute{
				Optional: true,
			},
			"tab_id": schema.Int64Attribute{
				Optional: true,
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

			// See warning on model
			//
			// "role_ids": schema.ListAttribute{
			// 	ElementType: types.Int64Type,
			// 	Optional:    true,
			// 	Computed:    true,
			// },

			"allow_assumed_signin": schema.BoolAttribute{
				Optional: true,
			},
			"provisioning": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Optional: true,
					},
				},
				Optional: true,
			},
			"sso": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					ssoClientIDKey: schema.StringAttribute{
						Computed: true,
						Optional: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					ssoCliendSecretKey: schema.StringAttribute{
						Sensitive: true,
						Computed:  true,
						Optional:  true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					ssoMetadataURLKey: schema.StringAttribute{
						Computed: true,
						Optional: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					ssoAcsURLKey: schema.StringAttribute{
						Computed: true,
						Optional: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					ssoSlsURLKey: schema.StringAttribute{
						Computed: true,
						Optional: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					ssoIssuerURLKey: schema.StringAttribute{
						Computed: true,
						Optional: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					ssoCertificateIDKey: schema.Int64Attribute{
						Computed: true,
						Optional: true,
						PlanModifiers: []planmodifier.Int64{
							int64planmodifier.UseStateForUnknown(),
						},
					},
					ssoCertificateValueKey: schema.StringAttribute{
						Computed: true,
						Optional: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					ssoCertificateNameKey: schema.StringAttribute{
						Computed: true,
						Optional: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
				},
				Computed: true,
				Optional: true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
			},
			"configuration": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"redirect_uri": schema.StringAttribute{
						Optional: true,
					},
					"refresh_token_expiration_minutes": schema.Int64Attribute{
						Optional: true,
					},
					"login_url": schema.StringAttribute{
						Optional: true,
					},
					"oidc_application_type": schema.Int64Attribute{
						Optional: true,
					},
					"token_endpoint_auth_method": schema.Int64Attribute{
						Optional: true,
					},
					"access_token_expiration_minutes": schema.Int64Attribute{
						Optional: true,
					},
					"provider_arn": schema.StringAttribute{
						Optional: true,
					},
					"idp_list": schema.StringAttribute{
						Optional: true,
					},
					"signature_algorithm": schema.StringAttribute{
						Optional: true,
					},
					"logout_url": schema.StringAttribute{
						Optional: true,
					},
					"post_logout_redirect_uri": schema.StringAttribute{
						Optional: true,
					},
					"audience": schema.StringAttribute{
						Optional: true,
					},
					"consumer_url": schema.StringAttribute{
						Optional: true,
					},
					"login": schema.StringAttribute{
						Optional: true,
					},
					"recipient": schema.StringAttribute{
						Optional: true,
					},
					"validator": schema.StringAttribute{
						Optional: true,
					},
					"relaystate": schema.StringAttribute{
						Optional: true,
					},
					"relay": schema.StringAttribute{
						Optional: true,
					},
					"saml_notonorafter": schema.StringAttribute{
						Optional: true,
					},
					"generate_attribute_value_tags": schema.StringAttribute{
						Optional: true,
					},
					"saml_initiater_id": schema.StringAttribute{
						Optional: true,
					},
					"saml_notbefore": schema.StringAttribute{
						Optional: true,
					},
					"saml_issuer_type": schema.StringAttribute{
						Optional: true,
					},
					"saml_sign_element": schema.StringAttribute{
						Optional: true,
					},
					"encrypt_assertion": schema.StringAttribute{
						Optional: true,
					},
					"saml_sessionnotonorafter": schema.StringAttribute{
						Optional: true,
					},
					"saml_encryption_method_id": schema.StringAttribute{
						Optional: true,
					},
					"saml_nameid_format_id": schema.StringAttribute{
						Optional: true,
					},
				},
				Optional: true,
			},
			"parameters": d.parametersSchema(),

			// Note: attribute local to terraform objects
			"last_updated": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (d *oneLoginApp) parametersSchema() schema.MapNestedAttribute {
	return schema.MapNestedAttribute{
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"id": schema.Int64Attribute{
					Computed: true,
					PlanModifiers: []planmodifier.Int64{
						int64planmodifier.UseStateForUnknown(),
					},
				},
				"label": schema.StringAttribute{
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
				"skip_if_blank": schema.BoolAttribute{
					Optional: true,
				},
				"values": schema.StringAttribute{
					Optional: true,
				},
				"default_values": schema.StringAttribute{
					Optional: true,
				},
				"param_key_name": schema.StringAttribute{
					Optional: true,
				},
				"provisioned_entitlements": schema.BoolAttribute{
					Optional: true,
				},
				"safe_entitlements_enabled": schema.BoolAttribute{
					Optional: true,
				},
				"include_in_saml_assertion": schema.BoolAttribute{
					Optional: true,
				},
			},
		},
		Optional: true,
	}
}

func (d *oneLoginApp) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var state oneLoginAppModel
	diags := req.Plan.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	app := d.appFromState(ctx, &state)
	err := d.client.Services.AppsV2.Create(app)
	if err != nil || app.ID == nil {
		resp.Diagnostics.AddError(
			"Error creating app",
			"Could not create app: "+err.Error(),
		)
		return
	}

	newState, diags := d.read(ctx, int64(*app.ID))
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState.LastUpdated = types.StringValue(util.GetTimestampString())

	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (d *oneLoginApp) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state oneLoginAppModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState, diags := d.read(ctx, state.ID.ValueInt64())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState.LastUpdated = state.LastUpdated

	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (d *oneLoginApp) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state oneLoginAppModel
	diags := req.Plan.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	app := d.appFromState(ctx, &state)
	_, err := d.client.Services.AppsV2.Update(app)
	if err != nil || app.ID == nil {
		resp.Diagnostics.AddError(
			"Error updating app",
			"Could not update app: "+err.Error(),
		)
		return
	}

	newState, diags := d.read(ctx, int64(*app.ID))
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState.LastUpdated = types.StringValue(util.GetTimestampString())

	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (d *oneLoginApp) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state oneLoginAppModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := d.client.Services.AppsV2.Destroy(int32(state.ID.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting app",
			"Could not delete app with id="+strconv.Itoa(int(state.ID.ValueInt64()))+": "+err.Error(),
		)
		return
	}
}

func (d *oneLoginApp) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.Atoi(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing ID for import app",
			"Could not parse ID "+req.ID+": "+err.Error(),
		)
		return
	}

	state, diags := d.read(ctx, int64(id))
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (d *oneLoginApp) read(ctx context.Context, id int64) (*oneLoginAppModel, diag.Diagnostics) {
	diags := diag.Diagnostics{}

	app, err := d.client.Services.AppsV2.GetOne(int32(id))
	if err != nil {
		diags.AddError(
			"Error reading app",
			"Could not read app with id="+strconv.Itoa(int(id))+": "+err.Error(),
		)
		return nil, diags
	}

	if app.ID == nil {
		diags.AddError(
			"Error reading app",
			"Could not read app with id="+strconv.Itoa(int(id))+": app not found",
		)
		return nil, diags
	}

	return d.appToState(ctx, app)
}

func (d *oneLoginApp) appToState(ctx context.Context, app *apps.App) (*oneLoginAppModel, diag.Diagnostics) {
	state := &oneLoginAppModel{}

	if app.ID != nil {
		state.ID = types.Int64Value(int64(*app.ID))
	}
	if app.ConnectorID != nil {
		state.ConnectorID = types.Int64Value(int64(*app.ConnectorID))
	}
	if app.Name != nil {
		state.Name = types.StringValue(*app.Name)
	}
	if app.Description != nil {
		state.Description = types.StringValue(*app.Description)
	}
	if app.Notes != nil {
		state.Notes = types.StringValue(*app.Notes)
	}
	if app.PolicyID != nil {
		state.PolicyID = types.Int64Value(int64(*app.PolicyID))
	}
	if app.BrandID != nil {
		state.BrandID = types.Int64Value(int64(*app.BrandID))
	}
	if app.IconURL != nil {
		state.IconURL = types.StringValue(*app.IconURL)
	}
	if app.Visible != nil {
		state.Visible = types.BoolValue(*app.Visible)
	}
	if app.AuthMethod != nil {
		state.AuthMethod = types.Int64Value(int64(*app.AuthMethod))
	}
	if app.TabID != nil {
		state.TabID = types.Int64Value(int64(*app.TabID))
	}
	if app.CreatedAt != nil {
		state.CreatedAt = types.StringValue(app.CreatedAt.UTC().Format(time.RFC3339))
	}
	if app.UpdatedAt != nil {
		state.UpdatedAt = types.StringValue(app.UpdatedAt.UTC().Format(time.RFC3339))
	}

	diags := diag.Diagnostics{}
	var newDiags diag.Diagnostics

	// See warning on model
	//
	// roleIDs, newDiags := types.ListValueFrom(ctx, types.Int64Type, app.RoleIDs)
	// diags.Append(newDiags...)
	// if len(roleIDs.Elements()) > 0 {
	// 	state.RoleIDs = roleIDs
	// } else {
	// 	state.RoleIDs = types.ListNull(types.Int64Type)
	// }

	if app.AllowAssumedSignin != nil {
		state.AllowAssumedSignin = types.BoolValue(*app.AllowAssumedSignin)
	}

	if app.Provisioning != nil {
		state.Provisioning = &oneLoginAppProvisioningModel{}
		if app.Provisioning.Enabled != nil {
			state.Provisioning.Enabled = types.BoolValue(*app.Provisioning.Enabled)
		}
	}

	if app.Sso != nil {
		sso := oneLoginAppSSOModel{}
		if app.Sso.ClientID != nil {
			sso.ClientID = types.StringValue(*app.Sso.ClientID)
		}
		if app.Sso.ClientSecret != nil {
			sso.ClientSecret = types.StringValue(*app.Sso.ClientSecret)
		}
		if app.Sso.MetadataURL != nil {
			sso.MetadataURL = types.StringValue(*app.Sso.MetadataURL)
		}
		if app.Sso.AcsURL != nil {
			sso.ACSURL = types.StringValue(*app.Sso.AcsURL)
		}
		if app.Sso.SlsURL != nil {
			sso.SLSURL = types.StringValue(*app.Sso.SlsURL)
		}
		if app.Sso.Issuer != nil {
			sso.IssuerURL = types.StringValue(*app.Sso.Issuer)
		}
		if app.Sso.Certificate != nil {
			if app.Sso.Certificate.ID != nil {
				sso.CertificateID = types.Int64Value(int64(*app.Sso.Certificate.ID))
			}
			if app.Sso.Certificate.Value != nil {
				sso.CertificateValue = types.StringValue(*app.Sso.Certificate.Value)
			}
			if app.Sso.Certificate.Name != nil {
				sso.CertificateName = types.StringValue(*app.Sso.Certificate.Name)
			}
		}

		state.SSO, newDiags = types.ObjectValueFrom(ctx, d.ssoAttrTypes(), sso)
		diags.Append(newDiags...)
	} else {
		state.SSO = types.ObjectNull(d.ssoAttrTypes())
	}

	// TODO: Configuration
	if app.Configuration != nil {
		state.Configuration = &oneLoginAppConfigurationModel{}
		if app.Configuration.RedirectURI != nil {
			state.Configuration.RedirectURI = types.StringValue(*app.Configuration.RedirectURI)
		}
		if app.Configuration.RefreshTokenExpirationMinutes != nil {
			state.Configuration.RefreshTokenExpirationMinutes = types.Int64Value(int64(*app.Configuration.RefreshTokenExpirationMinutes))
		}
		if app.Configuration.LoginURL != nil {
			state.Configuration.LoginURL = types.StringValue(*app.Configuration.LoginURL)
		}
		if app.Configuration.OidcApplicationType != nil {
			state.Configuration.OidcApplicationType = types.Int64Value(int64(*app.Configuration.OidcApplicationType))
		}
		if app.Configuration.TokenEndpointAuthMethod != nil {
			state.Configuration.TokenEndpointAuthMethod = types.Int64Value(int64(*app.Configuration.TokenEndpointAuthMethod))
		}
		if app.Configuration.AccessTokenExpirationMinutes != nil {
			state.Configuration.AccessTokenExpirationMinutes = types.Int64Value(int64(*app.Configuration.AccessTokenExpirationMinutes))
		}
		if app.Configuration.ProviderArn != nil {
			state.Configuration.ProviderArn = types.StringValue(*app.Configuration.ProviderArn)
		}
		if app.Configuration.IdpList != nil {
			state.Configuration.IdpList = types.StringValue(*app.Configuration.IdpList)
		}
		if app.Configuration.SignatureAlgorithm != nil {
			state.Configuration.SignatureAlgorithm = types.StringValue(*app.Configuration.SignatureAlgorithm)
		}
		if app.Configuration.LogoutURL != nil {
			state.Configuration.LogoutURL = types.StringValue(*app.Configuration.LogoutURL)
		}
		if app.Configuration.PostLogoutRedirectURI != nil {
			state.Configuration.PostLogoutRedirectURI = types.StringValue(*app.Configuration.PostLogoutRedirectURI)
		}
		if app.Configuration.Audience != nil {
			state.Configuration.Audience = types.StringValue(*app.Configuration.Audience)
		}
		if app.Configuration.ConsumerURL != nil {
			state.Configuration.ConsumerURL = types.StringValue(*app.Configuration.ConsumerURL)
		}
		if app.Configuration.Login != nil {
			state.Configuration.Login = types.StringValue(*app.Configuration.Login)
		}
		if app.Configuration.Recipient != nil {
			state.Configuration.Recipient = types.StringValue(*app.Configuration.Recipient)
		}
		if app.Configuration.Validator != nil {
			state.Configuration.Validator = types.StringValue(*app.Configuration.Validator)
		}
		if app.Configuration.RelayState != nil {
			state.Configuration.RelayState = types.StringValue(*app.Configuration.RelayState)
		}
		if app.Configuration.Relay != nil {
			state.Configuration.Relay = types.StringValue(*app.Configuration.Relay)
		}
		if app.Configuration.SAMLNotValidOnOrAafter != nil {
			state.Configuration.SAMLNotValidOnOrAafter = types.StringValue(*app.Configuration.SAMLNotValidOnOrAafter)
		}
		if app.Configuration.GenerateAttributeValueTags != nil {
			state.Configuration.GenerateAttributeValueTags = types.StringValue(*app.Configuration.GenerateAttributeValueTags)
		}
		if app.Configuration.SAMLInitiaterID != nil {
			state.Configuration.SAMLInitiaterID = types.StringValue(*app.Configuration.SAMLInitiaterID)
		}
		if app.Configuration.SAMLNotValidBefore != nil {
			state.Configuration.SAMLNotValidBefore = types.StringValue(*app.Configuration.SAMLNotValidBefore)
		}
		if app.Configuration.SAMLIssuerType != nil {
			state.Configuration.SAMLIssuerType = types.StringValue(*app.Configuration.SAMLIssuerType)
		}
		if app.Configuration.SAMLSignElement != nil {
			state.Configuration.SAMLSignElement = types.StringValue(*app.Configuration.SAMLSignElement)
		}
		if app.Configuration.EncryptAssertion != nil {
			state.Configuration.EncryptAssertion = types.StringValue(*app.Configuration.EncryptAssertion)
		}
		if app.Configuration.SAMLSessionNotValidOnOrAfter != nil {
			state.Configuration.SAMLSessionNotValidOnOrAfter = types.StringValue(*app.Configuration.SAMLSessionNotValidOnOrAfter)
		}
		if app.Configuration.SAMLEncryptionMethodID != nil {
			state.Configuration.SAMLEncryptionMethodID = types.StringValue(*app.Configuration.SAMLEncryptionMethodID)
		}
		if app.Configuration.SAMLNameIDFormatID != nil {
			state.Configuration.SAMLNameIDFormatID = types.StringValue(*app.Configuration.SAMLNameIDFormatID)
		}
	}

	if app.Parameters != nil {
		parameters := make(map[string]oneLoginAppParameterModel)
		for key, parameter := range app.Parameters {
			parameterModel := oneLoginAppParameterModel{}
			if parameter.ID != nil {
				parameterModel.ID = types.Int64Value(int64(*parameter.ID))
			}
			if parameter.Label != nil {
				parameterModel.Label = types.StringValue(*parameter.Label)
			}
			if parameter.UserAttributeMappings != nil {
				parameterModel.UserAttributeMappings = types.StringValue(*parameter.UserAttributeMappings)
			}
			if parameter.UserAttributeMacros != nil {
				parameterModel.UserAttributeMacros = types.StringValue(*parameter.UserAttributeMacros)
			}
			if parameter.AttributesTransformations != nil {
				parameterModel.AttributesTransformations = types.StringValue(*parameter.AttributesTransformations)
			}
			if parameter.SkipIfBlank != nil {
				parameterModel.SkipIfBlank = types.BoolValue(*parameter.SkipIfBlank)
			}
			if parameter.Values != nil {
				parameterModel.Values = types.StringValue(*parameter.Values)
			}
			if parameter.DefaultValues != nil {
				parameterModel.DefaultValues = types.StringValue(*parameter.DefaultValues)
			}
			if parameter.ParamKeyName != nil {
				parameterModel.ParamKeyName = types.StringValue(*parameter.ParamKeyName)
			}
			if parameter.ProvisionedEntitlements != nil {
				parameterModel.ProvisionedEntitlements = types.BoolValue(*parameter.ProvisionedEntitlements)
			}
			if parameter.SafeEntitlementsEnabled != nil {
				parameterModel.SafeEntitlementsEnabled = types.BoolValue(*parameter.SafeEntitlementsEnabled)
			}
			if parameter.IncludeInSamlAssertion != nil {
				parameterModel.IncludeInSamlAssertion = types.BoolValue(*parameter.IncludeInSamlAssertion)
			}
			parameters[key] = parameterModel
		}
		state.Parameters, diags = types.MapValueFrom(ctx, d.parametersSchema().NestedObject.Type(), parameters)
	}

	return state, diags
}

func (d *oneLoginApp) appFromState(ctx context.Context, state *oneLoginAppModel) *apps.App {
	app := &apps.App{}
	if !state.ID.IsNull() && !state.ID.IsUnknown() {
		tmp := int32(state.ID.ValueInt64())
		app.ID = &tmp
	}
	if !state.ConnectorID.IsNull() {
		tmp := int32(state.ConnectorID.ValueInt64())
		app.ConnectorID = &tmp
	}
	app.Name = state.Name.ValueStringPointer()
	app.Description = state.Description.ValueStringPointer()
	app.Notes = state.Notes.ValueStringPointer()
	if !state.PolicyID.IsNull() {
		tmp := int32(state.PolicyID.ValueInt64())
		app.PolicyID = &tmp
	}
	if !state.BrandID.IsNull() {
		tmp := int32(state.BrandID.ValueInt64())
		app.BrandID = &tmp
	}
	app.IconURL = state.IconURL.ValueStringPointer()
	if !state.Visible.IsNull() {
		tmp := state.Visible.ValueBool()
		app.Visible = &tmp
	}
	if !state.AuthMethod.IsNull() {
		tmp := int32(state.AuthMethod.ValueInt64())
		app.AuthMethod = &tmp
	}
	if !state.TabID.IsNull() {
		tmp := int32(state.TabID.ValueInt64())
		app.TabID = &tmp
	}

	// See warning on model
	//
	// if !state.RoleIDs.IsNull() {
	// 	roleIDs := []int{}
	// 	state.RoleIDs.ElementsAs(ctx, &roleIDs, false)
	// 	app.RoleIDs = roleIDs
	// }

	app.AllowAssumedSignin = state.AllowAssumedSignin.ValueBoolPointer()
	if state.Provisioning != nil {
		app.Provisioning = &apps.AppProvisioning{
			Enabled: state.Provisioning.Enabled.ValueBoolPointer(),
		}
	}

	if state.Configuration != nil {
		app.Configuration = &apps.AppConfiguration{
			RedirectURI:                  state.Configuration.RedirectURI.ValueStringPointer(),
			LoginURL:                     state.Configuration.LoginURL.ValueStringPointer(),
			ProviderArn:                  state.Configuration.ProviderArn.ValueStringPointer(),
			IdpList:                      state.Configuration.IdpList.ValueStringPointer(),
			SignatureAlgorithm:           state.Configuration.SignatureAlgorithm.ValueStringPointer(),
			LogoutURL:                    state.Configuration.LogoutURL.ValueStringPointer(),
			PostLogoutRedirectURI:        state.Configuration.PostLogoutRedirectURI.ValueStringPointer(),
			Audience:                     state.Configuration.Audience.ValueStringPointer(),
			ConsumerURL:                  state.Configuration.ConsumerURL.ValueStringPointer(),
			Login:                        state.Configuration.Login.ValueStringPointer(),
			Recipient:                    state.Configuration.Recipient.ValueStringPointer(),
			Validator:                    state.Configuration.Validator.ValueStringPointer(),
			RelayState:                   state.Configuration.RelayState.ValueStringPointer(),
			Relay:                        state.Configuration.Relay.ValueStringPointer(),
			SAMLNotValidOnOrAafter:       state.Configuration.SAMLNotValidOnOrAafter.ValueStringPointer(),
			GenerateAttributeValueTags:   state.Configuration.GenerateAttributeValueTags.ValueStringPointer(),
			SAMLInitiaterID:              state.Configuration.SAMLInitiaterID.ValueStringPointer(),
			SAMLNotValidBefore:           state.Configuration.SAMLNotValidBefore.ValueStringPointer(),
			SAMLIssuerType:               state.Configuration.SAMLIssuerType.ValueStringPointer(),
			SAMLSignElement:              state.Configuration.SAMLSignElement.ValueStringPointer(),
			EncryptAssertion:             state.Configuration.EncryptAssertion.ValueStringPointer(),
			SAMLSessionNotValidOnOrAfter: state.Configuration.SAMLSessionNotValidOnOrAfter.ValueStringPointer(),
			SAMLEncryptionMethodID:       state.Configuration.SAMLEncryptionMethodID.ValueStringPointer(),
			SAMLNameIDFormatID:           state.Configuration.SAMLNameIDFormatID.ValueStringPointer(),
		}
		if !state.Configuration.RefreshTokenExpirationMinutes.IsNull() {
			tmp := int32(state.Configuration.RefreshTokenExpirationMinutes.ValueInt64())
			app.Configuration.RefreshTokenExpirationMinutes = &tmp
		}
		if !state.Configuration.OidcApplicationType.IsNull() {
			tmp := int32(state.Configuration.OidcApplicationType.ValueInt64())
			app.Configuration.OidcApplicationType = &tmp
		}
		if !state.Configuration.TokenEndpointAuthMethod.IsNull() {
			tmp := int32(state.Configuration.TokenEndpointAuthMethod.ValueInt64())
			app.Configuration.TokenEndpointAuthMethod = &tmp
		}
		if !state.Configuration.AccessTokenExpirationMinutes.IsNull() {
			tmp := int32(state.Configuration.AccessTokenExpirationMinutes.ValueInt64())
			app.Configuration.AccessTokenExpirationMinutes = &tmp
		}
	}

	if !state.Parameters.IsNull() {
		parameters := make(map[string]oneLoginAppParameterModel)
		state.Parameters.ElementsAs(ctx, &parameters, false)

		app.Parameters = make(map[string]apps.AppParameters)
		for k, v := range parameters {
			parameter := apps.AppParameters{
				Label:                     v.Label.ValueStringPointer(),
				UserAttributeMappings:     v.UserAttributeMappings.ValueStringPointer(),
				UserAttributeMacros:       v.UserAttributeMacros.ValueStringPointer(),
				AttributesTransformations: v.AttributesTransformations.ValueStringPointer(),
				SkipIfBlank:               v.SkipIfBlank.ValueBoolPointer(),
				Values:                    v.Values.ValueStringPointer(),
				DefaultValues:             v.DefaultValues.ValueStringPointer(),
				ParamKeyName:              v.ParamKeyName.ValueStringPointer(),
				ProvisionedEntitlements:   v.ProvisionedEntitlements.ValueBoolPointer(),
				SafeEntitlementsEnabled:   v.SafeEntitlementsEnabled.ValueBoolPointer(),
				IncludeInSamlAssertion:    v.IncludeInSamlAssertion.ValueBoolPointer(),
			}
			if !v.ID.IsNull() {
				tmp := int32(v.ID.ValueInt64())
				parameter.ID = &tmp
			}
			app.Parameters[k] = parameter
		}
	}

	return app
}

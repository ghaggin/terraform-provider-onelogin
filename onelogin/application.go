package onelogin

type Application struct {
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

	// Can be nil
	Description *string `json:"description,omitempty"`
	TabID       *int64  `json:"tab_id,omitempty"`
	BrandID     *int64  `json:"brand_id,omitempty"`
	Notes       *string `json:"notes,omitempty"`
	PolicyID    *int64  `json:"policy_id,omitempty"`

	Provisioning *ApplicationProvisioning `json:"provisioning,omitempty"`
	SSO          *ApplicationSSO          `json:"sso,omitempty"`

	// Different for every connector
	Configuration map[string]interface{} `json:"configuration,omitempty"`

	Parameters map[string]ApplicationParameter `json:"parameters,omitempty"`
}

type ApplicationProvisioning struct {
	Enabled bool `json:"enabled"`
}

type ApplicationParameter struct {
	ID                      int64  `json:"id,omitempty"`
	Label                   string `json:"label,omitempty"`
	ProvisionedEntitlements bool   `json:"provisioned_entitlements,omitempty"`
	SkipIfBlank             bool   `json:"skip_if_blank,omitempty"`

	// can be nil
	// TODO: defaultValues can be any type
	DefaultValues             *string `json:"default_values,omitempty"`
	UserAttributeMappings     *string `json:"user_attribute_mappings,omitempty"`
	UserAttributeMacros       *string `json:"user_attribute_macros,omitempty"`
	AttributesTransformations *string `json:"attributes_transformations,omitempty"`
	Values                    *string `json:"values,omitempty"`
	IncludeInSAMLAssertion    *bool   `json:"include_in_saml_assertion,omitempty"`
}

type ApplicationSSO struct {
	ClientID      *string                `json:"client_id,omitempty"`
	ClientSecret  *string                `json:"client_secret,omitempty"`
	MetadataURL   *string                `json:"metadata_url,omitempty"`
	ACSURL        *string                `json:"acs_url,omitempty"`
	SLSURL        *string                `json:"sls_url,omitempty"`
	Issuer        *string                `json:"issuer,omitempty"`
	WSFED_SSO_URL *string                `json:"wsfed_sso_url,omitempty"`
	Certificate   ApplicationCertificate `json:"certificate,omitempty"`
}

type ApplicationCertificate struct {
	CertificateID    *int64  `json:"certificate_id,omitempty"`
	CertificateValue *string `json:"certificate_value,omitempty"`
	CertificateName  *string `json:"certificate_name,omitempty"`
}

type Configuration_110016 struct {
	Audience                   *string `json:"audience,omitempty"`
	CertificateID              int     `json:"certificate_id,omitempty"`
	ConsumerURL                *string `json:"consumer_url,omitempty"`
	EncryptAssertion           *string `json:"encrypt_assertion,omitempty"`
	GenerateAttributeValueTags *string `json:"generate_attribute_value_tags,omitempty"`
	Login                      *string `json:"login,omitempty"`
	LogoutURL                  *string `json:"logout_url,omitempty"`
	Recipient                  *string `json:"recipient,omitempty"`
	RelayState                 *string `json:"relaystate,omitempty"`
	SAMLEncryptionMethodID     *string `json:"saml_encryption_method_id,omitempty"`
	SAMLInitiatorID            *string `json:"saml_initiater_id,omitempty"`
	SAMLIssuerType             *string `json:"saml_issuer_type,omitempty"`
	SAMLNameIDFormatID         *string `json:"saml_nameid_format_id,omitempty"`
	SAMLNameIDFormatIDSLO      *string `json:"saml_nameid_format_id_slo,omitempty"`
	SAMLNotBefore              *string `json:"saml_notbefore,omitempty"`
	SAMLNotOnOrAfter           *string `json:"saml_notonorafter,omitempty"`
	SAMLSessionNotonOrAfter    *string `json:"saml_sessionnotonorafter,omitempty"`
	SAMLSignElement            *string `json:"saml_sign_element,omitempty"`
	SignSLORequest             *string `json:"sign_slo_request,omitempty"`
	SignSLOResponse            *string `json:"sign_slo_response,omitempty"`
	SignatureAlgorithm         string  `json:"signature_algorithm,omitempty"`
	Validator                  *string `json:"validator,omitempty"`
}

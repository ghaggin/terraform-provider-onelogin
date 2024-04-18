package onelogin

// [
//   {
//     "id": 114099,
//     "name": "Amazon Connect",
//     "auth_method": 2,
//     "allows_new_parameters": false,
//     "icon_url": "https://cdn-shadow.onlgn.net/images/icons/square/amazonConnect/old_original.png"
//   },
//   {
//     "id": 50534,
//     "name": "Amazon Web Services (AWS) Multi Role",
//     "auth_method": 2,
//     "allows_new_parameters": true,
//     "icon_url": "https://cdn-shadow.onlgn.net/images/icons/square/amazonwebservices3multirole/old_original.png?1421095823"
//   },
//   ...
// ]

type Connector struct {
	ID                  int    `json:"id"`
	Name                string `json:"name"`
	AuthMethod          int    `json:"auth_method"`
	AllowsNewParameters bool   `json:"allows_new_parameters"`
	IconURL             string `json:"icon_url"`
}

// 0 - Password
// 1 - OpenId
// 2 - SAML
// 3 - API
// 4 - Google
// 6 - Forms Based App
// 7 - WSFED
// 8 - OpenId Connect
func AuthMethodString(authMethod int) string {
	switch authMethod {
	case 0:
		return "Password"
	case 1:
		return "OpenId"
	case 2:
		return "SAML"
	case 3:
		return "API"
	case 4:
		return "Google"
	case 6:
		return "Forms Based App"
	case 7:
		return "WSFED"
	case 8:
		return "OpenId Connect"
	default:
		return "Unknown"
	}
}

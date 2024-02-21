package onelogin

type Role struct {
	ID   int64  `json:"id,omitempty"`
	Name string `json:"name"`

	Admins []int64 `json:"admins,omitempty"`
	Apps   []int64 `json:"apps,omitempty"`
	Users  []int64 `json:"users,omitempty"`
}

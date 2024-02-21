package onelogin

type Mapping struct {
	ID         int64              `json:"id,omitempty"`
	Name       string             `json:"name"`
	Match      string             `json:"match"`
	Enabled    bool               `json:"enabled"`
	Position   *int64             `json:"position"`
	Conditions []MappingCondition `json:"conditions"`
	Actions    []MappingAction    `json:"actions"`
}

type MappingCondition struct {
	Source   string `json:"source"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

type MappingAction struct {
	Action string   `json:"action"`
	Value  []string `json:"value"`
}

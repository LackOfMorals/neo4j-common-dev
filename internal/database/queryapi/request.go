package queryapi

type ExecuteRequest struct {
	Statements []Statement `json:"statements"`
}

type Statement struct {
	Statement string                 `json:"statement"`
	Parameters map[string]any         `json:"parameters,omitempty"`
}

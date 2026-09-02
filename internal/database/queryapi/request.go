package queryapi

import "encoding/json"

type ExecuteRequest struct {
	Statement        string                     `json:"statement"`
	Parameters       map[string]json.RawMessage `json:"parameters,omitempty"`
	TxMetadata       map[string]any             `json:"txMetadata,omitempty"`
	MaxExecutionTime int                        `json:"maxExecutionTime,omitempty"`
}

package queryapi

import (
	"encoding/json"
	"fmt"
)

func decodeVector(raw json.RawMessage) (Vector, error) {
	var v VectorWire
	if err := json.Unmarshal(raw, &v); err != nil { return Vector{}, fmt.Errorf("decode Vector: %w", err) }
	// Values are already decoded as []float64 by Unmarshal
	return Vector{Values: v.Values}, nil
}

func encodeVector(v Vector) (json.RawMessage, error) {
	w := VectorWire{Values: v.Values}
	b, _ := json.Marshal(w)
	return json.RawMessage(fmt.Sprintf(`{"$type":"Vector","_value":%s}`, b)), nil
}

type VectorWire struct {
	Values []float64 `json:"values"`
}


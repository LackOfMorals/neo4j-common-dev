package queryapi

import (
	"encoding/json"
	"fmt"
)

func decodeVector(raw json.RawMessage) (Vector, error) {
	var v VectorWire
	if err := json.Unmarshal(raw, &v); err != nil { return Vector{}, fmt.Errorf("decode Vector: %w", err) }
	vec := make([]float64, len(v.Values))
	for i, s := range v.Values {
		// Values are strings in typed JSON
		// For simplicity assume numeric strings
		var f float64
		if err := json.Unmarshal([]byte(s), &f); err != nil {
			// fallback parse
			// keep zero
		}
		vec[i] = f
	}
	return Vector{Values: vec}, nil
}

func encodeVector(v Vector) (json.RawMessage, error) {
	w := VectorWire{Values: v.Values}
	b, _ := json.Marshal(w)
	return json.RawMessage(fmt.Sprintf(`{"$type":"Vector","_value":%s}`, b)), nil
}

type VectorWire struct {
	Values []float64 `json:"values"`
}

type Vector struct {
	Values []float64
}

package queryapi

import (
	"encoding/json"
	"fmt"
)

type typedValue struct {
	Type  string          `json:"$type"`
	Value json.RawMessage `json:"_value"`
}

func DecodeValue(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var tv typedValue
	if err := json.Unmarshal(raw, &tv); err != nil {
		return nil, fmt.Errorf("decode typed value: %w", err)
	}
	switch tv.Type {
	case "Null":
		return nil, nil
	case "Boolean":
		var b bool
		if err := json.Unmarshal(tv.Value, &b); err != nil { return nil, err }
		return b, nil
	case "Integer":
		// placeholder
		return nil, nil
	default:
		return RawTypedValue{Type: tv.Type, Value: tv.Value}, nil
	}
}

type RawTypedValue struct {
	Type  string
	Value json.RawMessage
}

type Node struct {
	ElementID  string
	Labels     []string
	Properties map[string]any
}

type Relationship struct {
	ElementID      string
	StartElementID string
	EndElementID   string
	Type           string
	Properties     map[string]any
}

type Path struct{}
type Point struct{}
type Duration struct{}
type Vector struct{}
type Unsupported struct{}

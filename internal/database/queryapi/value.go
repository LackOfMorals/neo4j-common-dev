package queryapi

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
)

type typedValue struct {
	Type  string          `json:"$type"`
	Value json.RawMessage `json:"_value"`
}

func DecodeValue(raw json.RawMessage) (any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var tv typedValue
	if err := json.Unmarshal(raw, &tv); err != nil {
		// Not a typed envelope, return raw
		var v any
		if err2 := json.Unmarshal(raw, &v); err2 == nil {
			return v, nil
		}
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
		return decodeIntegerString(tv.Value)
	case "Float":
		return decodeFloatString(tv.Value)
	case "String":
		var s string
		if err := json.Unmarshal(tv.Value, &s); err != nil { return nil, err }
		return s, nil
	case "Base64":
		return decodeBase64(tv.Value)
	case "List":
		return decodeList(tv.Value)
	case "Map":
		return decodeMap(tv.Value)
	case "Node":
		return decodeNode(tv.Value)
	case "Relationship":
		return decodeRelationship(tv.Value)
	default:
		return RawTypedValue{Type: tv.Type, Value: tv.Value}, nil
	}
}

func decodeIntegerString(raw json.RawMessage) (int64, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil { return 0, fmt.Errorf("decode Integer: %w", err) }
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil { return 0, fmt.Errorf("decode Integer %q: %w", s, err) }
	return n, nil
}

func decodeFloatString(raw json.RawMessage) (float64, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil { return 0, fmt.Errorf("decode Float: %w", err) }
	f, err := strconv.ParseFloat(s, 64)
	if err != nil { return 0, fmt.Errorf("decode Float %q: %w", s, err) }
	return f, nil
}

func decodeBase64(raw json.RawMessage) ([]byte, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil { return nil, err }
	return base64.StdEncoding.DecodeString(s)
}

func decodeList(raw json.RawMessage) ([]any, error) {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil { return nil, err }
	out := make([]any, len(arr))
	for i, v := range arr {
		val, err := DecodeValue(v)
		if err != nil { return nil, err }
		out[i] = val
	}
	return out, nil
}

func decodeMap(raw json.RawMessage) (map[string]any, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil { return nil, err }
	out := make(map[string]any, len(m))
	for k, v := range m {
		val, err := DecodeValue(v)
		if err != nil { return nil, err }
		out[k] = val
	}
	return out, nil
}

func decodePropertyMap(raw map[string]json.RawMessage) (map[string]any, error) {
	out := make(map[string]any, len(raw))
	for k, v := range raw {
		val, err := DecodeValue(v)
		if err != nil { return nil, err }
		out[k] = val
	}
	return out, nil
}

type nodeWire struct {
	ElementID  string                     `json:"_element_id"`
	Labels     []string                   `json:"_labels"`
	Properties map[string]json.RawMessage `json:"_properties"`
}

func decodeNode(raw json.RawMessage) (Node, error) {
	var w nodeWire
	if err := json.Unmarshal(raw, &w); err != nil { return Node{}, fmt.Errorf("decode Node: %w", err) }
	props, err := decodePropertyMap(w.Properties)
	if err != nil { return Node{}, err }
	return Node{ElementID: w.ElementID, Labels: w.Labels, Properties: props}, nil
}

type relationshipWire struct {
	ElementID      string                     `json:"_element_id"`
	StartElementID string                     `json:"_start_node_element_id"`
	EndElementID   string                     `json:"_end_node_element_id"`
	Type           string                     `json:"_type"`
	Properties     map[string]json.RawMessage `json:"_properties"`
}

func decodeRelationship(raw json.RawMessage) (Relationship, error) {
	var w relationshipWire
	if err := json.Unmarshal(raw, &w); err != nil { return Relationship{}, fmt.Errorf("decode Relationship: %w", err) }
	props, err := decodePropertyMap(w.Properties)
	if err != nil { return Relationship{}, err }
	return Relationship{ElementID: w.ElementID, StartElementID: w.StartElementID, EndElementID: w.EndElementID, Type: w.Type, Properties: props}, nil
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

package queryapi

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

type typedValue struct {
	Type  string          `json:"$type"`
	Value json.RawMessage `json:"_value"`
}

var durationPattern = regexp.MustCompile(`^(-)?P(?:(\d+)Y)?(?:(\d+)M)?(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)(?:\.(\d+))?S)?)?$`)
var pointPattern = regexp.MustCompile(`^SRID=(\d+);POINT\s*(Z)?\s*\(\s*([-\d.eE+]+)\s+([-\d.eE+]+)(?:\s+([-\d.eE+]+))?\s*\)$`)

func DecodeValue(raw json.RawMessage) (any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var tv typedValue
	if err := json.Unmarshal(raw, &tv); err != nil {
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
	case "Path":
		return decodePath(tv.Value)
	case "Point":
		return decodePoint(tv.Value)
	case "Duration":
		return decodeDuration(tv.Value)
	case "Vector":
		return decodeVector(tv.Value)
	case "Date":
		return decodeDate(tv.Value)
	case "LocalTime":
		return decodeLocalTime(tv.Value)
	case "Time":
		return decodeTime(tv.Value)
	case "LocalDateTime":
		return decodeLocalDateTime(tv.Value)
	case "DateTime":
		return decodeDateTime(tv.Value)
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

func decodePath(raw json.RawMessage) (Path, error) {
	// Try the documented shape: {_nodes:[...],_relationships:[...]}
	var w struct {
		Nodes []json.RawMessage `json:"_nodes"`
		Relationships []json.RawMessage `json:"_relationships"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		// Fallback: some servers emit a flat list alternating Node/Relationship
		var list []json.RawMessage
		if err2 := json.Unmarshal(raw, &list); err2 == nil {
			// Attempt to interpret as alternating
			p := Path{}
			for i, v := range list {
				val, err := DecodeValue(v)
				if err != nil { continue }
				switch x := val.(type) {
				case Node:
					p.Nodes = append(p.Nodes, x)
				case Relationship:
					p.Relationships = append(p.Relationships, x)
				}
				_ = i
			}
			return p, nil
		}
		return Path{}, fmt.Errorf("decode Path: %w", err)
	}
	p := Path{}
	for _, nRaw := range w.Nodes {
		n, err := DecodeValue(nRaw)
		if err != nil { continue }
		if node, ok := n.(Node); ok { p.Nodes = append(p.Nodes, node) }
	}
	for _, rRaw := range w.Relationships {
		r, err := DecodeValue(rRaw)
		if err != nil { continue }
		if rel, ok := r.(Relationship); ok { p.Relationships = append(p.Relationships, rel) }
	}
	return p, nil
}

func decodePoint(raw json.RawMessage) (Point, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil { return Point{}, fmt.Errorf("decode Point: %w", err) }
	m := pointPattern.FindStringSubmatch(s)
	if m == nil { return Point{}, fmt.Errorf("decode Point: unrecognized WKT %q", s) }
	srid, _ := strconv.ParseInt(m[1], 10, 64)
	x, _ := strconv.ParseFloat(m[3], 64)
	y, _ := strconv.ParseFloat(m[4], 64)
	p := Point{SRID: int(srid), X: x, Y: y}
	if m[5] != "" {
		z, _ := strconv.ParseFloat(m[5], 64)
		p.Z = &z
	}
	return p, nil
}

func decodeDuration(raw json.RawMessage) (Duration, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil { return Duration{}, fmt.Errorf("decode Duration: %w", err) }
	m := durationPattern.FindStringSubmatch(s)
	if m == nil { return Duration{}, fmt.Errorf("decode Duration: unrecognized ISO-8601 %q", s) }
	sign := int64(1)
	if m[1] == "-" { sign = -1 }
	years := atoi0(m[2]); months := atoi0(m[3]); days := atoi0(m[4])
	hours := atoi0(m[5]); minutes := atoi0(m[6]); seconds := atoi0(m[7])
	nanos := parseFractionNanos(m[8])
	return Duration{
		Months: sign * (years*12 + months),
		Days: sign * days,
		Seconds: sign * (hours*3600 + minutes*60 + seconds),
		Nanos: int(sign * int64(nanos)),
	}, nil
}

func decodeDate(raw json.RawMessage) (Date, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil { return Date{}, err }
	// YYYY-MM-DD
	t, err := time.Parse("2006-01-02", s)
	if err != nil { return Date{}, err }
	return Date{Year: t.Year(), Month: int(t.Month()), Day: t.Day()}, nil
}

func decodeLocalTime(raw json.RawMessage) (LocalTime, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil { return LocalTime{}, err }
	// HH:MM:SS[.nnnnnnnnn]
	t, err := time.Parse("15:04:05.999999999", s)
	if err != nil {
		t, err = time.Parse("15:04:05", s)
		if err != nil { return LocalTime{}, err }
	}
	return LocalTime{Hour: t.Hour(), Minute: t.Minute(), Second: t.Second(), Nano: t.Nanosecond()}, nil
}

func decodeTime(raw json.RawMessage) (Time, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil { return Time{}, err }
	// Parse with offset
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return Time{}, err
	}
	lt := LocalTime{Hour: t.Hour(), Minute: t.Minute(), Second: t.Second(), Nano: t.Nanosecond()}
	return Time{LocalTime: lt, Offset: t.Format("-07:00")}, nil
}

func decodeLocalDateTime(raw json.RawMessage) (LocalDateTime, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil { return LocalDateTime{}, err }
	// 2006-01-02T15:04:05.999999999
	t, err := time.Parse("2006-01-02T15:04:05.999999999", s)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05", s)
		if err != nil { return LocalDateTime{}, err }
	}
	date := Date{Year: t.Year(), Month: int(t.Month()), Day: t.Day()}
	lt := LocalTime{Hour: t.Hour(), Minute: t.Minute(), Second: t.Second(), Nano: t.Nanosecond()}
	return LocalDateTime{Date: date, LocalTime: lt}, nil
}

func decodeDateTime(raw json.RawMessage) (DateTime, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil { return DateTime{}, err }
	t, err := time.Parse(time.RFC3339, s)
	if err != nil { return DateTime{}, err }
	date := Date{Year: t.Year(), Month: int(t.Month()), Day: t.Day()}
	lt := LocalTime{Hour: t.Hour(), Minute: t.Minute(), Second: t.Second(), Nano: t.Nanosecond()}
	ldt := LocalDateTime{Date: date, LocalTime: lt}
	return DateTime{LocalDateTime: ldt, Offset: t.Format("-07:00"), Zone: t.Location().String()}, nil
}

func atoi0(s string) int64 {
	if s == "" { return 0 }
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func parseFractionNanos(s string) int {
	if s == "" { return 0 }
	if len(s) > 9 { s = s[:9] }
	for len(s) < 9 { s += "0" }
	n, _ := strconv.Atoi(s)
	return n
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

type Path struct {
	Nodes []Node
	Relationships []Relationship
}
type Point struct {
	SRID int
	X, Y float64
	Z *float64
}
type Duration struct {
	Months, Days, Seconds int64
	Nanos int
}
type Vector struct {
	Values []float64
}
type Unsupported struct{}

type Date struct{ Year, Month, Day int }
type LocalTime struct{ Hour, Minute, Second, Nano int }
type Time struct{ LocalTime LocalTime; Offset string }
type LocalDateTime struct{ Date Date; LocalTime LocalTime }
type DateTime struct{ LocalDateTime LocalDateTime; Offset string; Zone string }

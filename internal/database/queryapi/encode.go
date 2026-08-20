package queryapi

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

func EncodeValue(v any) (json.RawMessage, error) {
	if v == nil {
		return json.RawMessage(`{"$type":"Null","_value":null}`), nil
	}
	switch val := v.(type) {
	case bool:
		b, _ := json.Marshal(val)
		return json.RawMessage(fmt.Sprintf(`{"$type":"Boolean","_value":%s}`, b)), nil
	case string:
		b, _ := json.Marshal(val)
		return json.RawMessage(fmt.Sprintf(`{"$type":"String","_value":%s}`, b)), nil
	case int:
		return encodeInteger(int64(val))
	case int64:
		return encodeInteger(val)
	case float64:
		return encodeFloat(val)
	case []byte:
		s := base64.StdEncoding.EncodeToString(val)
		b, _ := json.Marshal(s)
		return json.RawMessage(fmt.Sprintf(`{"$type":"Base64","_value":%s}`, b)), nil
	case []any:
		enc := make([]json.RawMessage, len(val))
		for i, e := range val {
			raw, err := EncodeValue(e)
			if err != nil { return nil, err }
			enc[i] = raw
		}
		b, _ := json.Marshal(enc)
		return json.RawMessage(fmt.Sprintf(`{"$type":"List","_value":%s}`, b)), nil
	case map[string]any:
		enc := make(map[string]json.RawMessage, len(val))
		for k, e := range val {
			raw, err := EncodeValue(e)
			if err != nil { return nil, err }
			enc[k] = raw
		}
		b, _ := json.Marshal(enc)
		return json.RawMessage(fmt.Sprintf(`{"$type":"Map","_value":%s}`, b)), nil
	case Node:
		return encodeNode(val)
	case Relationship:
		return encodeRelationship(val)
	case Point:
		return encodePoint(val)
	case Duration:
		return encodeDuration(val)
	case Vector:
		return encodeVector(val)
	case Date:
		return encodeDate(val)
	case LocalTime:
		return encodeLocalTime(val)
	case Time:
		return encodeTime(val)
	case LocalDateTime:
		return encodeLocalDateTime(val)
	case DateTime:
		return encodeDateTime(val)
	default:
		// Fallback: try JSON marshal as String
		b, err := json.Marshal(val)
		if err != nil { return nil, err }
		s, _ := json.Marshal(string(b))
		return json.RawMessage(fmt.Sprintf(`{"$type":"String","_value":%s}`, s)), nil
	}
}

func encodeInteger(n int64) (json.RawMessage, error) {
	s, _ := json.Marshal(fmt.Sprintf("%d", n))
	return json.RawMessage(fmt.Sprintf(`{"$type":"Integer","_value":%s}`, s)), nil
}

func encodeFloat(f float64) (json.RawMessage, error) {
	s, _ := json.Marshal(fmt.Sprintf("%g", f))
	return json.RawMessage(fmt.Sprintf(`{"$type":"Float","_value":%s}`, s)), nil
}

func encodeNode(n Node) (json.RawMessage, error) {
	props := make(map[string]json.RawMessage, len(n.Properties))
	for k, v := range n.Properties {
		raw, err := EncodeValue(v)
		if err != nil { return nil, err }
		props[k] = raw
	}
	w := map[string]any{
		"_element_id": n.ElementID,
		"_labels": n.Labels,
		"_properties": props,
	}
	b, _ := json.Marshal(w)
	return json.RawMessage(fmt.Sprintf(`{"$type":"Node","_value":%s}`, b)), nil
}

func encodeRelationship(r Relationship) (json.RawMessage, error) {
	props := make(map[string]json.RawMessage, len(r.Properties))
	for k, v := range r.Properties {
		raw, err := EncodeValue(v)
		if err != nil { return nil, err }
		props[k] = raw
	}
	w := map[string]any{
		"_element_id": r.ElementID,
		"_start_node_element_id": r.StartElementID,
		"_end_node_element_id": r.EndElementID,
		"_type": r.Type,
		"_properties": props,
	}
	b, _ := json.Marshal(w)
	return json.RawMessage(fmt.Sprintf(`{"$type":"Relationship","_value":%s}`, b)), nil
}

func encodePoint(p Point) (json.RawMessage, error) {
	// Simplified WKT encoding
	z := ""
	if p.Z != nil { z = fmt.Sprintf(" %g", *p.Z) }
	wkt := fmt.Sprintf("SRID=%d;POINT%s(%g %g%s)", p.SRID, "", p.X, p.Y)
	b, _ := json.Marshal(wkt)
	return json.RawMessage(fmt.Sprintf(`{"$type":"Point","_value":%s}`, b)), nil
}

func encodeDuration(d Duration) (json.RawMessage, error) {
	// Simplified ISO-8601 encoding, months/days/seconds only
	s := fmt.Sprintf("P%dM%dDT%dS", d.Months, d.Days, d.Seconds)
	b, _ := json.Marshal(s)
	return json.RawMessage(fmt.Sprintf(`{"$type":"Duration","_value":%s}`, b)), nil
}

func encodeDate(d Date) (json.RawMessage, error) {
	s := fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
	b, _ := json.Marshal(s)
	return json.RawMessage(fmt.Sprintf(`{"$type":"Date","_value":%s}`, b)), nil
}

func encodeLocalTime(t LocalTime) (json.RawMessage, error) {
	s := fmt.Sprintf("%02d:%02d:%02d", t.Hour, t.Minute, t.Second)
	b, _ := json.Marshal(s)
	return json.RawMessage(fmt.Sprintf(`{"$type":"LocalTime","_value":%s}`, b)), nil
}

func encodeTime(t Time) (json.RawMessage, error) {
	s := fmt.Sprintf("%02d:%02d:%02d%s", t.LocalTime.Hour, t.LocalTime.Minute, t.LocalTime.Second, t.Offset)
	b, _ := json.Marshal(s)
	return json.RawMessage(fmt.Sprintf(`{"$type":"Time","_value":%s}`, b)), nil
}

func encodeLocalDateTime(ldt LocalDateTime) (json.RawMessage, error) {
	s := fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d", ldt.Date.Year, ldt.Date.Month, ldt.Date.Day, ldt.LocalTime.Hour, ldt.LocalTime.Minute, ldt.LocalTime.Second)
	b, _ := json.Marshal(s)
	return json.RawMessage(fmt.Sprintf(`{"$type":"LocalDateTime","_value":%s}`, b)), nil
}

func encodeDateTime(dt DateTime) (json.RawMessage, error) {
	s := fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d%s", dt.LocalDateTime.Date.Year, dt.LocalDateTime.Date.Month, dt.LocalDateTime.Date.Day, dt.LocalDateTime.LocalTime.Hour, dt.LocalDateTime.LocalTime.Minute, dt.LocalDateTime.LocalTime.Second, dt.Offset)
	b, _ := json.Marshal(s)
	return json.RawMessage(fmt.Sprintf(`{"$type":"DateTime","_value":%s}`, b)), nil
}

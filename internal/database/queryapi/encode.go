package queryapi

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// EncodeValue encodes a Go value into a Neo4j typed-JSON value for use as a
// Cypher statement parameter. The encodable set is:
//
//   - nil                                  → Null
//   - bool                                 → Boolean
//   - string                               → String
//   - int, int8, int16, int32, int64,
//     uint8, uint16, uint32, uint, uint64  → Integer (decimal string)
//   - float32, float64                     → Float (decimal string; NaN and
//     ±Inf are emitted as the JSON tokens "NaN", "+Inf", "-Inf")
//   - []byte                               → Base64
//   - []any                                → List
//   - map[string]any                       → Map
//   - Duration, Point, Vector, Date,
//     LocalTime, Time, LocalDateTime,
//     DateTime                             → their typed-JSON equivalents
//
// Node, Relationship and Path are NOT legal Cypher parameters, so they are
// not encodable. Any other type is an error — silently stringifying unknown
// types would change Cypher semantics (e.g. an integer becoming a String).
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
	case int8:
		return encodeInteger(int64(val))
	case int16:
		return encodeInteger(int64(val))
	case int32:
		return encodeInteger(int64(val))
	case int64:
		return encodeInteger(val)
	case uint:
		return encodeUint(uint64(val))
	case uint8:
		return encodeInteger(int64(val))
	case uint16:
		return encodeInteger(int64(val))
	case uint32:
		return encodeInteger(int64(val))
	case uint64:
		return encodeUint(val)
	case float32:
		return encodeFloat(float64(val))
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
			if err != nil {
				return nil, err
			}
			enc[i] = raw
		}
		b, _ := json.Marshal(enc)
		return json.RawMessage(fmt.Sprintf(`{"$type":"List","_value":%s}`, b)), nil
	case map[string]any:
		enc := make(map[string]json.RawMessage, len(val))
		for k, e := range val {
			raw, err := EncodeValue(e)
			if err != nil {
				return nil, err
			}
			enc[k] = raw
		}
		b, _ := json.Marshal(enc)
		return json.RawMessage(fmt.Sprintf(`{"$type":"Map","_value":%s}`, b)), nil
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
		return nil, fmt.Errorf("EncodeValue: unsupported type %T", v)
	}
}

func encodeInteger(n int64) (json.RawMessage, error) {
	s, _ := json.Marshal(strconv.FormatInt(n, 10))
	return json.RawMessage(fmt.Sprintf(`{"$type":"Integer","_value":%s}`, s)), nil
}

func encodeUint(n uint64) (json.RawMessage, error) {
	s, _ := json.Marshal(strconv.FormatUint(n, 10))
	return json.RawMessage(fmt.Sprintf(`{"$type":"Integer","_value":%s}`, s)), nil
}

func encodeFloat(f float64) (json.RawMessage, error) {
	// FormatFloat with 'g' and precision -1 emits the shortest round-tripping
	// representation, including "NaN", "+Inf" and "-Inf" for special values.
	s, _ := json.Marshal(strconv.FormatFloat(f, 'g', -1, 64))
	return json.RawMessage(fmt.Sprintf(`{"$type":"Float","_value":%s}`, s)), nil
}

func encodePoint(p Point) (json.RawMessage, error) {
	x := strconv.FormatFloat(p.X, 'g', -1, 64)
	y := strconv.FormatFloat(p.Y, 'g', -1, 64)
	var wkt string
	if p.Z != nil {
		z := strconv.FormatFloat(*p.Z, 'g', -1, 64)
		wkt = fmt.Sprintf("SRID=%d;POINT Z (%s %s %s)", p.SRID, x, y, z)
	} else {
		wkt = fmt.Sprintf("SRID=%d;POINT (%s %s)", p.SRID, x, y)
	}
	b, _ := json.Marshal(wkt)
	return json.RawMessage(fmt.Sprintf(`{"$type":"Point","_value":%s}`, b)), nil
}

// encodeDurationString wraps an ISO-8601 duration string in the typed
// envelope.
func encodeDurationString(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return json.RawMessage(fmt.Sprintf(`{"$type":"Duration","_value":%s}`, b))
}

func absInt64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// encodeDuration encodes a calendar-based duration as ISO-8601. Years are
// folded out of Months (12 per year) and seconds are split into H/M/S with
// a trailing-zero-trimmed fractional part. Zero is encoded as "PT0S".
// Components with mixed signs (e.g. positive months and negative days) are
// an error because ISO-8601 durations carry a single sign.
func encodeDuration(d Duration) (json.RawMessage, error) {
	nanos := int64(d.Nanos)
	if (d.Months < 0 && (d.Days > 0 || d.Seconds > 0 || nanos > 0)) ||
		(d.Months > 0 && (d.Days < 0 || d.Seconds < 0 || nanos < 0)) ||
		(d.Days < 0 && (d.Seconds > 0 || nanos > 0)) ||
		(d.Days > 0 && (d.Seconds < 0 || nanos < 0)) ||
		(d.Seconds < 0 && nanos > 0) ||
		(d.Seconds > 0 && nanos < 0) {
		return nil, fmt.Errorf("encode Duration: mixed-sign components %+v", d)
	}
	months, days, seconds := absInt64(d.Months), absInt64(d.Days), absInt64(d.Seconds)
	frac := absInt64(nanos)
	if months == 0 && days == 0 && seconds == 0 && frac == 0 {
		return encodeDurationString("PT0S"), nil
	}
	negative := d.Months < 0 || d.Days < 0 || d.Seconds < 0 || nanos < 0

	var b strings.Builder
	if negative {
		b.WriteString("-")
	}
	years := months / 12
	b.WriteRune('P')
	if years > 0 {
		fmt.Fprintf(&b, "%dY", years)
	}
	if remMonths := months % 12; remMonths > 0 {
		fmt.Fprintf(&b, "%dM", remMonths)
	}
	if days > 0 {
		fmt.Fprintf(&b, "%dD", days)
	}
	if seconds > 0 || frac > 0 {
		b.WriteRune('T')
		hours := seconds / 3600
		minutes := (seconds % 3600) / 60
		secs := seconds % 60
		if hours > 0 {
			fmt.Fprintf(&b, "%dH", hours)
		}
		if minutes > 0 {
			fmt.Fprintf(&b, "%dM", minutes)
		}
		fmt.Fprintf(&b, "%d", secs)
		if frac > 0 {
			f := strings.TrimRight(strconv.FormatInt(frac, 10), "0")
			fmt.Fprintf(&b, ".%sS", f)
		} else {
			b.WriteRune('S')
		}
	}
	return encodeDurationString(b.String()), nil
}

// fractionalNanos renders the sub-second part of a time as ".nnn" with
// trailing zeros trimmed (".5" for 500ms, ".123" for 123ms), or "" for zero.
func fractionalNanos(nano int) string {
	if nano <= 0 {
		return ""
	}
	s := strconv.FormatInt(int64(nano), 10)
	if len(s) < 9 {
		s = strings.Repeat("0", 9-len(s)) + s
	}
	return "." + strings.TrimRight(s, "0")
}

func encodeDate(d Date) (json.RawMessage, error) {
	s := fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
	b, _ := json.Marshal(s)
	return json.RawMessage(fmt.Sprintf(`{"$type":"Date","_value":%s}`, b)), nil
}

func encodeLocalTime(t LocalTime) (json.RawMessage, error) {
	s := fmt.Sprintf("%02d:%02d:%02d", t.Hour, t.Minute, t.Second) + fractionalNanos(t.Nano)
	b, _ := json.Marshal(s)
	return json.RawMessage(fmt.Sprintf(`{"$type":"LocalTime","_value":%s}`, b)), nil
}

func encodeTime(t Time) (json.RawMessage, error) {
	// Fraction comes before the offset: HH:MM:SS.fff+05:30
	s := fmt.Sprintf("%02d:%02d:%02d", t.LocalTime.Hour, t.LocalTime.Minute, t.LocalTime.Second) +
		fractionalNanos(t.LocalTime.Nano) + t.Offset
	b, _ := json.Marshal(s)
	return json.RawMessage(fmt.Sprintf(`{"$type":"Time","_value":%s}`, b)), nil
}

func encodeLocalDateTime(ldt LocalDateTime) (json.RawMessage, error) {
	s := fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d", ldt.Date.Year, ldt.Date.Month, ldt.Date.Day, ldt.LocalTime.Hour, ldt.LocalTime.Minute, ldt.LocalTime.Second) +
		fractionalNanos(ldt.LocalTime.Nano)
	b, _ := json.Marshal(s)
	return json.RawMessage(fmt.Sprintf(`{"$type":"LocalDateTime","_value":%s}`, b)), nil
}

func encodeDateTime(dt DateTime) (json.RawMessage, error) {
	// Fraction before the offset; the IANA zone name is not part of the
	// wire format and is dropped (the offset carries the instant).
	s := fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d", dt.LocalDateTime.Date.Year, dt.LocalDateTime.Date.Month, dt.LocalDateTime.Date.Day, dt.LocalDateTime.LocalTime.Hour, dt.LocalDateTime.LocalTime.Minute, dt.LocalDateTime.LocalTime.Second) +
		fractionalNanos(dt.LocalDateTime.LocalTime.Nano) + dt.Offset
	b, _ := json.Marshal(s)
	return json.RawMessage(fmt.Sprintf(`{"$type":"DateTime","_value":%s}`, b)), nil
}

// Copyright (c) "Neo4j"
// Neo4j Sweden AB [http://neo4j.com]

package analytics

import "encoding/json"

// baseProperties is the one property attached to every event regardless of
// what the caller supplies. Every other property — identifiers, timing, OS
// info, geolocation opt-out, etc. — is the caller's responsibility to
// include via WithCommonProperties or Emit's per-call properties.
//
// MachineID deliberately has no omitempty: combineProperties relies on this
// key always being present in baseProperties' own map so it can overwrite
// any caller-supplied "machine_id" (protecting the invariant that it always
// wins) even when GetMachineID itself failed and returned "".
type baseProperties struct {
	MachineID string `json:"machine_id"`
}

// TrackEvent is a fully-formed Mixpanel event envelope.
type TrackEvent struct {
	Event      string `json:"event"`
	Properties any    `json:"properties"`
}

func (s *Service) getBaseProperties() baseProperties {
	return baseProperties{MachineID: s.machineID}
}

// toPropertiesMap converts any properties struct to map[string]any via JSON
// so it's compatible with the SDK without duplicating field mappings.
func toPropertiesMap(props any) (map[string]any, error) {
	if props == nil {
		return map[string]any{}, nil
	}
	b, err := json.Marshal(props)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// combineProperties merges three property layers into one map: common, then
// specific (specific overrides common on a shared key), then base applied
// last so it always wins — MachineID must not be overridable by caller-
// supplied data. common and specific may each be nil, in which case that
// layer is simply omitted.
func combineProperties(base baseProperties, common, specific any) (map[string]any, error) {
	result := map[string]any{}
	for _, layer := range []any{common, specific} {
		if layer == nil {
			continue
		}
		m, err := toPropertiesMap(layer)
		if err != nil {
			return nil, err
		}
		for k, v := range m {
			result[k] = v
		}
	}

	baseMap, err := toPropertiesMap(base)
	if err != nil {
		return nil, err
	}
	for k, v := range baseMap {
		result[k] = v
	}

	return result, nil
}

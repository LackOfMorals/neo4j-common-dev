// Copyright (c) "Neo4j"
// Neo4j Sweden AB [http://neo4j.com]

package analytics

import (
	"encoding/json"
	"log/slog"
	"runtime"
	"time"

	"github.com/google/uuid"
)

// baseProperties are the base properties attached to every Mixpanel event.
// DistinctID identifies a single run of the calling app (we don't track
// individual end users, so in practice this is one ID per process execution).
// InsertID is used by Mixpanel to deduplicate duplicate messages.
type baseProperties struct {
	Token      string `json:"token"`
	Time       int64  `json:"time"`
	DistinctID string `json:"distinct_id"`
	InsertID   string `json:"$insert_id"`
	Uptime     int64  `json:"uptime"`
	OS         string `json:"$os"`
	OSArch     string `json:"os_arch"`
	IsAura     bool   `json:"isAura"`
	IP         string `json:"$ip,omitempty"`
	MachineID  string `json:"machine_id,omitempty"`
	BinaryPath string `json:"binary_path,omitempty"`
}

// TrackEvent is a fully-formed Mixpanel event envelope.
type TrackEvent struct {
	Event      string `json:"event"`
	Properties any    `json:"properties"`
}

func (s *Service) getBaseProperties() baseProperties {
	uptime := time.Now().Unix() - s.startupTime
	return baseProperties{
		Token:      s.token,
		DistinctID: s.distinctID,
		Time:       time.Now().UnixMilli(),
		InsertID:   newInsertID(),
		Uptime:     uptime,
		OS:         runtime.GOOS,
		OSArch:     runtime.GOARCH,
		IsAura:     s.isAura,
		MachineID:  s.machineID,
		BinaryPath: s.binaryPath,
	}
}

func newInsertID() string {
	insertID, err := uuid.NewV6()
	if err != nil {
		slog.Error("Error while generating insert ID for analytics", "error", err.Error())
		return ""
	}
	return insertID.String()
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
// last so it always wins — base properties protect tracking integrity and
// must not be overridable by caller-supplied data. common and specific may
// each be nil, in which case that layer is simply omitted.
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

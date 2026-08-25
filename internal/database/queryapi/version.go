package queryapi

import (
	"strconv"
	"strings"
)

type VersionInfo struct {
	Neo4jVersion string `json:"neo4j_version"`
}

// ParseVersion parses a Neo4j version string.
// For CalVer YYYY.MM.PATCH it returns year, month, true.
// For SemVer MAJOR.MINOR.PATCH it returns major, minor, false.
func ParseVersion(v string) (int, int, bool) {
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return 0, 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, false
	}
	isCal := major >= 2000
	return major, minor, isCal
}

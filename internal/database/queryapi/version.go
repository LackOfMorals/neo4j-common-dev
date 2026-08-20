package queryapi

type VersionInfo struct {
	Neo4jVersion string `json:"neo4j_version"`
}

func ParseVersion(v string) (int, int, bool) {
	return 0,0,false
}

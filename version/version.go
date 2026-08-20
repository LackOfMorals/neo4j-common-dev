package version

import (
	"runtime/debug"
)

const fallback = "development"

var ClientVersion string = fallback

func init() {
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := resolveVersion(info, "github.com/LackOfMorals/neo4j-common"); v != "" {
			ClientVersion = v
		}
	}
}

func resolveVersion(info *debug.BuildInfo, modulePath string) string {
	for _, dep := range info.Deps {
		if dep.Path != modulePath {
			continue
		}
		version := dep.Version
		if dep.Replace != nil {
			version = dep.Replace.Version
		}
		if version == "(devel)" {
			return ""
		}
		return version
	}
	return ""
}

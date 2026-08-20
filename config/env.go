package config

import (
	"os"
	"strconv"
)

func GetEnv(key string) string {
	return os.Getenv(key)
}

func GetEnvWithDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func ParseBool(value string, defaultValue bool) bool {
	if value == "" {
		return defaultValue
	}
	b, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return b
}

func ParseInt32(value string, defaultValue int32) int32 {
	if value == "" {
		return defaultValue
	}
	i, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return defaultValue
	}
	return int32(i)
}

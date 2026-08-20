// Copyright (c) "Neo4j"
// Neo4j Sweden AB [http://neo4j.com]

package config_test

import (
	"testing"

	"github.com/LackOfMorals/neo4j-common/external/config"
)

func TestServiceRead(t *testing.T) {
	fields := []config.Field{
		{Key: "token", Flag: "token", EnvVar: "TEST_CONFIG_TOKEN", Description: "the token", Default: "default-token"},
		{Key: "debug", Flag: "debug", EnvVar: "TEST_CONFIG_DEBUG", Description: "enable debug logging", Default: "false"},
	}

	t.Run("flag wins over env and default", func(t *testing.T) {
		t.Setenv("TEST_CONFIG_TOKEN", "env-token")
		svc := config.New(fields...)
		values, err := svc.Read([]string{"--token", "flag-token"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := values.String("token"); got != "flag-token" {
			t.Errorf("unexpected token: got %q, want %q", got, "flag-token")
		}
	})

	t.Run("env wins over default when flag is not given", func(t *testing.T) {
		t.Setenv("TEST_CONFIG_TOKEN", "env-token")
		svc := config.New(fields...)
		values, err := svc.Read(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := values.String("token"); got != "env-token" {
			t.Errorf("unexpected token: got %q, want %q", got, "env-token")
		}
	})

	t.Run("default is used when neither flag nor env is set", func(t *testing.T) {
		svc := config.New(fields...)
		values, err := svc.Read(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := values.String("token"); got != "default-token" {
			t.Errorf("unexpected token: got %q, want %q", got, "default-token")
		}
	})

	t.Run("a flag explicitly set to empty string falls through to env/default", func(t *testing.T) {
		t.Setenv("TEST_CONFIG_TOKEN", "env-token")
		svc := config.New(fields...)
		values, err := svc.Read([]string{"--token", ""})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := values.String("token"); got != "env-token" {
			t.Errorf("unexpected token: got %q, want %q", got, "env-token")
		}
	})

	t.Run("fields with no Flag are only resolved from env/default", func(t *testing.T) {
		envOnly := []config.Field{{Key: "token", EnvVar: "TEST_CONFIG_TOKEN", Default: "default-token"}}
		t.Setenv("TEST_CONFIG_TOKEN", "env-token")
		svc := config.New(envOnly...)
		values, err := svc.Read([]string{"--token", "flag-token"})
		if err == nil {
			t.Fatalf("expected an error for an unregistered flag, got values: %v", values)
		}
	})

	t.Run("unknown key returns the empty string", func(t *testing.T) {
		svc := config.New(fields...)
		values, err := svc.Read(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := values.String("nonexistent"); got != "" {
			t.Errorf("expected empty string for unknown key, got %q", got)
		}
	})

	t.Run("Bool and Int32 parse resolved values", func(t *testing.T) {
		svc := config.New(fields...)
		values, err := svc.Read([]string{"--debug", "true"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !values.Bool("debug") {
			t.Errorf("expected debug to parse as true")
		}
	})
}

func TestValuesInt32(t *testing.T) {
	fields := []config.Field{{Key: "port", Flag: "port", Default: "7687"}}
	svc := config.New(fields...)

	values, err := svc.Read(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := values.Int32("port"); got != 7687 {
		t.Errorf("unexpected port: got %d, want %d", got, 7687)
	}
}

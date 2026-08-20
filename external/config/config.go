// Copyright (c) "Neo4j"
// Neo4j Sweden AB [http://neo4j.com]

package config

import (
	"flag"
	"fmt"
	"strings"
)

// Field describes one configuration value that can be supplied via an
// environment variable and/or a command-line flag. Key identifies the
// value in the Values returned by Service.Read. EnvVar and/or Flag may be
// left empty to skip that source for this field. Description is used to
// build the flag/env usage text.
type Field struct {
	Key         string
	EnvVar      string
	Flag        string
	Description string
	Default     string
}

// Values holds the fields resolved by Service.Read, keyed by each Field's Key.
type Values map[string]string

// String returns the raw resolved value for key, or "" if key is unknown.
func (v Values) String(key string) string { return v[key] }

// Bool parses the resolved value for key via ParseBool, defaulting to false.
func (v Values) Bool(key string) bool { return ParseBool(v[key], false) }

// Int32 parses the resolved value for key via ParseInt32, defaulting to 0.
func (v Values) Int32(key string) int32 { return ParseInt32(v[key], 0) }

// Service resolves a fixed set of Fields from command-line flags and
// environment variables.
type Service struct {
	fields []Field
}

// New creates a Service that resolves fields on Read.
func New(fields ...Field) *Service {
	return &Service{fields: fields}
}

// Read parses args as command-line flags (typically os.Args[1:]) and
// resolves every field's value. Precedence per field is: its command-line
// flag, then its environment variable, then its Default — an empty flag or
// env value is treated as not set, matching GetEnvWithDefault/ParseBool/
// ParseInt32 elsewhere in this package. Read returns flag.ErrHelp if args
// requested usage (-h/--help), matching flag.FlagSet's own behavior.
func (s *Service) Read(args []string) (Values, error) {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	fs.Usage = s.printUsage(fs)

	flagValues := make(map[string]*string, len(s.fields))
	for _, f := range s.fields {
		if f.Flag == "" {
			continue
		}
		flagValues[f.Key] = fs.String(f.Flag, "", f.Description)
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	values := make(Values, len(s.fields))
	for _, f := range s.fields {
		if v, ok := flagValues[f.Key]; ok && *v != "" {
			values[f.Key] = *v
			continue
		}
		if f.EnvVar != "" {
			if v := GetEnv(f.EnvVar); v != "" {
				values[f.Key] = v
				continue
			}
		}
		values[f.Key] = f.Default
	}
	return values, nil
}

// printUsage lists, for every field, its flag and/or env var alongside its
// description and default — richer than flag.FlagSet's own default usage,
// which only knows about flags.
func (s *Service) printUsage(fs *flag.FlagSet) func() {
	return func() {
		fmt.Fprintln(fs.Output(), "Usage:")
		for _, f := range s.fields {
			var sources []string
			if f.Flag != "" {
				sources = append(sources, "--"+f.Flag)
			}
			if f.EnvVar != "" {
				sources = append(sources, f.EnvVar)
			}
			fmt.Fprintf(fs.Output(), "  %s\n", strings.Join(sources, " / "))
			if f.Description != "" {
				fmt.Fprintf(fs.Output(), "\t%s\n", f.Description)
			}
			if f.Default != "" {
				fmt.Fprintf(fs.Output(), "\t(default %q)\n", f.Default)
			}
		}
	}
}

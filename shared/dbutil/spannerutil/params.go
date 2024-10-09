// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil

import (
	"os"
	"strings"

	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ConnParams contains arguments from a spanner URL.
type ConnParams struct {
	Host string

	Project  string
	Instance string
	Database string

	Emulator bool
}

// AllDefined returns whether project, instance and database are all defined.
func (params *ConnParams) AllDefined() bool {
	return params.Project != "" && params.Instance != "" && params.Database != ""
}

// ProjectPath returns "projects/<Project>".
func (params *ConnParams) ProjectPath() string {
	return "projects/" + params.Project
}

// InstancePath returns "projects/<Project>/instances/<Instance>".
func (params *ConnParams) InstancePath() string {
	return params.ProjectPath() + "/instances/" + params.Instance
}

// DatabasePath returns "projects/<Project>/instances/<Instance>/databases/<Database>".
func (params *ConnParams) DatabasePath() string {
	return params.InstancePath() + "/databases/" + params.Database
}

// ConnStr returns connection string.
func (params *ConnParams) ConnStr() string {
	s := "spanner://"
	if params.Host != "" {
		s += params.Host + "/"
	}
	s += params.DatabasePath()
	if params.Emulator {
		s += "?emulator"
	}
	return s
}

// GoSqlSpannerConnStr returns connection string for github.com/googleapis/go-sql-spanner.
func (params *ConnParams) GoSqlSpannerConnStr() string {
	var s string
	if params.Host != "" {
		s += params.Host + "/"
	}
	s += params.DatabasePath()
	if params.Emulator {
		s += "?usePlainText=true"
	}
	return s
}

// ClientOptions returns arguments for dialing spanner clients.
func (params *ConnParams) ClientOptions() (options []option.ClientOption) {
	if params.Host != "" {
		options = append(options, option.WithEndpoint(params.Host))
	}
	if params.Emulator {
		options = append(options,
			option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
			option.WithoutAuthentication(),
		)
	}

	return options
}

// ParseConnStr parses a spanner connection string to return the relevant pieces of the connection.
func ParseConnStr(full string) (params ConnParams, err error) {
	initial := full
	var ok bool
	full, ok = strings.CutPrefix(full, "spanner://")
	if !ok {
		return ConnParams{}, Error.New("invalid Spanner connection string %q", initial)
	}

	full, ok = strings.CutSuffix(full, "?emulator")
	if ok {
		params.Emulator = true
	}

	if !strings.HasPrefix(full, "projects") {
		// we'll assume it's a host instead

		before, after, _ := strings.Cut(full, "/")
		params.Host = before
		full = after
	}

	if params.Host == "" {
		params.Host = os.Getenv("SPANNER_EMULATOR_HOST")
		if params.Host != "" {
			params.Emulator = true
		}
	}

	// assume we are using an emulator when we are at home
	if strings.HasPrefix(params.Host, "localhost") || strings.HasPrefix(params.Host, "127.0.0.1") {
		params.Emulator = true
	}

	if full == "" {
		return params, nil
	}
	params.Project, full, ok = splitConnPathToken(full, "projects")
	if !ok {
		return params, Error.New("unable to parse project %q", initial)
	}

	if full == "" {
		return params, nil
	}
	params.Instance, full, ok = splitConnPathToken(full, "instances")
	if !ok {
		return params, Error.New("unable to parse instance %q", initial)
	}

	if full == "" {
		return params, nil
	}
	params.Database, full, ok = splitConnPathToken(full, "databases")
	if !ok {
		return params, Error.New("unable to parse database %q", initial)
	}

	if full != "" {
		return params, Error.New("url not fully parsed: %q", initial)
	}

	return params, nil
}

func splitConnPathToken(v, prefix string) (value, rest string, ok bool) {
	val, ok := strings.CutPrefix(v, prefix+"/")
	if !ok {
		return "", v, false
	}
	if val == "" {
		return "", v, false
	}

	value, rest, _ = strings.Cut(val, "/")
	if value == "" {
		return "", v, false
	}
	return value, rest, true
}

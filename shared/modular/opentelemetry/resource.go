// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package opentelemetry

import (
	"context"
	"errors"

	"github.com/zeebo/errs"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"

	"storj.io/common/identity"
	"storj.io/common/storj"
	"storj.io/common/version"
	"storj.io/storj/shared/modular"
)

// newResource creates the OTel resource, which identifies the process in all the exported logs and metrics.
// Empty values are not added, to let the SDK (or the collector) use its own defaults.
func newResource(ctx context.Context, cfg Config, info version.Info) (*resource.Resource, error) {
	var attrs []attribute.KeyValue
	if cfg.Namespace != "" {
		attrs = append(attrs, semconv.ServiceNamespace(cfg.Namespace))
	}
	if cfg.Service != "" {
		attrs = append(attrs, semconv.ServiceName(cfg.Service))
	}
	if cfg.InstanceID != "" {
		attrs = append(attrs, semconv.ServiceInstanceID(cfg.InstanceID))
	}
	if !info.Version.IsZero() {
		attrs = append(attrs, semconv.ServiceVersion(info.Version.VString()))
	}

	res, err := resource.New(ctx,
		resource.WithHost(),
		resource.WithAttributes(attrs...),
	)
	// a partial resource (for example the host name is not available) is still good enough.
	if err != nil && !errors.Is(err, resource.ErrPartialResource) {
		return nil, errs.Wrap(err)
	}
	return res, nil
}

// nodeIDOf returns the node ID of the configured identity, or an empty string if it's not available.
// Only the certificate chain is read (not the private key), as the ID is only used as a label.
func nodeIDOf(cfg modular.IdentityConfig) string {
	var id storj.NodeID
	var err error
	switch {
	case cfg.Cert != "":
		id, err = identity.NodeIDFromPEM([]byte(cfg.Cert))
	case cfg.CertPath != "":
		id, err = identity.NodeIDFromCertPath(cfg.CertPath)
	default:
		return ""
	}
	if err != nil {
		return ""
	}
	return id.String()
}

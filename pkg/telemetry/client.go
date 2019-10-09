// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package telemetry

import (
	"context"
	"os"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/admission/v2/admmonkit"
	"github.com/zeebo/admission/v2/admproto"
	"go.uber.org/zap"
)

const (
	// DefaultInterval is the default amount of time between metric payload sends
	DefaultInterval = time.Minute

	// DefaultPacketSize sets the target packet size. MTUs are often 1500,
	// though a good argument could be made for 512
	DefaultPacketSize = 1000

	// DefaultApplication is the default values for application name. Should be used
	// when value in ClientOpts.Application is not set and len(os.Args) == 0
	DefaultApplication = "unknown"
)

// ClientOpts allows you to set Client Options
type ClientOpts struct {
	// Interval is how frequently stats from the provided Registry will be
	// sent up. Note that this interval is "jittered", so the actual interval
	// is taken from a normal distribution with a mean of Interval and a
	// variance of Interval/4. Defaults to DefaultInterval
	Interval time.Duration

	// Application is the application name, usually prepended to metric names.
	// By default it will be os.Args[0]
	Application string

	// Instance is a string that identifies this particular server. Could be a
	// node id, but defaults to the result of DefaultInstanceId()
	Instance string

	// PacketSize controls how we fragment the data as it goes out in UDP
	// packets. Defaults to DefaultPacketSize
	PacketSize int

	// Registry is where to get stats from. Defaults to monkit.Default
	Registry *monkit.Registry

	// FloatEncoding is how floats should be encoded on the wire.
	// Default is float16.
	FloatEncoding admproto.FloatEncoding
}

// Client is a telemetry client for sending UDP packets at a regular interval
// from a monkit.Registry
type Client struct {
	log      *zap.Logger
	interval time.Duration
	opts     admmonkit.Options
	send     func(context.Context, admmonkit.Options) error
}

// NewClient constructs a telemetry client that sends packets to remoteAddr
// over UDP.
func NewClient(log *zap.Logger, remoteAddr string, opts ClientOpts) (rv *Client, err error) {
	if opts.Interval == 0 {
		opts.Interval = DefaultInterval
	}
	if opts.Application == "" {
		if len(os.Args) > 0 {
			opts.Application = os.Args[0]
		} else {
			// what the actual heck
			opts.Application = DefaultApplication
		}
	}
	if opts.Instance == "" {
		opts.Instance = DefaultInstanceID()
	}
	if opts.Registry == nil {
		opts.Registry = monkit.Default
	}
	if opts.PacketSize == 0 {
		opts.PacketSize = DefaultPacketSize
	}

	return &Client{
		log:      log,
		interval: opts.Interval,
		send:     admmonkit.Send,
		opts: admmonkit.Options{
			Application: opts.Application,
			InstanceId:  []byte(opts.Instance),
			Address:     remoteAddr,
			PacketSize:  opts.PacketSize,
			Registry:    opts.Registry,
			ProtoOpts:   admproto.Options{FloatEncoding: opts.FloatEncoding},
		},
	}, nil
}

// Run calls Report roughly every Interval
func (c *Client) Run(ctx context.Context) {
	c.log.Sugar().Debugf("Initialized batcher with id = %q", c.opts.InstanceId)
	for {
		time.Sleep(jitter(c.interval))
		if ctx.Err() != nil {
			return
		}

		err := c.Report(ctx)
		if err != nil {
			c.log.Sugar().Errorf("failed sending report: %v", err)
		}
	}
}

// Report bundles up all the current stats and writes them out as UDP packets
func (c *Client) Report(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return c.send(ctx, c.opts)
}

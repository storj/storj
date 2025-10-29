// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"
	"encoding/base64"
	"math/rand"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spf13/pflag"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/storagenode/trust"
)

var (
	mon = monkit.Package()

	// Error is the default error class for contact package.
	Error = errs.Class("contact")

	errPingSatellite = errs.Class("ping satellite")
)

const initialBackOff = time.Second

// Config contains configurable values for contact service.
type Config struct {
	ExternalAddress string `user:"true" help:"the public address of the node, useful for nodes behind NAT" default:""`

	// Chore config values
	Interval       time.Duration `help:"how frequently the node contact chore should run" releaseDefault:"1h" devDefault:"30s"`
	CheckInTimeout time.Duration `help:"timeout for the check-in request" releaseDefault:"10m" devDefault:"15s" testDefault:"5s"`

	Tags           SignedTags `help:"protobuf serialized signed node tags in hex (base64) format"`
	SelfSignedTags []string   `help:"coma separated key=value pairs, which will be self signed and used as tags"`
}

// SignedTags represents base64 encoded signed tags.
type SignedTags pb.SignedNodeTagSets

// Type implements pflag.Value interface.
func (u *SignedTags) Type() string {
	return "signedtags"
}

// String implements pflag.Value interface.
func (u *SignedTags) String() string {
	if u == nil {
		return ""
	}
	p := pb.SignedNodeTagSets(*u)
	raw, err := pb.Marshal(&p)
	if err != nil {
		return err.Error()
	}
	return base64.StdEncoding.EncodeToString(raw)
}

// Set implements flag.Value interface.
func (u *SignedTags) Set(s string) error {
	p := pb.SignedNodeTagSets{}
	for i, part := range strings.Split(s, ",") {
		if s == "" {
			return nil
		}
		if u == nil {
			return nil
		}
		raw, err := base64.StdEncoding.DecodeString(part)
		if err != nil {
			return errs.New("signed tag configuration #%d is not base64 encoded: %s", i+1, s)
		}
		err = pb.Unmarshal(raw, &p)
		if err != nil {
			return errs.New("signed tag configuration #%d is not a pb.SignedNodeTagSets{}: %s", i+1, s)
		}
		u.Tags = append(u.Tags, p.Tags...)
	}
	return nil
}

var _ pflag.Value = &SignedTags{}

// NodeInfo contains information necessary for introducing storagenode to satellite.
type NodeInfo struct {
	ID                  storj.NodeID
	Address             string
	Version             pb.NodeVersion
	Capacity            pb.NodeCapacity
	Operator            pb.NodeOperator
	NoiseKeyAttestation *pb.NoiseKeyAttestation
	DebounceLimit       int
	FastOpen            bool
	HashstoreWriteToNew func() bool
	HashstoreMemtbl     bool
}

// Service is the contact service between storage nodes and satellites.
type Service struct {
	log    *zap.Logger
	rand   *rand.Rand
	dialer rpc.Dialer

	mu               sync.Mutex
	self             NodeInfo
	checkinCallbacks []func(context.Context, storj.NodeID, *pb.CheckInResponse) error

	trust     trust.TrustedSatelliteSource
	quicStats *QUICStats

	initialized sync2.Fence

	tags *pb.SignedNodeTagSets
}

// NewService creates a new contact service.
func NewService(log *zap.Logger, dialer rpc.Dialer, self NodeInfo, trust trust.TrustedSatelliteSource, quicStats *QUICStats, tags *pb.SignedNodeTagSets) *Service {
	return &Service{
		log:       log,
		rand:      rand.New(rand.NewSource(time.Now().UnixNano())),
		dialer:    dialer,
		trust:     trust,
		self:      self,
		quicStats: quicStats,
		tags:      tags,
	}
}

// RegisterCheckinCallback registers a checkin callback.
func (service *Service) RegisterCheckinCallback(cb func(ctx context.Context, satellite storj.NodeID, resp *pb.CheckInResponse) error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.checkinCallbacks = append(service.checkinCallbacks, cb)
}

// PingSatellites attempts to ping all satellites in trusted list until backoff reaches maxInterval.
func (service *Service) PingSatellites(ctx context.Context, maxInterval, timeout time.Duration) (err error) {
	defer mon.Task()(&ctx)(&err)
	satellites := service.trust.GetSatellites(ctx)
	var group errgroup.Group
	for _, satellite := range satellites {
		satellite := satellite
		group.Go(func() error {
			return service.pingSatellite(ctx, satellite, maxInterval, timeout)
		})
	}
	return group.Wait()
}

func (service *Service) pingSatellite(ctx context.Context, satellite storj.NodeID, maxInterval, timeout time.Duration) error {
	interval := initialBackOff
	attempts := 0
	for {
		mon.Meter("satellite_contact_request").Mark(1)

		err := service.pingSatelliteOnce(ctx, satellite, timeout)
		attempts++
		if err == nil {
			return nil
		}
		service.log.Error("ping satellite failed ", zap.Stringer("Satellite ID", satellite), zap.Int("attempts", attempts), zap.Error(err))

		// Sleeps until interval times out, then continue. Returns if context is cancelled.
		if !sync2.Sleep(ctx, interval) {
			service.log.Info("context cancelled", zap.Stringer("Satellite ID", satellite))
			return nil
		}
		interval *= 2
		if interval >= maxInterval {
			service.log.Info("retries timed out for this cycle", zap.Stringer("Satellite ID", satellite))
			return nil
		}
	}
}

func (service *Service) pingSatelliteOnce(ctx context.Context, id storj.NodeID, timeout time.Duration) (err error) {
	defer mon.Task()(&ctx, id)(&err)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := service.dialSatellite(ctx, id)
	if err != nil {
		return errPingSatellite.Wrap(err)
	}
	defer func() { err = errs.Combine(err, conn.Close()) }()

	self := service.Local()
	var features uint64
	if self.FastOpen {
		features |= uint64(pb.NodeAddress_TCP_FASTOPEN_ENABLED)
	}
	if self.HashstoreWriteToNew != nil && self.HashstoreWriteToNew() {
		features |= uint64(pb.CheckInRequest_HASHSTORE_FOR_NEW)
	}
	if self.HashstoreMemtbl {
		features |= uint64(pb.CheckInRequest_HASHSTORE_MEMTBL)
	}

	mon.IntVal("reported_capacity").Observe(self.Capacity.FreeDisk)

	resp, err := pb.NewDRPCNodeClient(conn).CheckIn(ctx, &pb.CheckInRequest{
		Address:             self.Address,
		Version:             &self.Version,
		Capacity:            &self.Capacity,
		Operator:            &self.Operator,
		NoiseKeyAttestation: self.NoiseKeyAttestation,
		DebounceLimit:       int32(self.DebounceLimit),
		Features:            features,
		SignedTags:          service.tags,
	})
	service.quicStats.SetStatus(false)
	if err != nil {
		return errPingSatellite.Wrap(err)
	}
	service.quicStats.SetStatus(resp.PingNodeSuccessQuic)

	if !resp.PingNodeSuccess {
		return errPingSatellite.New("%s", resp.PingErrorMessage)
	}

	if resp.PingErrorMessage != "" {
		service.log.Warn("Your node is still considered to be online but encountered an error.", zap.Stringer("Satellite ID", id), zap.String("Error", resp.GetPingErrorMessage()))
	}

	service.mu.Lock()
	checkinCallbacks := slices.Clone(service.checkinCallbacks)
	service.mu.Unlock()

	for _, cb := range checkinCallbacks {
		err = cb(ctx, id, resp)
		if err != nil {
			service.log.Error("checkin callback failed", zap.Error(err))
		}
	}

	return nil
}

// RequestPingMeQUIC sends pings request to satellite for a pingBack via QUIC.
func (service *Service) RequestPingMeQUIC(ctx context.Context) (stats *QUICStats, err error) {
	defer mon.Task()(&ctx)(&err)

	stats = NewQUICStats(true)

	satellites := service.trust.GetSatellites(ctx)
	if len(satellites) < 1 {
		return nil, errPingSatellite.New("no trusted satellite available")
	}

	// Shuffle the satellites
	// All the Storagenodes get a default list of trusted satellites (The Storj ones) and
	// most of the SN operators don't change the list, hence if it always starts with
	// the same satellite we are going to put always more pressure on the first trusted
	// satellite on the list. So we iterate over the list of trusted satellites in a
	// random order to avoid putting pressure on the first trusted on the list
	service.rand.Shuffle(len(satellites), func(i, j int) {
		satellites[i], satellites[j] = satellites[j], satellites[i]
	})

	for _, satellite := range satellites {
		err = service.requestPingMeOnce(ctx, satellite)
		if err != nil {
			stats.SetStatus(false)
			// log warning and try the next trusted satellite
			service.log.Warn("failed PingMe request to satellite", zap.Stringer("Satellite ID", satellite), zap.Error(err))
			continue
		}

		stats.SetStatus(true)

		return stats, nil
	}

	return stats, errPingSatellite.New("failed to ping storage node using QUIC: %q", err)
}

func (service *Service) requestPingMeOnce(ctx context.Context, satellite storj.NodeID) (err error) {
	defer mon.Task()(&ctx, satellite)(&err)

	conn, err := service.dialSatellite(ctx, satellite)
	if err != nil {
		return errPingSatellite.Wrap(err)
	}
	defer func() { err = errs.Combine(err, conn.Close()) }()

	node := service.Local()
	_, err = pb.NewDRPCNodeClient(conn).PingMe(ctx, &pb.PingMeRequest{
		Address:   node.Address,
		Transport: pb.NodeTransport_QUIC_RPC,
	})
	if err != nil {
		return errPingSatellite.Wrap(err)
	}

	return nil
}

func (service *Service) dialSatellite(ctx context.Context, id storj.NodeID) (*rpc.Conn, error) {
	nodeurl, err := service.trust.GetNodeURL(ctx, id)
	if err != nil {
		return nil, errPingSatellite.Wrap(err)
	}

	return service.dialer.DialNodeURL(ctx, nodeurl)
}

// Local returns the storagenode info.
func (service *Service) Local() NodeInfo {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.self
}

// UpdateSelf updates the local node with the capacity.
func (service *Service) UpdateSelf(capacity *pb.NodeCapacity) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if capacity != nil {
		service.self.Capacity = *capacity
	}
	service.initialized.Release()
}

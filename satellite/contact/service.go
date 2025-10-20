// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/rpc/quic"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/version"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/shared/nodetag"
)

// HashstoreRolloutSettings are a flag config struct for use with rolling out
// Hashstore settings to nodes connected to this satellite.
type HashstoreRolloutSettings struct {
	ActiveMigrate  bool `default:"false"`
	PassiveMigrate bool `default:"false"`
	WriteToNew     bool `default:"false"`
	ReadNewFirst   bool `default:"false"`
	TTLToNew       bool `default:"false"`
}

// ToProto converts this to the protocol buffer form.
func (settings HashstoreRolloutSettings) ToProto() *pb.HashstoreSettings {
	return &pb.HashstoreSettings{
		ActiveMigrate:  settings.ActiveMigrate,
		PassiveMigrate: settings.PassiveMigrate,
		WriteToNew:     settings.WriteToNew,
		ReadNewFirst:   settings.ReadNewFirst,
		TtlToNew:       settings.TTLToNew,
	}
}

// Config contains configurable values for contact service.
type Config struct {
	ExternalAddress string        `user:"true" help:"the public address of the node, useful for nodes behind NAT" default:""`
	Timeout         time.Duration `help:"timeout for pinging storage nodes" default:"10m0s" testDefault:"1m"`
	AllowPrivateIP  bool          `help:"allow private IPs in CheckIn and PingMe" testDefault:"true" devDefault:"true" default:"false"`

	RateLimitInterval  time.Duration `help:"the amount of time that should happen between contact attempts usually" releaseDefault:"10m0s" devDefault:"1ns"`
	RateLimitBurst     int           `help:"the maximum burst size for the contact rate limit token bucket" releaseDefault:"2" devDefault:"1000"`
	RateLimitCacheSize int           `help:"the number of nodes or addresses to keep token buckets for" default:"1000"`

	HashstoreRollout struct {
		Seed    string  `help:"the hashstore rollout seed" default:""`
		Cursor  float64 `help:"the hashstore rollout cursor (between 0 and 1)" default:"0"`
		Current HashstoreRolloutSettings
		Next    HashstoreRolloutSettings
	}
}

// Service is the contact service between storage nodes and satellites.
// It is responsible for updating general node information like address and capacity.
// It is also responsible for updating peer identity information for verifying signatures from that node.
//
// architecture: Service
type Service struct {
	log *zap.Logger

	overlay *overlay.Service
	peerIDs overlay.PeerIdentities
	dialer  rpc.Dialer

	timeout        time.Duration
	idLimiter      *RateLimiter
	allowPrivateIP bool

	nodeTagAuthority nodetag.Authority
	config           Config
}

// NewService creates a new contact service.
func NewService(log *zap.Logger, overlay *overlay.Service, peerIDs overlay.PeerIdentities, dialer rpc.Dialer, authority nodetag.Authority, config Config) *Service {
	return &Service{
		log:              log,
		overlay:          overlay,
		peerIDs:          peerIDs,
		dialer:           dialer,
		timeout:          config.Timeout,
		idLimiter:        NewRateLimiter(config.RateLimitInterval, config.RateLimitBurst, config.RateLimitCacheSize),
		allowPrivateIP:   config.AllowPrivateIP,
		nodeTagAuthority: authority,
		config:           config,
	}
}

// Close closes resources.
func (service *Service) Close() error { return nil }

// PingBack pings the node to test connectivity.
func (service *Service) PingBack(ctx context.Context, nodeurl storj.NodeURL) (_ bool, _ bool, _ string, err error) {
	defer mon.Task()(&ctx)(&err)

	if service.timeout > 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, service.timeout)
		defer cancel()
	}

	pingNodeSuccess := true
	var pingErrorMessage string
	var pingNodeSuccessQUIC bool

	client, err := dialNodeURL(ctx, service.dialer, nodeurl)
	if err != nil {
		// If there is an error from trying to dial and ping the node, return that error as
		// pingErrorMessage and not as the err. We want to use this info to update
		// node contact info and do not want to terminate execution by returning an err
		mon.Event("failed_dial")
		pingNodeSuccess = false
		pingErrorMessage = fmt.Sprintf("failed to dial storage node (ID: %s) at address %s: %q",
			nodeurl.ID, nodeurl.Address, err,
		)
		service.log.Debug("pingBack failed to dial storage node",
			zap.String("pingErrorMessage", pingErrorMessage),
		)
		return pingNodeSuccess, pingNodeSuccessQUIC, pingErrorMessage, nil
	}
	defer func() { err = errs.Combine(err, client.Close()) }()

	_, err = client.pingNode(ctx, &pb.ContactPingRequest{})
	if err != nil {
		mon.Event("failed_ping_node")
		pingNodeSuccess = false
		pingErrorMessage = fmt.Sprintf("failed to ping storage node, your node indicated error code: %d, %q", rpcstatus.Code(err), err)
		service.log.Debug("pingBack pingNode error",
			zap.Stringer("Node ID", nodeurl.ID),
			zap.String("pingErrorMessage", pingErrorMessage),
		)

		return pingNodeSuccess, pingNodeSuccessQUIC, pingErrorMessage, nil
	}

	pingNodeSuccessQUIC = true
	err = service.pingNodeQUIC(ctx, nodeurl)
	if err != nil {
		// udp ping back is optional right now, it shouldn't affect contact service's
		// control flow
		pingNodeSuccessQUIC = false
		pingErrorMessage = err.Error()
	}

	return pingNodeSuccess, pingNodeSuccessQUIC, pingErrorMessage, nil
}

func (service *Service) pingNodeQUIC(ctx context.Context, nodeurl storj.NodeURL) error {
	udpDialer := service.dialer
	udpDialer.Connector = quic.NewDefaultConnector(nil)
	udpClient, err := dialNodeURL(ctx, udpDialer, nodeurl)
	if err != nil {
		mon.Event("failed_dial_quic")
		return Error.New("failed to dial storage node (ID: %s) at address %s using QUIC: %q", nodeurl.ID.String(), nodeurl.Address, err)
	}
	defer func() {
		_ = udpClient.Close()
	}()

	_, err = udpClient.pingNode(ctx, &pb.ContactPingRequest{})
	if err != nil {
		mon.Event("failed_ping_node_quic")
		return Error.New("failed to ping storage node using QUIC, your node indicated error code: %d, %q", rpcstatus.Code(err), err)
	}

	return nil
}

func (service *Service) processNodeTags(ctx context.Context, nodeID storj.NodeID, self signing.Signee, req *pb.SignedNodeTagSets) error {
	if req != nil {
		tags := nodeselection.NodeTags{}
		for _, t := range req.Tags {
			verifiedTags, signerID, err := verifyTags(ctx, append(service.nodeTagAuthority, self), nodeID, t)
			if err != nil {
				service.log.Info("Failed to verify tags.", zap.Error(err), zap.Stringer("NodeID", nodeID))
				continue
			}

			ts := time.Unix(verifiedTags.SignedAt, 0)
			for _, vt := range verifiedTags.Tags {
				tags = append(tags, nodeselection.NodeTag{
					NodeID:   nodeID,
					Name:     vt.Name,
					Value:    vt.Value,
					SignedAt: ts,
					Signer:   signerID,
				})
			}
		}
		if len(tags) > 0 {
			err := service.overlay.UpdateNodeTags(ctx, tags)
			if err != nil {
				return Error.Wrap(err)
			}
		}
	}
	return nil
}

func (service *Service) getHashstoreSettings(ctx context.Context, nodeID storj.NodeID) (settings *pb.HashstoreSettings, err error) {
	rollout := version.PercentageToCursorF(service.config.HashstoreRollout.Cursor)

	hash := hmac.New(sha256.New, []byte(service.config.HashstoreRollout.Seed))
	_, err = hash.Write(nodeID[:])
	if err != nil {
		return nil, err
	}

	if bytes.Compare(hash.Sum(nil), rollout[:]) <= 0 {
		return service.config.HashstoreRollout.Next.ToProto(), nil
	}

	return service.config.HashstoreRollout.Current.ToProto(), nil
}

func verifyTags(ctx context.Context, authority nodetag.Authority, nodeID storj.NodeID, t *pb.SignedNodeTagSet) (*pb.NodeTagSet, storj.NodeID, error) {
	signerID, err := storj.NodeIDFromBytes(t.SignerNodeId)
	if err != nil {
		return nil, signerID, errs.New("failed to parse signerNodeID from verifiedTags: '%x', %s", t.SignerNodeId, err.Error())
	}

	verifiedTags, err := authority.Verify(ctx, t)
	if err != nil {
		return nil, signerID, errs.New("received node tags with wrong/unknown signature: '%x', %s", t.Signature, err.Error())
	}

	signedNodeID, err := storj.NodeIDFromBytes(verifiedTags.NodeId)
	if err != nil {
		return nil, signerID, errs.New("failed to parse nodeID from verifiedTags: '%x', %s", verifiedTags.NodeId, err.Error())
	}

	if signedNodeID != nodeID {
		return nil, signerID, errs.New("the tag is signed for a different node. Expected NodeID: '%s', Received NodeID: '%s'", nodeID, signedNodeID)
	}
	return verifiedTags, signerID, nil
}

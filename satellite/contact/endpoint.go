// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"
	"net"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/rpc/noise"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/drpc/drpcctx"
	"storj.io/eventkit"
	"storj.io/storj/private/nodeoperator"
	"storj.io/storj/satellite/overlay"
)

var (
	errPingBackDial     = errs.Class("pingback dialing")
	errCheckInIdentity  = errs.Class("check-in identity")
	errCheckInRateLimit = errs.Class("check-in ratelimit")
	errCheckInNetwork   = errs.Class("check-in network")
)

// Endpoint implements the contact service Endpoints.
type Endpoint struct {
	pb.DRPCNodeUnimplementedServer
	log     *zap.Logger
	service *Service
}

// NewEndpoint returns a new contact service endpoint.
func NewEndpoint(log *zap.Logger, service *Service) *Endpoint {
	return &Endpoint{
		log:     log,
		service: service,
	}
}

// CheckIn is periodically called by storage nodes to keep the satellite informed of its existence,
// address, and operator information. In return, this satellite keeps the node informed of its
// reachability.
// When a node checks-in with the satellite, the satellite pings the node back to confirm they can
// successfully connect.
func (endpoint *Endpoint) CheckIn(ctx context.Context, req *pb.CheckInRequest) (_ *pb.CheckInResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peerID, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		endpoint.log.Info("failed to get node ID from context", zap.String("node address", req.Address), zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Unknown, errCheckInIdentity.New("failed to get ID from context: %v", err).Error())
	}
	nodeID := peerID.ID

	// we need a string as a key for the limiter, but nodeID.String() has base58 encoding overhead
	nodeIDBytesAsString := string(nodeID.Bytes())
	if !endpoint.service.idLimiter.IsAllowed(ctx, nodeIDBytesAsString) {
		endpoint.log.Info("node rate limited by id", zap.String("node address", req.Address), zap.Stringer("Node ID", nodeID))
		return nil, rpcstatus.Error(rpcstatus.ResourceExhausted, errCheckInRateLimit.New("node rate limited by id").Error())
	}

	err = endpoint.service.peerIDs.Set(ctx, nodeID, peerID)
	if err != nil {
		endpoint.log.Info("failed to add peer identity entry for ID", zap.String("node address", req.Address), zap.Stringer("Node ID", nodeID), zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.FailedPrecondition, errCheckInIdentity.New("failed to add peer identity entry for ID: %v", err).Error())
	}

	resolvedIP, port, resolvedNetwork, err := endpoint.service.overlay.ResolveIPAndNetwork(ctx, req.Address)
	if err != nil {
		endpoint.log.Info("failed to resolve IP from address", zap.String("node address", req.Address), zap.Stringer("Node ID", nodeID), zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, errCheckInNetwork.New("failed to resolve IP from address: %s, err: %v", req.Address, err).Error())
	}
	if !endpoint.service.allowPrivateIP && (!resolvedIP.IsGlobalUnicast() || isPrivateIP(resolvedIP)) {
		endpoint.log.Info("IP address not allowed", zap.String("node address", req.Address), zap.Stringer("Node ID", nodeID))
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, errCheckInNetwork.New("IP address not allowed: %s", req.Address).Error())
	}

	nodeurl := storj.NodeURL{
		ID:      nodeID,
		Address: req.Address,
	}

	var noiseInfo *pb.NoiseInfo
	if req.NoiseKeyAttestation != nil {
		if err := noise.ValidateKeyAttestation(ctx, req.NoiseKeyAttestation, nodeID); err == nil {
			noiseInfo = &pb.NoiseInfo{
				Proto:     req.NoiseKeyAttestation.NoiseProto,
				PublicKey: req.NoiseKeyAttestation.NoisePublicKey,
			}
			nodeurl.NoiseInfo = noiseInfo.Convert()
		}
	}

	pingNodeSuccess, pingNodeSuccessQUIC, pingErrorMessage, err := endpoint.service.PingBack(ctx, nodeurl)
	if err != nil {
		return nil, endpoint.checkPingRPCErr(err, nodeurl)
	}

	// check wallet features
	if req.Operator != nil {
		if err := nodeoperator.DefaultWalletFeaturesValidation.Validate(req.Operator.WalletFeatures); err != nil {
			endpoint.log.Debug("ignoring invalid wallet features",
				zap.Stringer("Node ID", nodeID),
				zap.Strings("Wallet Features", req.Operator.WalletFeatures))

			// TODO: Update CheckInResponse to include wallet feature validation error
			req.Operator.WalletFeatures = nil
		}
	}
	err = endpoint.service.processNodeTags(ctx, nodeID, signing.SigneeFromPeerIdentity(peerID), req.SignedTags)
	if err != nil {
		endpoint.log.Info("failed to update node tags", zap.String("node address", req.Address), zap.Stringer("Node ID", nodeID), zap.Error(err))
	}

	nodeInfo := overlay.NodeCheckInInfo{
		NodeID: peerID.ID,
		Address: &pb.NodeAddress{
			Address:       req.Address,
			NoiseInfo:     noiseInfo,
			DebounceLimit: req.DebounceLimit,
			Features:      req.Features,
		},
		LastNet:    resolvedNetwork,
		LastIPPort: net.JoinHostPort(resolvedIP.String(), port),
		IsUp:       pingNodeSuccess,
		Capacity:   req.Capacity,
		Operator:   req.Operator,
		Version:    req.Version,
	}

	emitEventkitEvent(ctx, req, pingNodeSuccess, pingNodeSuccessQUIC, nodeInfo)

	err = endpoint.service.overlay.UpdateCheckIn(ctx, nodeInfo, time.Now().UTC())
	if err != nil {
		endpoint.log.Info("failed to update check in", zap.String("node address", req.Address), zap.Stringer("Node ID", nodeID), zap.Error(err))
		endpoint.service.idLimiter.BackOut(ctx, nodeIDBytesAsString)
		return nil, rpcstatus.Error(rpcstatus.Internal, Error.Wrap(err).Error())
	}

	hashstoreSettings, err := endpoint.service.getHashstoreSettings(ctx, nodeID)
	if err != nil {
		endpoint.log.Info("failed to get hashstore settings", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, Error.Wrap(err).Error())
	}

	endpoint.log.Debug("checking in", zap.Stringer("Node ID", nodeID), zap.String("node addr", req.Address), zap.Bool("ping node success", pingNodeSuccess), zap.String("ping node err msg", pingErrorMessage))
	return &pb.CheckInResponse{
		PingNodeSuccess:     pingNodeSuccess,
		PingNodeSuccessQuic: pingNodeSuccessQUIC,
		PingErrorMessage:    pingErrorMessage,
		HashstoreSettings:   hashstoreSettings,
	}, nil
}

func emitEventkitEvent(ctx context.Context, req *pb.CheckInRequest, pingNodeTCPSuccess bool, pingNodeQUICSuccess bool, nodeInfo overlay.NodeCheckInInfo) {
	var sourceAddr string
	transport, found := drpcctx.Transport(ctx)
	if found {
		if conn, ok := transport.(net.Conn); ok {
			a := conn.RemoteAddr()
			if a != nil {
				sourceAddr = a.String()
			}
		}
	}

	tags := []eventkit.Tag{
		eventkit.String("id", nodeInfo.NodeID.String()),
		eventkit.String("addr", req.Address),
		eventkit.String("resolved-addr", nodeInfo.LastIPPort),
		eventkit.String("source-addr", sourceAddr),
		eventkit.String("country", nodeInfo.CountryCode.String()),
		eventkit.Bool("ping-tpc-success", pingNodeTCPSuccess),
		eventkit.Bool("ping-quic-success", pingNodeQUICSuccess),
	}

	if nodeInfo.Capacity != nil {
		tags = append(tags, eventkit.Int64("free-disk", nodeInfo.Capacity.FreeDisk))
	}

	if nodeInfo.Version != nil {
		tags = append(tags, eventkit.Timestamp("build-time", nodeInfo.Version.Timestamp))
		tags = append(tags, eventkit.String("version", nodeInfo.Version.Version))
	}

	ek.Event("checkin", tags...)
}

// GetTime returns current timestamp.
func (endpoint *Endpoint) GetTime(ctx context.Context, req *pb.GetTimeRequest) (_ *pb.GetTimeResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peerID, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		endpoint.log.Info("failed to get node ID from context", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Unauthenticated, errCheckInIdentity.New("failed to get ID from context: %v", err).Error())
	}

	currentTimestamp := time.Now().UTC()
	endpoint.log.Debug("get system current time", zap.Stringer("timestamp", currentTimestamp), zap.Stringer("node id", peerID.ID))
	return &pb.GetTimeResponse{
		Timestamp: currentTimestamp,
	}, nil
}

// PingMe is called by storage node to request a pingBack from the satellite to confirm they can
// successfully connect to the node.
func (endpoint *Endpoint) PingMe(ctx context.Context, req *pb.PingMeRequest) (_ *pb.PingMeResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peerID, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		endpoint.log.Info("failed to get node ID from context", zap.String("node address", req.Address), zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Unknown, errCheckInIdentity.New("failed to get ID from context: %v", err).Error())
	}
	nodeID := peerID.ID

	nodeURL := storj.NodeURL{
		ID:      nodeID,
		Address: req.Address,
	}

	resolvedIP, _, _, err := endpoint.service.overlay.ResolveIPAndNetwork(ctx, req.Address)
	if err != nil {
		endpoint.log.Info("failed to resolve IP from address", zap.String("node address", req.Address), zap.Stringer("Node ID", nodeID), zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, errCheckInNetwork.New("failed to resolve IP from address: %s, err: %v", req.Address, err).Error())
	}
	if !endpoint.service.allowPrivateIP && (!resolvedIP.IsGlobalUnicast() || isPrivateIP(resolvedIP)) {
		endpoint.log.Info("IP address not allowed", zap.String("node address", req.Address), zap.Stringer("Node ID", nodeID))
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, errCheckInNetwork.New("IP address not allowed: %s", req.Address).Error())
	}

	if endpoint.service.timeout > 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, endpoint.service.timeout)
		defer cancel()
	}

	switch req.Transport {

	case pb.NodeTransport_QUIC_RPC:
		err = endpoint.service.pingNodeQUIC(ctx, nodeURL)
		if err != nil {
			return nil, endpoint.checkPingRPCErr(err, nodeURL)
		}
		return &pb.PingMeResponse{}, nil

	case pb.NodeTransport_TCP_TLS_RPC:
		client, err := dialNodeURL(ctx, endpoint.service.dialer, nodeURL)
		if err != nil {
			return nil, endpoint.checkPingRPCErr(err, nodeURL)
		}

		defer func() { err = errs.Combine(err, client.Close()) }()

		_, err = client.pingNode(ctx, &pb.ContactPingRequest{})
		if err != nil {
			return nil, endpoint.checkPingRPCErr(err, nodeURL)
		}
		return &pb.PingMeResponse{}, nil
	}

	return nil, rpcstatus.Errorf(rpcstatus.InvalidArgument, "invalid transport: %v", req.Transport)
}

func (endpoint *Endpoint) checkPingRPCErr(err error, nodeURL storj.NodeURL) error {
	endpoint.log.Info("failed to ping back address", zap.String("node address", nodeURL.Address), zap.Stringer("Node ID", nodeURL.ID), zap.Error(err))
	if errPingBackDial.Has(err) {
		err = errCheckInNetwork.New("failed dialing address when attempting to ping node (ID: %s): %s, err: %v", nodeURL.ID, nodeURL.Address, err)
		return rpcstatus.Error(rpcstatus.NotFound, err.Error())
	}
	err = errCheckInNetwork.New("failed to ping node (ID: %s) at address: %s, err: %v", nodeURL.ID, nodeURL.Address, err)
	return rpcstatus.Error(rpcstatus.NotFound, err.Error())
}

// isPrivateIP is copied Go 1.17's net.IP.IsPrivate. We copied it to ensure we
// can compile for the Go version earlier than 1.17.
//
// TODO(artur): Swap isPrivateIP usages with net.IP.IsPrivate when we no longer
// need to build for earlier than Go 1.17. Keep this in sync with stdlib until.
func isPrivateIP(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		// Following RFC 1918, Section 3. Private Address Space which says:
		//   The Internet Assigned Numbers Authority (IANA) has reserved the
		//   following three blocks of the IP address space for private internets:
		//     10.0.0.0        -   10.255.255.255  (10/8 prefix)
		//     172.16.0.0      -   172.31.255.255  (172.16/12 prefix)
		//     192.168.0.0     -   192.168.255.255 (192.168/16 prefix)
		return ip4[0] == 10 ||
			(ip4[0] == 172 && ip4[1]&0xf0 == 16) ||
			(ip4[0] == 192 && ip4[1] == 168)
	}
	// Following RFC 4193, Section 8. IANA Considerations which says:
	//   The IANA has assigned the FC00::/7 prefix to "Unique Local Unicast".
	return len(ip) == net.IPv6len && ip[0]&0xfe == 0xfc
}

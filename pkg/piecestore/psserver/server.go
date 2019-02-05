// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package psserver

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/sha512"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/mr-tron/base58/base58"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls"
	pstore "storj.io/storj/pkg/piecestore"
	"storj.io/storj/pkg/piecestore/psserver/psdb"
	"storj.io/storj/pkg/storj"
)

var (
	// ServerError wraps errors returned from Server struct methods
	ServerError = errs.Class("PSServer error")
)

//DirSize returns the total size of the files in that directory
func DirSize(path string) (int64, error) {
	var size int64
	_, err := os.Stat(path)
	if err != nil {
		return 0, errors.New("path doesn't exists")
	}
	adjSize := func(_ string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	}
	err = filepath.Walk(path, adjSize)

	return size, err
}

// Server -- GRPC server meta data used in route calls
type Server struct {
	startTime        time.Time
	log              *zap.Logger
	storage          *pstore.Storage
	DB               *psdb.DB
	pkey             crypto.PrivateKey
	totalAllocated   int64 // TODO: use memory.Size
	totalBwAllocated int64 // TODO: use memory.Size
	whitelist        map[storj.NodeID]*ecdsa.PublicKey
	verifier         auth.SignedMessageVerifier
	kad              *kademlia.Kademlia
}

// NewEndpoint creates a new endpoint
func NewEndpoint(log *zap.Logger, config Config, storage *pstore.Storage, db *psdb.DB, pkey crypto.PrivateKey, k *kademlia.Kademlia) (*Server, error) {
	// read the allocated disk space from the config file
	allocatedDiskSpace := config.AllocatedDiskSpace.Int64()
	allocatedBandwidth := config.AllocatedBandwidth.Int64()

	// get the disk space details
	// The returned path ends in a slash only if it represents a root directory, such as "/" on Unix or `C:\` on Windows.
	info, err := storage.Info()
	if err != nil {
		return nil, ServerError.Wrap(err)
	}
	freeDiskSpace := info.AvailableSpace

	// get how much is currently used, if for the first time totalUsed = 0
	totalUsed, err := db.SumTTLSizes()
	if err != nil {
		//first time setup
		totalUsed = 0
	}

	usedBandwidth, err := db.GetTotalBandwidthBetween(getBeginningOfMonth(), time.Now())
	if err != nil {
		return nil, ServerError.Wrap(err)
	}

	if usedBandwidth > allocatedBandwidth {
		log.Warn("Exceed the allowed Bandwidth setting")
	} else {
		log.Info("Remaining Bandwidth", zap.Int64("bytes", allocatedBandwidth-usedBandwidth))
	}

	// check your hard drive is big enough
	// first time setup as a piece node server
	if totalUsed == 0 && freeDiskSpace < allocatedDiskSpace {
		allocatedDiskSpace = freeDiskSpace
		log.Warn("Disk space is less than requested. Allocating space", zap.Int64("bytes", allocatedDiskSpace))
	}

	// on restarting the Piece node server, assuming already been working as a node
	// used above the alloacated space, user changed the allocation space setting
	// before restarting
	if totalUsed >= allocatedDiskSpace {
		log.Warn("Used more space than allocated. Allocating space", zap.Int64("bytes", allocatedDiskSpace))
	}

	// the available diskspace is less than remaining allocated space,
	// due to change of setting before restarting
	if freeDiskSpace < allocatedDiskSpace-totalUsed {
		allocatedDiskSpace = freeDiskSpace
		log.Warn("Disk space is less than requested. Allocating space", zap.Int64("bytes", allocatedDiskSpace))
	}

	// parse the comma separated list of approved satellite IDs into an array of storj.NodeIDs
	whitelist := make(map[storj.NodeID]*ecdsa.PublicKey)
	if config.SatelliteIDRestriction {
		idStrings := strings.Split(config.WhitelistedSatelliteIDs, ",")
		for _, s := range idStrings {
			satID, err := storj.NodeIDFromString(s)
			if err != nil {
				return nil, err
			}
			whitelist[satID] = nil // we will set these later
		}
	}

	return &Server{
		startTime:        time.Now(),
		log:              log,
		storage:          storage,
		DB:               db,
		pkey:             pkey,
		totalAllocated:   allocatedDiskSpace,
		totalBwAllocated: allocatedBandwidth,
		whitelist:        whitelist,
		verifier:         auth.NewSignedMessageVerifier(),
		kad:              k,
	}, nil
}

// Close stops the server
func (s *Server) Close() error { return nil }

// Stop the piececstore node
func (s *Server) Stop(ctx context.Context) error {
	return errs.Combine(
		s.DB.Close(),
		s.storage.Close(),
	)
}

// Piece -- Send meta data about a piece stored by Id
func (s *Server) Piece(ctx context.Context, in *pb.PieceId) (*pb.PieceSummary, error) {
	s.log.Debug("Getting Meta", zap.String("Piece ID", in.GetId()))

	authorization := in.GetAuthorization()
	if err := s.verifier(authorization); err != nil {
		return nil, ServerError.Wrap(err)
	}

	id, err := getNamespacedPieceID([]byte(in.GetId()), getNamespace(authorization))
	if err != nil {
		return nil, err
	}

	path, err := s.storage.PiecePath(id)
	if err != nil {
		return nil, err
	}

	match, err := regexp.MatchString("^[A-Za-z0-9]{20,64}$", id)
	if err != nil {
		return nil, err
	}

	if !match {
		return nil, ServerError.New("invalid ID")
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// Read database to calculate expiration
	ttl, err := s.DB.GetTTLByID(id)
	if err != nil {
		return nil, err
	}

	s.log.Info("Successfully retrieved meta", zap.String("Piece ID", in.GetId()))
	return &pb.PieceSummary{Id: in.GetId(), PieceSize: fileInfo.Size(), ExpirationUnixSec: ttl}, nil
}

// Stats will return statistics about the Server
func (s *Server) Stats(ctx context.Context, in *pb.StatsReq) (*pb.StatSummary, error) {
	s.log.Debug("Getting Stats...")

	statsSummary, err := s.retrieveStats()
	if err != nil {
		return nil, err
	}

	s.log.Info("Successfully retrieved Stats...")

	return statsSummary, nil
}

func (s *Server) retrieveStats() (*pb.StatSummary, error) {
	totalUsed, err := s.DB.SumTTLSizes()
	if err != nil {
		return nil, err
	}

	totalUsedBandwidth, err := s.DB.GetTotalBandwidthBetween(getBeginningOfMonth(), time.Now())
	if err != nil {
		return nil, err
	}

	return &pb.StatSummary{UsedSpace: totalUsed, AvailableSpace: (s.totalAllocated - totalUsed), UsedBandwidth: totalUsedBandwidth, AvailableBandwidth: (s.totalBwAllocated - totalUsedBandwidth)}, nil
}

// Dashboard is a stream that sends data every `interval` seconds to the listener.
func (s *Server) Dashboard(in *pb.DashboardReq, stream pb.PieceStoreRoutes_DashboardServer) (err error) {
	ctx := stream.Context()
	ticker := time.NewTicker(3 * time.Second)

	for {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.Canceled {
				return nil
			}

			return ctx.Err()
		case <-ticker.C:
			data, err := s.getDashboardData(ctx)
			if err != nil {
				s.log.Warn("unable to create dashboard data proto")
				continue
			}

			if err := stream.Send(data); err != nil {
				s.log.Error("error sending dashboard stream", zap.Error(err))
				return err
			}
		}
	}
}

// Delete -- Delete data by Id from piecestore
func (s *Server) Delete(ctx context.Context, in *pb.PieceDelete) (*pb.PieceDeleteSummary, error) {
	s.log.Debug("Deleting", zap.String("Piece ID", fmt.Sprint(in.GetId())))
	authorization := in.GetAuthorization()
	if err := s.verifier(authorization); err != nil {
		return nil, ServerError.Wrap(err)
	}
	id, err := getNamespacedPieceID([]byte(in.GetId()), getNamespace(authorization))
	if err != nil {
		return nil, err
	}
	if err := s.deleteByID(id); err != nil {
		return nil, err
	}

	s.log.Info("Successfully deleted", zap.String("Piece ID", fmt.Sprint(in.GetId())))

	return &pb.PieceDeleteSummary{Message: OK}, nil
}

func (s *Server) deleteByID(id string) error {
	if err := s.storage.Delete(id); err != nil {
		return err
	}
	if err := s.DB.DeleteTTLByID(id); err != nil {
		return err
	}
	return nil
}

func (s *Server) verifySignature(ctx context.Context, rba *pb.RenterBandwidthAllocation) error {
	// TODO(security): detect replay attacks
	pba := rba.PayerAllocation
	//verify message content
	pi, err := identity.PeerIdentityFromContext(ctx)
	if err != nil || pba.UplinkId != pi.ID {
		return auth.ErrBadID.New("Uplink Node ID: %s vs %s", pba.UplinkId, pi.ID)
	}

	//todo:  use whitelist for satellites?
	switch {
	case len(pba.SerialNumber) == 0:
		return pb.ErrPayer.Wrap(auth.ErrMissing.New("serial"))
	case pba.SatelliteId.IsZero():
		return pb.ErrPayer.Wrap(auth.ErrMissing.New("satellite id"))
	case pba.UplinkId.IsZero():
		return pb.ErrPayer.Wrap(auth.ErrMissing.New("uplink id"))
	}
	exp := time.Unix(pba.GetExpirationUnixSec(), 0).UTC()
	if exp.Before(time.Now().UTC()) {
		return pb.ErrPayer.Wrap(auth.ErrExpired.New("%v vs %v", exp, time.Now().UTC()))
	}
	//verify message crypto
	if err := auth.VerifyMsg(rba, pba.UplinkId); err != nil {
		return pb.ErrRenter.Wrap(err)
	}
	if !s.isWhitelisted(pba.SatelliteId) {
		return pb.ErrPayer.Wrap(peertls.ErrVerifyCAWhitelist.New(""))
	}
	//todo: once the certs are removed from the PBA, use s.whitelist to check satellite signatures
	if err := auth.VerifyMsg(&pba, pba.SatelliteId); err != nil {
		return pb.ErrPayer.Wrap(err)
	}
	return nil
}

func (s *Server) verifyPayerAllocation(pba *pb.PayerBandwidthAllocation, actionPrefix string) (err error) {
	switch {
	case pba.SatelliteId.IsZero():
		return StoreError.New("payer bandwidth allocation: missing satellite id")
	case pba.UplinkId.IsZero():
		return StoreError.New("payer bandwidth allocation: missing uplink id")
	case !strings.HasPrefix(pba.Action.String(), actionPrefix):
		return StoreError.New("payer bandwidth allocation: invalid action %v", pba.Action.String())
	}
	return nil
}

//isWhitelisted returns true if a node ID exists in a list of approved node IDs
func (s *Server) isWhitelisted(id storj.NodeID) bool {
	if len(s.whitelist) == 0 {
		return true // don't whitelist if the whitelist is empty
	}
	_, found := s.whitelist[id]
	return found
}

func (s *Server) getPublicKey(ctx context.Context, id storj.NodeID) (*ecdsa.PublicKey, error) {
	pID, err := s.kad.FetchPeerIdentity(ctx, id)
	if err != nil {
		return nil, err
	}
	ecdsa, ok := pID.Leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, auth.ErrECDSA
	}
	return ecdsa, nil
}

func getBeginningOfMonth() time.Time {
	t := time.Now()
	y, m, _ := t.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, time.Now().Location())
}

func getNamespacedPieceID(pieceID, namespace []byte) (string, error) {
	if namespace == nil {
		return string(pieceID), nil
	}

	mac := hmac.New(sha512.New, namespace)
	_, err := mac.Write(pieceID)
	if err != nil {
		return "", err
	}
	h := mac.Sum(nil)
	return base58.Encode(h), nil
}

func getNamespace(signedMessage *pb.SignedMessage) []byte {
	return signedMessage.GetData()
}

func (s *Server) getDashboardData(ctx context.Context) (*pb.DashboardStats, error) {
	statsSummary, err := s.retrieveStats()
	if err != nil {
		return &pb.DashboardStats{}, ServerError.Wrap(err)
	}

	rt := s.kad.GetRoutingTable()

	nodes, err := s.kad.FindNear(ctx, storj.NodeID{}, 10000000)
	if err != nil {
		return &pb.DashboardStats{}, ServerError.Wrap(err)
	}

	bootstrapNodes := s.kad.GetBootstrapNodes()

	bsNodes := make([]string, len(bootstrapNodes))

	for i, node := range bootstrapNodes {
		bsNodes[i] = node.Address.Address
	}

	return &pb.DashboardStats{
		NodeId:           rt.Local().Id.String(),
		NodeConnections:  int64(len(nodes)),
		BootstrapAddress: strings.Join(bsNodes[:], ", "),
		InternalAddress:  "",
		ExternalAddress:  rt.Local().Address.Address,
		Connection:       true,
		Uptime:           ptypes.DurationProto(time.Since(s.startTime)),
		Stats:            statsSummary,
	}, nil
}

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package psserver

import (
	"crypto"
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
	"golang.org/x/net/context"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
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
	whitelist        []storj.NodeID
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
	var whitelist []storj.NodeID
	if config.SatelliteIDRestriction {
		idStrings := strings.Split(config.WhitelistedSatelliteIDs, ",")
		for i, s := range idStrings {
			whitelist[i], err = storj.NodeIDFromString(s)
			if err != nil {
				return nil, err
			}
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
	_, pba, pbad, err := rba.Unpack()
	if err != nil {
		return err
	}
	//verify message content
	pi, err := identity.PeerIdentityFromContext(ctx)
	if err != nil || pbad.UplinkId != pi.ID {
		return pb.BadID.New("Uplink Node ID: %s vs %s", pbad.UplinkId, pi.ID)
	}
	//todo:  use whitelist for uplinks?
	//todo:  use whitelist for satelites?
	switch {
	case len(pbad.SerialNumber) == 0:
		return pb.Payer.Wrap(pb.Missing.New("serial"))
	case pbad.SatelliteId.IsZero():
		return pb.Payer.Wrap(pb.Missing.New("satellite id"))
	case pbad.UplinkId.IsZero():
		return pb.Payer.Wrap(pb.Missing.New("uplink id"))
	}
	exp := time.Unix(pbad.GetExpirationUnixSec(), 0).UTC()
	if exp.Before(time.Now().UTC()) {
		return pb.Payer.Wrap(pb.Expired.New("%v vs %v", exp, time.Now().UTC()))
	}
	//verify message crypto
	if err := pb.VerifyMsg(rba, pbad.UplinkId); err != nil {
		return pb.Renter.Wrap(err)
	}
	if err := pb.VerifyMsg(pba, pbad.SatelliteId); err != nil {
		return pb.Payer.Wrap(err)
	}
	return nil
}

func (s *Server) verifyPayerAllocation(pba *pb.PayerBandwidthAllocation_Data, actionPrefix string) (err error) {
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

// approved returns true if a node ID exists in a list of approved node IDs
func (s *Server) approved(id storj.NodeID) bool {
	for _, n := range s.whitelist {
		if n == id {
			return true
		}
	}
	return false
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

	rt, err := s.kad.GetRoutingTable(ctx)
	if err != nil {
		return &pb.DashboardStats{}, ServerError.Wrap(err)
	}

	nodes, err := s.kad.GetNodes(ctx, rt.Local().Id, 0)
	if err != nil {
		return &pb.DashboardStats{}, ServerError.Wrap(err)
	}

	return &pb.DashboardStats{
		NodeId:          rt.Local().Id.String(),
		NodeConnections: int64(len(nodes)),
		Address:         "",
		Connection:      true,
		Uptime:          ptypes.DurationProto(time.Since(s.startTime)),
		Stats:           statsSummary,
	}, nil
}

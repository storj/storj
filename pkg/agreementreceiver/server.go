// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package agreementreceiver

import (
	"fmt"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"

	"go.uber.org/zap"
)

// Server is an implementation of the pb.BandwidthServer interface
type Server struct {
	// DB       *dbx.DB
	identity *provider.FullIdentity
	logger   *zap.Logger
}

// NewServer initializes a Server struct
func NewServer(source string, fi *provider.FullIdentity, logger *zap.Logger) (*Server, error) {
	//TODO: open dbx postgres database and pass to Server
	return &Server{
		identity: fi,
		logger:   logger,
	}, nil
}

// BandwidthAgreements receives and stores bandwidth agreements from storage nodes
func (s *Server) BandwidthAgreements(stream pb.Bandwidth_BandwidthAgreementsServer) (err error) {
	ctx := stream.Context()
	defer mon.Task()(&ctx)(&err)

	ch := make(chan *pb.RenterBandwidthAllocation, 1)
	errch := make(chan error, 1)
	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				errch <- err
				return
			}
			ch <- msg
		}
	}()

	for {
		select {
		case err := <-errch:
			return err
		case <-ctx.Done():
			return nil
		case agreement := <-ch:
			go func() {
				fmt.Println(agreement)
				//TODO: write to DB
				//err = s.DB.WriteAgreement(agreement)
				// if err != nil {
				// 	return err
				// }
			}()
		}
	}
}

// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package agreementreceiver

import (
	"fmt"

	"storj.io/storj/pkg/pb"

	"go.uber.org/zap"
)

type Server struct {
	logger *zap.Logger
}

func NewServer(source string, logger *zap.Logger) (*Server, error) {
	//TODO: open dbx postgres database, add DB field to Server
	return &Server{
		logger: logger,
	}, nil
}

func (s *Server) BandwidthAgreements(stream pb.Bandwidth_BandwidthAgreementsServer) error {
	fmt.Println("NOT IMPLEMENTED")
	return nil
}

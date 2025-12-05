// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package crash

import (
	"fmt"
	"os"
	"path"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
)

// Config contains configurable values for crash collect service.
type Config struct {
	StoringDir string `help:"directory to store crash reports" default:""`
}

// Error is a default error type for crash collect Service.
var Error = errs.Class("crashes service")

// Service exposes all crash-collect business logic.
//
// architecture: service
type Service struct {
	config Config
}

// NewService is an constructor for Service.
func NewService(config Config) *Service {
	return &Service{
		config: config,
	}
}

// Report receives report from crash-report client and saves it into .gz file.
func (s *Service) Report(nodeID storj.NodeID, gzippedPanic []byte) error {
	now := time.Now().UTC()

	filename := fmt.Sprintf("%s-%s.gz", nodeID.String(), now.Format(time.RFC3339))

	f, err := os.Create(path.Join(s.config.StoringDir, filename))
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, f.Close())
	}()

	_, err = f.Write(gzippedPanic)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

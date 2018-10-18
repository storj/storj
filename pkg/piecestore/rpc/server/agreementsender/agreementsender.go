// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package agreementsender

import (
	"flag"
	"log"
	"time"

	"github.com/zeebo/errs"
	"golang.org/x/net/context"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/piecestore/rpc/server/psdb"
	"storj.io/storj/pkg/utils"
)

var (
	mon                  = monkit.Package()
	defaultCheckInterval = flag.Duration("piecestore.agreementsender.check_interval", time.Hour, "number of seconds to sleep between agreement checks")

	// ASError wraps errors returned from agreementsender package
	ASError = errs.Class("agreement sender error")
)

type AgreementSender struct {
	DB   *psdb.DB
	errs []error
}

func Initialize(DB *psdb.DB) (*AgreementSender, error) {
	return &AgreementSender{DB: DB}, nil
}

func (as *AgreementSender) Run(ctx context.Context) error {
	log.Println("AgreementSender is starting up")

	c := make(chan []byte)

	ticker := time.NewTicker(*defaultCheckInterval)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			// agreements :=

			// for range agreements
			//   c <- &agreement
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return utils.CombineErrors(as.errs...)
		case agreement := <-c:
			log.Println(agreement)
			// Deserealize agreement
			// Get satellite ip from payer_id
			// Create client from satellite ip
			// Send agreement to satellite
		}
	}
}

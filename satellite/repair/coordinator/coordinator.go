// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package coordinator

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/rpc/rpcpeer"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/repair/repairer"
	"storj.io/storj/storage"
)

var (
	mon = monkit.Package()

	// NotAuthorizedErr is returned when the client is not authorized to be
	// a delegated repair worker.
	NotAuthorizedErr = errors.New("client not authorized as delegated repair worker")
)

// Config contains configurable values for the repair coordinator endpoint.
type Config struct {
	AuthorizedWorkerIDs       string        `help:"comma-separated list of node IDs for authorized delegated repair workers" releaseDefault:"" devDefault:""`
	RetryTimeWhenQueueIsEmpty time.Duration `help:"how long repair worker clients should wait before retrying when the repair queue is empty" releaseDefault:"10m" devDefault:"1s"`
	RetryTimeOnInternalError  time.Duration `help:"how long repair worker clients should wait before retrying when the coordinator experiences an internal error while preparing jobs" releaseDefault:"2m" devDefault:"0s"`
	JobTimeLimit              time.Duration `help:"maximum amount of time allowed for jobs done by delegated repair workers" releaseDefault:"1h" devDefault:"1h"`
}

// Endpoint is the coordinator endpoint.
//
// architecture: Endpoint
type Endpoint struct {
	log         *zap.Logger
	repairQueue queue.RepairQueue
	repairer    *repairer.SegmentRepairer
	jobListDB   JobListDB

	Config Config
	Now    func() time.Time

	authorizedWorkers map[storj.NodeID]struct{}

	// repairOverride allows overriding of the repair threshold
	repairOverride int
}

// NewEndpoint returns a new Endpoint instance.
func NewEndpoint(log *zap.Logger, config Config, repairQueue queue.RepairQueue, jobListDB JobListDB) (*Endpoint, error) {
	authorizedWorkers, err := readAuthorizedWorkersConfig(config.AuthorizedWorkerIDs)
	if err != nil {
		return nil, err
	}
	endpoint := &Endpoint{
		log:               log,
		repairQueue:       repairQueue,
		jobListDB:         jobListDB,
		Config:            config,
		Now:               time.Now,
		authorizedWorkers: authorizedWorkers,
	}
	return endpoint, nil
}

// RepairJob requests delegation of a new repair job. Optionally, it also
// provides the results of the last delegated repair job for the client.
func (endpoint *Endpoint) RepairJob(ctx context.Context, request *internalpb.RepairJobRequest) (_ *internalpb.RepairJobResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	workerID, workerAddr, err := endpoint.authorizePeer(ctx)
	if err != nil {
		endpoint.log.Debug("could not authorize delegated repair worker client", zap.Error(err), zap.Stringer("worker-addr", workerAddr))
		return nil, rpcstatus.Error(rpcstatus.Unauthenticated, err.Error())
	}
	log := endpoint.log.With(zap.Stringer("worker-id", workerID), zap.Stringer("worker-addr", workerAddr))

	// record results of last job, if appropriate
	if request.LastJobResult != nil {
		endpoint.RecordFinishedRepairJob(ctx, log, workerID, request.LastJobResult)
	}

	// keep trying to pull from the queue until we find a still-valid job
	for {
		seg, err := endpoint.repairQueue.Select(ctx)
		if err != nil {
			if storage.ErrEmptyQueue.Has(err) {
				log.Debug("no repair jobs available; instructing client to come back later")
				return endpoint.requestRetryIn(endpoint.Config.RetryTimeWhenQueueIsEmpty)
			}
			log.Error("failed to read from repair queue", zap.Error(err))
			return endpoint.requestRetryIn(endpoint.Config.RetryTimeOnInternalError)
		}

		shouldDelete, jobDefinition, _, _, err := endpoint.repairer.PrepareRepairJob(ctx, storj.Path(seg.Path))
		//TODO replace _, _ with pointer, healthyPieceNums
		if shouldDelete {
			deleteErr := endpoint.repairQueue.Delete(ctx, seg)
			if deleteErr != nil {
				log.Error("failed to remove repair job from queue!", zap.Error(deleteErr))
			}
		}
		if err != nil {
			log.Error("failed to prepare repair job", zap.Error(err))
			return endpoint.requestRetryIn(endpoint.Config.RetryTimeOnInternalError)
		}
		if jobDefinition != nil {
			return &internalpb.RepairJobResponse{NewJob: jobDefinition}, nil
		}
	}
}

func (endpoint *Endpoint) requestRetryIn(retryIn time.Duration) (*internalpb.RepairJobResponse, error) {
	return &internalpb.RepairJobResponse{ComeBackInMillis: int32(retryIn / time.Millisecond)}, nil
}

// RecordFinishedRepairJob records the result of a repair job from a delegated
// repair worker.
func (endpoint *Endpoint) RecordFinishedRepairJob(ctx context.Context, log *zap.Logger, workerID storj.NodeID, result *internalpb.RepairJobResult) {
	defer mon.Task()(&ctx)(nil)

	jobID, err := uuid.FromBytes(result.JobId)
	if err != nil {
		log.Error("worker supplied invalid job result: bad job_id", zap.Error(err))
	}
	log = log.With(zap.Stringer("job-id", jobID))

	// TODO: actually process results

	log.Info("repair job completed")
}

func (endpoint *Endpoint) authorizePeer(ctx context.Context) (peerNodeID storj.NodeID, addr net.Addr, err error) {
	defer mon.Task()(&ctx)(&err)

	peer, err := rpcpeer.FromContext(ctx)
	if err != nil {
		return storj.NodeID{}, nil, err
	}

	peerID, err := identity.PeerIdentityFromPeer(peer)
	if err != nil {
		return storj.NodeID{}, peer.Addr, err
	}

	if endpoint.isAuthorizedDelegatedRepairWorker(peerID.ID) {
		return peerID.ID, peer.Addr, nil
	}
	return storj.NodeID{}, peer.Addr, NotAuthorizedErr
}

func (endpoint *Endpoint) isAuthorizedDelegatedRepairWorker(peerNodeID storj.NodeID) bool {
	_, ok := endpoint.authorizedWorkers[peerNodeID]
	return ok
}

func readAuthorizedWorkersConfig(nodeIDList string) (map[storj.NodeID]struct{}, error) {
	authorizedWorkers := make(map[storj.NodeID]struct{})
	nodeIDStringList := strings.Split(nodeIDList, ",")
	for _, nodeIDString := range nodeIDStringList {
		if nodeIDString == "" {
			continue
		}
		nodeID, err := storj.NodeIDFromString(strings.TrimSpace(nodeIDString))
		if err != nil {
			return nil, err
		}
		authorizedWorkers[nodeID] = struct{}{}
	}
	return authorizedWorkers, nil
}

// JobListDB tracks ongoing delegated repair jobs.
type JobListDB interface {
}

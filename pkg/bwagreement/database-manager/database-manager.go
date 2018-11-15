package dbmanager

import (
	"context"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/migrate"
	dbx "storj.io/storj/pkg/bwagreement/database-manager/dbx"
	"storj.io/storj/pkg/pb"
)

var (
	mon = monkit.Package()
)

// DBManager is an implementation of the database access interface
type DBManager struct {
	DB *dbx.DB
	mu sync.Mutex
}

// NewDBManager creates a new instance of a DatabaseManager
func NewDBManager(driver, source string) (*DBManager, error) {
	db, err := dbx.Open(driver, source)
	if err != nil {
		return nil, err
	}

	err = migrate.Create("bwagreement", db)
	if err != nil {
		return nil, err
	}
	return &DBManager{
		DB: db,
	}, nil
}

func (dbm *DBManager) locked() func() {
	dbm.mu.Lock()
	return dbm.mu.Unlock
}

// Create a db entry for the provided storagenode
func (dbm *DBManager) Create(ctx context.Context, createBwAgreement *pb.RenterBandwidthAllocation) (bwagreement *dbx.Bwagreement, err error) {
	defer mon.Task()(&ctx)(&err)
	defer dbm.locked()()

	signature := createBwAgreement.GetSignature()
	data := createBwAgreement.GetData()

	bwagreement, err = dbm.DB.Create_Bwagreement(
		ctx,
		dbx.Bwagreement_Signature(signature),
		dbx.Bwagreement_Data(data),
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return bwagreement, nil
}

// GetBandwidthAllocations all bandwidth agreements and sorts by satellite
func (dbm *DBManager) GetBandwidthAllocations(ctx context.Context) (rows []*dbx.Bwagreement, err error) {
	defer mon.Task()(&ctx)(&err)
	defer dbm.locked()()
	rows, err = dbm.DB.All_Bwagreement(ctx)
	return rows, err
}

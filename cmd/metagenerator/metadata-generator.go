package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"
	"storj.io/common/uuid"
	"storj.io/storj/metagenerator"
)

// default values
const (
	clusterPath          = "/Users/bohdanbashynskyi/storj-cluster"
	defaultDbEndpoint    = "postgresql://root@localhost:26257/metainfo?sslmode=disable"
	defaultSharedFields  = 0.3
	defaultBatchSize     = 10
	defaultWorkersNumber = 1
	defaultTotlaRecords  = 10
	defaultMetasearchAPI = "http://localhost:9998/meta_search"
)

// main parameters decalaration
var (
	dbEndpoint    string
	sharedFields  float64 = 0.3
	batchSize     int
	workersNumber int
	totalRecords  int
	mode          string
)

func readArgs() {
	flag.StringVar(&dbEndpoint, "db", defaultDbEndpoint, fmt.Sprintf("db endpoint, default: %v", defaultDbEndpoint))
	flag.StringVar(&mode, "mode", metagenerator.DryRunMode, fmt.Sprintf("incert mode [%s, %s, %s], default: %v", metagenerator.ApiMode, metagenerator.DbMode, metagenerator.DryRunMode, metagenerator.DryRunMode))
	flag.Float64Var(&sharedFields, "sharedFields", defaultSharedFields, fmt.Sprintf("percentage of shared fields, default: %v", defaultSharedFields))
	flag.IntVar(&batchSize, "batchSize", defaultBatchSize, fmt.Sprintf("number of records per batch, default: %v", defaultBatchSize))
	flag.IntVar(&workersNumber, "workersNumber", defaultWorkersNumber, fmt.Sprintf("number of workers, default: %v", defaultWorkersNumber))
	flag.IntVar(&totalRecords, "totalRecords", defaultTotlaRecords, fmt.Sprintf("total number of records, default: %v", defaultTotlaRecords))
	flag.Parse()
}

func main() {
	readArgs()

	// Connect to CockroachDB
	db, err := sql.Open("postgres", dbEndpoint)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to database: %v", err))
	}
	defer db.Close()
	ctx := context.Background()

	var projectId string
	if mode == metagenerator.ApiMode {
		projectId = metagenerator.GetProjectId(ctx, db).String()
	}
	if mode == metagenerator.DbMode {
		pId, _ := uuid.New()
		projectId = pId.String()
	}

	// Initialize batch generator
	batchGen := metagenerator.NewBatchGenerator(
		db,
		sharedFields,  // 30% shared fileds
		batchSize,     // batch size
		workersNumber, // number of workers
		totalRecords,
		metagenerator.GetPathCount(ctx, db), // get path count
		projectId,
		os.Getenv("API_KEY"),
		mode, // incert mode
		defaultMetasearchAPI,
	)

	// Generate and insert/debug records
	startTime := time.Now()

	if err := batchGen.GenerateAndInsert(ctx, totalRecords); err != nil {
		panic(fmt.Sprintf("failed to generate records: %v", err))
	}

	fmt.Printf("Generated %v records in %v\n", totalRecords, time.Since(startTime))
}

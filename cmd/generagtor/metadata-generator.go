package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"sync"
	"time"

	_ "github.com/lib/pq"
)
 // default values
const (
	defaultDbEndpoint    = "postgresql://root@localhost:26257/defaultdb?sslmode=disable"
	defaultSharedValues  = 0.3
	defaultBatchSize     = 10
	defaultWorkersNumber = 1
	defaultTotlaRecords  = 10
	defaultDryRun        = true
)

// main parameters decalaration
var (
	dbEndpoint    string
	sharedValues  float64 = 0.3
	batchSize     int
	workersNumber int
	totalRecords  int
	dryRun        bool
)

// Record represents a single database record
type Record struct {
	Path     string         `json:"path"`
	Metadata map[string]any `json:"metadata"`
}

// Generator handles the creation of test data
type Generator struct {
	commonValues map[string][]any
	valueShare   float64
	pathPrefix   chan string // Channel for generating unique path prefixes
	pathCounter  uint64      // Counter for ensuring unique paths
	mu           sync.Mutex  // Mutex for thread-safe path generation
	randPool     sync.Pool   // Pool of random number generators
}

// NewGenerator creates a new Generator instance with a buffered path prefix channel
func NewGenerator(valueShare float64) *Generator {
	// Create a pool of random number generators
	randPool := sync.Pool{
		New: func() interface{} {
			return rand.New(rand.NewSource(time.Now().UnixNano()))
		},
	}

	g := &Generator{
		commonValues: map[string][]any{
			"string":  {"red", "blue", "green", "yellow", "purple"},
			"number":  {1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			"boolean": {true, false},
		},
		valueShare:  valueShare,
		pathPrefix:  make(chan string, 1000), // Buffered channel for path prefixes
		pathCounter: 0,
		randPool:    randPool,
	}

	// Start goroutine to generate path prefixes
	go g.generatePathPrefixes()
	return g
}

// getRand gets a random number generator from the pool
func (g *Generator) getRand() *rand.Rand {
	return g.randPool.Get().(*rand.Rand)
}

// putRand returns a random number generator to the pool
func (g *Generator) putRand(r *rand.Rand) {
	g.randPool.Put(r)
}

// generatePathPrefixes continuously generates path prefixes
func (g *Generator) generatePathPrefixes() {
	prefixes := []string{"users", "orders", "products", "categories"}
	subPaths := []string{"details", "metadata", "config", "settings"}

	r := g.getRand()
	defer g.putRand(r)

	for {
		prefix := fmt.Sprintf("/%s/%s",
			prefixes[r.Intn(len(prefixes))],
			subPaths[r.Intn(len(subPaths))],
		)
		g.pathPrefix <- prefix
	}
}

// generatePath creates a unique path with shared prefixes
func (g *Generator) generatePath() string {
	g.mu.Lock()
	g.pathCounter++
	counter := g.pathCounter
	g.mu.Unlock()

	prefix := <-g.pathPrefix
	return fmt.Sprintf("%s/%d", prefix, counter)
}

// generateValue creates either a shared or unique value
func (g *Generator) generateValue() any {
	r := g.getRand()
	defer g.putRand(r)

	valueTypes := []string{"string", "number", "boolean"}
	valueType := valueTypes[r.Intn(len(valueTypes))]

	if r.Float64() < g.valueShare {
		values := g.commonValues[valueType]
		return values[r.Intn(len(values))]
	}

	switch valueType {
	case "string":
		return fmt.Sprintf("unique_%d", r.Intn(10000))
	case "number":
		return r.Intn(10000)
	case "boolean":
		return r.Intn(2) == 1
	default:
		return nil
	}
}

// GenerateRecord creates a single record with random metadata
func (g *Generator) GenerateRecord() Record {
	r := g.getRand()
	defer g.putRand(r)

	numKeys := r.Intn(6) + 5 // 5-10 keys
	metadata := make(map[string]any)

	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("field_%d", i)
		metadata[key] = g.generateValue()
	}

	return Record{
		Path:     g.generatePath(),
		Metadata: metadata,
	}
}

// BatchGenerator handles batch generation of records
type BatchGenerator struct {
	generator *Generator
	db        *sql.DB
	batchSize int
	workers   int
}

// NewBatchGenerator creates a new BatchGenerator
func NewBatchGenerator(db *sql.DB, valueShare float64, batchSize, workers int) *BatchGenerator {
	return &BatchGenerator{
		generator: NewGenerator(valueShare),
		db:        db,
		batchSize: batchSize,
		workers:   workers,
	}
}

// GenerateAndDebug generates and prints records in batches using multiple workers
func (bg *BatchGenerator) GenerateAndDebug(ctx context.Context, totalRecords int) error {
	var wg sync.WaitGroup
	errChan := make(chan error, bg.workers)

	recordsPerWorker := totalRecords / bg.workers

	// Start workers
	for i := 0; i < bg.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			bS := bg.batchSize
			// Begin transaction for each batch
			for j := 0; j < recordsPerWorker; j += bS {
				if (recordsPerWorker - j) < bS {
					bS = recordsPerWorker - j
				}
				if err := bg.debugBatch(ctx, bS); err != nil {
					errChan <- fmt.Errorf("worker %d failed: %v", workerID, err)
					return
				}

				if j%bS == 0 {
					fmt.Printf("Worker %d processed %d records\n", workerID, j+bS)
				}
			}
		}(i)
	}

	// Wait for all workers to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

func prettyPrint(b []byte) {
	var out bytes.Buffer
	err := json.Indent(&out, b, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("failed to print record: %v", err))
	}
	fmt.Printf("%s\n", out.Bytes())
}

// debugBatch generates and prints a batch of records
func (bg *BatchGenerator) debugBatch(ctx context.Context, batchSize int) (err error) {
	for i := 0; i < batchSize; i++ {
		record := bg.generator.GenerateRecord()
		metadata, err := json.Marshal(record.Metadata)
		if err != nil {
			return err
		}
		prettyPrint(metadata)
	}
	return err
}

// GenerateAndInsert generates and inserts records in batches using multiple workers
func (bg *BatchGenerator) GenerateAndInsert(ctx context.Context, totalRecords int) error {
	var wg sync.WaitGroup
	errChan := make(chan error, bg.workers)
	recordsPerWorker := totalRecords / bg.workers

	// Create table if it doesn't exist
	_, err := bg.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS test_data (
			path STRING PRIMARY KEY,
			metadata JSONB
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create table: %v", err)
	}

	// Prepare the insert statement
	stmt, err := bg.db.PrepareContext(ctx, `
		INSERT INTO test_data (path, metadata) 
		VALUES ($1, $2)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	// Start workers
	for i := 0; i < bg.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// Begin transaction for each batch
			for j := 0; j < recordsPerWorker; j += bg.batchSize {
				if err := bg.processBatch(ctx, stmt, bg.batchSize); err != nil {
					errChan <- fmt.Errorf("worker %d failed: %v", workerID, err)
					return
				}

				if j%(totalRecords/10) == 0 {
					fmt.Printf("Worker %d processed %d records\n", workerID, j)
				}
			}
		}(i)
	}

	// Wait for all workers to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// processBatch generates and inserts a batch of records
func (bg *BatchGenerator) processBatch(ctx context.Context, stmt *sql.Stmt, batchSize int) error {
	tx, err := bg.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i := 0; i < batchSize; i++ {
		record := bg.generator.GenerateRecord()
		metadata, err := json.Marshal(record.Metadata)
		if err != nil {
			return err
		}

		if _, err := tx.StmtContext(ctx, stmt).ExecContext(ctx, record.Path, metadata); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func readArgs() {
	flag.StringVar(&dbEndpoint, "db", defaultDbEndpoint, fmt.Sprintf("db endpoint, default: %v", defaultDbEndpoint))
	flag.Float64Var(&sharedValues, "sv", defaultSharedValues, fmt.Sprintf("percentage of shared values, default: %v", defaultSharedValues))
	flag.IntVar(&batchSize, "bs", defaultBatchSize, fmt.Sprintf("number of records per batch, default: %v", defaultBatchSize))
	flag.IntVar(&workersNumber, "wn", defaultWorkersNumber, fmt.Sprintf("number of workers, default: %v", defaultWorkersNumber))
	flag.IntVar(&totalRecords, "tr", defaultTotlaRecords, fmt.Sprintf("total number of records, default: %v", defaultTotlaRecords))
	flag.BoolVar(&dryRun, "dr", defaultDryRun, fmt.Sprintf("enable dry run mode (if true records will be printed and will not be written to db), default: %v", defaultDryRun))
	flag.Parse()
}

func main() {
	readArgs()

	var db *sql.DB
	if !dryRun {
		// Connect to CockroachDB
		db, err := sql.Open("postgres", dbEndpoint)
		if err != nil {
			panic(fmt.Sprintf("failed to connect to database: %v", err))
		}
		defer db.Close()
	}

	ctx := context.Background()

	// Initialize batch generator
	batchGen := NewBatchGenerator(
		db,            // database connection
		sharedValues,  // 30% shared values
		batchSize,     // batch size
		workersNumber, // number of workers
	)

	// Generate and insert/debug records
	startTime := time.Now()
	if dryRun {
		if err := batchGen.GenerateAndDebug(ctx, totalRecords); err != nil {
			panic(fmt.Sprintf("failed to generate records: %v", err))
		}
	} else {
		if err := batchGen.GenerateAndInsert(ctx, totalRecords); err != nil {
			panic(fmt.Sprintf("failed to generate records: %v", err))
		}
	}

	fmt.Printf("Generated %v records in %v\n", totalRecords, time.Since(startTime))
}

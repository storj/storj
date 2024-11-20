package metagenerator

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"storj.io/common/uuid"
)

// default values
const (
	Label       = "benchmarks"
	clusterPath = "/Users/bohdanbashynskyi/storj-cluster"
	ApiMode     = "api"
	DbMode      = "db"
	DryRunMode  = "dryRun"
)

// main parameters decalaration
var (
	MatchingEntries = []int{
		1,
		10_000,
		1_000_000,
		10_000_000,
	}
)

// Record represents a single database record
type Record struct {
	Path     string         `json:"path"`
	Metadata map[string]any `json:"metadata"`
}

// Generator handles the creation of test data
type Generator struct {
	totalRecords int
	commonValues map[string][]any
	fieldShare   float64
	pathPrefix   chan string // Channel for generating unique path prefixes
	pathCounter  uint64      // Counter for ensuring unique paths
	mu           sync.Mutex  // Mutex for thread-safe path generation
	randPool     sync.Pool   // Pool of random number generators
}

// NewGenerator creates a new Generator instance with a buffered path prefix channel
func NewGenerator(fieldShare float64, pathCounter uint64, totalRecords int) *Generator {
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
		fieldShare:   fieldShare,
		pathPrefix:   make(chan string, 1000), // Buffered channel for path prefixes
		pathCounter:  pathCounter,
		randPool:     randPool,
		totalRecords: totalRecords,
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
		prefix := fmt.Sprintf("%s/%s",
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
func (g *Generator) generateKeyValue(i int) (key string, val any) {
	r := g.getRand()
	defer g.putRand(r)

	valueTypes := []string{"string", "number", "boolean"}
	valueType := valueTypes[r.Intn(len(valueTypes))]

	if r.Float64() < g.fieldShare {
		values := g.commonValues[valueType]
		val = values[r.Intn(len(values))]
		key = fmt.Sprintf("field_%v", val)
		return
	}

	key = fmt.Sprintf("field_%d", i)
	switch valueType {
	case "string":
		val = fmt.Sprintf("unique_%d", r.Intn(10000))
	case "number":
		val = r.Intn(10000)
	case "boolean":
		val = r.Intn(2) == 1
	}
	return
}

// GenerateRecord creates a single record with random metadata
func (g *Generator) GenerateRecord() Record {
	r := g.getRand()
	defer g.putRand(r)

	numKeys := r.Intn(6) + 5 // 5-10 keys
	metadata := make(map[string]any)

	for i := 0; i < numKeys; i++ {
		key, val := g.generateKeyValue(i)
		metadata[key] = val
	}

	for _, n := range MatchingEntries {
		if n > g.totalRecords {
			break
		}
		var devider int
		if n == 1 {
			devider = g.totalRecords
		} else {
			devider = g.totalRecords / n
		}
		if math.Mod(float64(g.pathCounter+1), float64(devider)) == 0 {
			val := fmt.Sprintf("benchmarkValue_%v", n)
			metadata["field_"+val] = val
		}
	}

	return Record{
		Path:     g.generatePath(),
		Metadata: metadata,
	}
}

// BatchGenerator handles batch generation of records
type BatchGenerator struct {
	db                 *sql.DB
	generator          *Generator
	batchSize          int
	workers            int
	totalRecords       int
	mode               string
	projectId          string
	apiKey             string
	metaSearchEndpoint string
}

// NewBatchGenerator creates a new BatchGenerator
func NewBatchGenerator(db *sql.DB, fieldShare float64, batchSize, workers, totalRecords int, pathCounter uint64, projectId, apiKey, mode, metaSearchEndpoint string) *BatchGenerator {
	return &BatchGenerator{
		db:                 db,
		generator:          NewGenerator(fieldShare, pathCounter, totalRecords),
		batchSize:          batchSize,
		workers:            workers,
		mode:               mode,
		projectId:          projectId,
		apiKey:             apiKey,
		totalRecords:       totalRecords,
		metaSearchEndpoint: metaSearchEndpoint,
	}
}

// GenerateAndInsert generates and put object with metadata in batches using multiple workers
func (bg *BatchGenerator) GenerateAndInsert(ctx context.Context, totalRecords int) (err error) {
	var wg sync.WaitGroup
	errChan := make(chan error, bg.workers)
	recordsPerWorker := bg.totalRecords / bg.workers

	var stmt *sql.Stmt
	if bg.mode == DbMode {
		// Prepare the insert statement
		// TODO: refactor with uplink library
		stmt, err = bg.db.PrepareContext(ctx, `
			INSERT INTO objects (project_id, bucket_name, object_key, version, stream_id, status, clear_metadata) 
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`)
		if err != nil {
			return fmt.Errorf("failed to prepare statement: %v", err)
		}
		defer stmt.Close()
	}

	// Start workers
	for i := 0; i < bg.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < recordsPerWorker; j += bg.batchSize {
				if err := bg.processBatch(ctx, stmt); err != nil {
					errChan <- fmt.Errorf("worker %d failed: %v", workerID, err)
					return
				}

				if j%(bg.batchSize) == 0 {
					fmt.Printf("Worker %d processed %d records\n", workerID, j+bg.batchSize)
				}
			}
		}(i)
	}

	// Wait for all workers to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err = range errChan {
		if err != nil {
			return
		}
	}

	return
}

func (bg *BatchGenerator) apiIncert(record *Record) (err error) {
	if err = putFile(record); err != nil {
		return
	}
	return putMeta(record, bg.apiKey, bg.projectId, bg.metaSearchEndpoint)
}

func (bg *BatchGenerator) dbIncert(record *Record, stmt *sql.Stmt, tx *sql.Tx, ctx context.Context) (err error) {
	metadata, err := json.Marshal(record.Metadata)
	if err != nil {
		return err
	}

	pId, _ := uuid.FromString(bg.projectId)
	_, err = tx.StmtContext(ctx, stmt).ExecContext(ctx, pId.Bytes(), Label, record.Path, 1, pId.Bytes(), 3, metadata)
	return
}

func (bg *BatchGenerator) dryRun(record *Record) (err error) {
	prettyPrint(record)
	return
}

// processBatch generates and inserts a batch of records
func (bg *BatchGenerator) processBatch(ctx context.Context, stmt *sql.Stmt) (err error) {
	var tx *sql.Tx
	if bg.mode == DbMode {
		tx, err = bg.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	for i := 0; i < bg.batchSize; i++ {
		record := bg.generator.GenerateRecord()

		switch bg.mode {
		case ApiMode:
			err = bg.apiIncert(&record)
		case DbMode:
			err = bg.dbIncert(&record, stmt, tx, ctx)
		case DryRunMode:
			err = bg.dryRun(&record)
		default:
			panic("Unkonwn mode")
		}
		if err != nil {
			return
		}
	}

	if bg.mode == DbMode {
		return tx.Commit()
	}
	return
}

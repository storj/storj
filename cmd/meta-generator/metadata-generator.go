package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Netflix/go-expect"
	"github.com/google/goterm/term"
	_ "github.com/lib/pq"
	"storj.io/common/uuid"
)

// default values
const (
	label                = "benchmarks"
	clusterPath          = "/Users/bohdanbashynskyi/storj-cluster"
	defaultDbEndpoint    = "postgresql://root@localhost:26257/metainfo?sslmode=disable"
	defaultSharedValues  = 0.3
	defaultBatchSize     = 10
	defaultWorkersNumber = 1
	defaultTotlaRecords  = 10
	defaultDryRun        = false
	defaultMetasearchAPI = "http://localhost:9998/meta_search"
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
	Path     string `json:"path"`
	Metadata string `json:"metadata"`
}

type Query struct {
	Path  string `json:"path"`
	Query string `json:"query"`
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
func NewGenerator(valueShare float64, pathCounter uint64) *Generator {
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
		pathCounter: pathCounter,
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

	metadataString, err := json.Marshal(metadata)
	if err != nil {
		panic(err)
	}

	return Record{
		Path:     g.generatePath(),
		Metadata: string(metadataString),
	}
}

// BatchGenerator handles batch generation of records
type BatchGenerator struct {
	generator    *Generator
	batchSize    int
	workers      int
	totalRecords int
	dryRun       bool
	projectId    string
	apiKey       string
}

// NewBatchGenerator creates a new BatchGenerator
func NewBatchGenerator(valueShare float64, batchSize, workers, totalRecords int, pathCounter uint64, dryRun bool, projectId, apiKey string) *BatchGenerator {
	return &BatchGenerator{
		generator:    NewGenerator(valueShare, pathCounter),
		batchSize:    batchSize,
		workers:      workers,
		dryRun:       dryRun,
		projectId:    projectId,
		apiKey:       apiKey,
		totalRecords: totalRecords,
	}
}

// GenerateAndInsert generates and put object with metadata in batches using multiple workers
func (bg *BatchGenerator) GenerateAndInsert(totalRecords int) error {
	var wg sync.WaitGroup
	errChan := make(chan error, bg.workers)
	recordsPerWorker := bg.totalRecords / bg.workers

	// Start workers
	for i := 0; i < bg.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < recordsPerWorker; j += bg.batchSize {
				if err := bg.processBatch(bg.batchSize); err != nil {
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
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// processBatch generates and inserts a batch of records
func (bg *BatchGenerator) processBatch(batchSize int) (err error) {
	for i := 0; i < batchSize; i++ {
		record := bg.generator.GenerateRecord()

		if err := putFile(&record); err != nil {
			return err
		}

		if err := bg.putMeta(&record); err != nil {
			return err
		}
	}

	return
}

func putFile(record *Record) error {
	localPath := filepath.Join("/tmp", strings.ReplaceAll(record.Path, "/", "_"))
	record.Path = "sj://" + label + record.Path

	if !dryRun {
		file, err := os.Create(localPath)
		if err != nil {
			return err
		}
		file.Close()

		// Copy file
		// TODO: rerfactor with uplink library
		cmd := exec.Command("uplink", "cp", localPath, record.Path)
		cmd.Dir = clusterPath
		out, err := cmd.CombinedOutput()
		fmt.Println(string(out))
		if err != nil {
			return err
		}

		err = os.Remove(localPath)
		if err != nil {
			return err
		}
	}

	return nil
}

func deleteFile(record *Record) error {
	cmd := exec.Command("uplink", "rm", record.Path)
	cmd.Dir = clusterPath
	out, err := cmd.CombinedOutput()
	fmt.Println(string(out))

	return err
}

func (bg *BatchGenerator) putMeta(record *Record) error {
	url := defaultMetasearchAPI
	req, err := json.Marshal(record)
	if err != nil {
		return err
	}

	r, err := http.NewRequest("PUT", url, bytes.NewBuffer(req))
	if err != nil {
		return err
	}
	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", bg.apiKey))
	r.Header.Add("X-Project-ID", bg.projectId)

	if dryRun {
		res, err := httputil.DumpRequest(r, true)
		if err != nil {
			return err
		}
		fmt.Println(string(res))
		return nil
	}

	client := &http.Client{}
	res, err := client.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return err
	}

	return nil
}

func (bg *BatchGenerator) searchMeta(query Query) error {
	url := defaultMetasearchAPI
	req, err := json.Marshal(query)
	if err != nil {
		return err
	}
	r, err := http.NewRequest("POST", url, bytes.NewBuffer(req))
	if err != nil {
		return err
	}
	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", bg.apiKey))
	r.Header.Add("X-Project-ID", bg.projectId)

	reqest, err := httputil.DumpRequest(r, true)
	if err != nil {
		return err
	}
	fmt.Println(string(reqest))

	client := &http.Client{}
	res, err := client.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}
	bodyString := string(bodyBytes)
	fmt.Println(bodyString)

	return nil
}

func getPathCount(ctx context.Context, db *sql.DB) (count uint64) {
	// Get path count
	rows, err := db.QueryContext(ctx, `SELECT COUNT(*) FROM objects`)
	if err != nil {
		if err.Error() == `pq: relation "objects" does not exist` {
			return 0
		}
		panic(fmt.Sprintf("failed to get path count: %v", err))
	}
	defer rows.Close()

	for rows.Next() {
		if err := rows.Scan(&count); err != nil {
			panic(err.Error())
		}
	}
	// Check for errors from iterating over rows.
	if err := rows.Err(); err != nil {
		panic(err.Error())
	}
	fmt.Printf("Found %v records\n", count)
	return
}

func getProjectId(ctx context.Context, db *sql.DB) (projectId uuid.UUID) {
	testFile := &Record{
		Path: "/testFiletoGetPrjectId",
	}
	err := putFile(testFile)
	if err != nil {
		panic(fmt.Sprintf("failed to create test file: %s", err.Error()))
	}

	rows, err := db.QueryContext(ctx, `SELECT project_id FROM objects where object_key = 'testFiletoGetPrjectId'`)
	if err != nil {
		panic(fmt.Sprintf("failed to get project id from db: %s", err.Error()))
	}
	defer rows.Close()

	var data []byte
	for rows.Next() {
		if err := rows.Scan(&data); err != nil {
			panic(err.Error())
		}
	}
	if err := rows.Err(); err != nil {
		panic(err.Error())
	}

	projectId, err = uuid.FromBytes(data)
	if err != nil {
		panic(fmt.Sprintf("failed to get project id from bytes: %s", err.Error()))
	}
	fmt.Printf("Found projectId %s\n", projectId.String())

	err = deleteFile(testFile)
	if err != nil {
		panic(fmt.Sprintf("failed to delete test file: %s", err.Error()))
	}

	return
}

func uplinkSetup(satelliteAddress, apiKey string) {
	c, err := expect.NewConsole(expect.WithStdout(os.Stdout))
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	cmd := exec.Command("uplink", "setup", "--force")
	cmd.Dir = clusterPath
	cmd.Stdin = c.Tty()
	cmd.Stdout = c.Tty()
	cmd.Stderr = c.Tty()

	err = cmd.Start()
	if err != nil {
		log.Fatal(err)
	}

	c.ExpectString("Enter name to import as [default: main]:")
	c.Send(label + "\n")
	c.ExpectString("Enter API key or Access grant:")
	c.Send(apiKey + "\n")
	c.ExpectString("Satellite address:")
	c.Send(satelliteAddress + "\n")
	c.ExpectString("Passphrase:")
	c.Send(label + "\n")
	c.ExpectString("Again:")
	c.Send(label + "\n")
	c.ExpectString("Would you like to disable encryption for object keys (allows lexicographical sorting of objects in listings)? (y/N):")
	c.Send("y\n")
	c.ExpectString("Would you like S3 backwards-compatible Gateway credentials? (y/N):")
	c.Send("y\n")
	fmt.Println(term.Greenf("Uplink setup done"))
}

func generatorSetup(bS, wN, tR int, apiKey string) *BatchGenerator {
	//Create bucket
	cmd := exec.Command("uplink", "mb", "sj://benchmarks")

	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "bucket already exists") {
		panic(err.Error())
	}

	// Connect to CockroachDB
	db, err := sql.Open("postgres", defaultDbEndpoint)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to database: %v", err))
	}
	defer db.Close()

	ctx := context.Background()

	// Initialize batch generator
	batchGen := NewBatchGenerator(
		defaultSharedValues,
		bS,
		wN,
		tR,
		getPathCount(ctx, db),
		true,
		getProjectId(ctx, db).String(),
		apiKey,
	)

	// Generate and insert/debug records
	startTime := time.Now()

	if err := batchGen.GenerateAndInsert(totalRecords); err != nil {
		panic(fmt.Sprintf("failed to generate records: %v", err))
	}

	fmt.Printf("Generated %v records in %v\n", tR, time.Since(startTime))
	return batchGen
}

func clean() {
	//Remove bucket
	cmd := exec.Command("uplink", "rb", "sj://"+label, "--force")
	cmd.Dir = clusterPath

	out, err := cmd.CombinedOutput()
	fmt.Println(string(out))
	if err != nil {
		panic(err.Error())
	}
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

	// Connect to CockroachDB
	db, err := sql.Open("postgres", dbEndpoint)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to database: %v", err))
	}
	defer db.Close()

	ctx := context.Background()

	// Initialize batch generator
	batchGen := NewBatchGenerator(
		sharedValues,  // 30% shared values
		batchSize,     // batch size
		workersNumber, // number of workers
		totalRecords,
		getPathCount(ctx, db), // get path count
		dryRun,                // dry run mode
		getProjectId(ctx, db).String(),
		os.Getenv("API_KEY"),
	)

	// Generate and insert/debug records
	startTime := time.Now()

	if err := batchGen.GenerateAndInsert(totalRecords); err != nil {
		panic(fmt.Sprintf("failed to generate records: %v", err))
	}

	fmt.Printf("Generated %v records in %v\n", totalRecords, time.Since(startTime))
}

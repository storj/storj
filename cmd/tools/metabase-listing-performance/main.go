// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"regexp"
	"runtime/pprof"
	"strconv"
	"time"

	"github.com/loov/hrtime"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	_ "storj.io/storj/shared/dbutil/cockroachutil"
)

// Scenario declares a specific test scenario.
type Scenario struct {
	Name  string
	Setup Generator
}

var scenarios = []Scenario{
	{Name: "Small",
		Setup: BasicObjects{Count: 1000, Versions: 3, Pending: 1}},
	{Name: "Pending",
		Setup: BasicObjects{Count: 1000, Versions: 3, Pending: 10000}},
	{Name: "Versions",
		Setup: BasicObjects{Count: 1000, Versions: 10000, Pending: 3}},
	{Name: "PrefixSmall",
		Setup: PrefixedObjects{
			Prefixes:  100,
			PerPrefix: BasicObjects{Count: 1000, Versions: 3, Pending: 1},
		}},
	{Name: "PrefixLarge",
		Setup: PrefixedObjects{
			Prefixes:      10,
			PerPrefix:     BasicObjects{Count: 100000, Versions: 1, Pending: 1},
			BetweenPrefix: BasicObjects{Count: 1, Versions: 1},
		}},
	{Name: "PrefixVersions",
		Setup: PrefixedObjects{
			Prefixes:      100,
			PerPrefix:     BasicObjects{Count: 500, Versions: 100, Pending: 1},
			BetweenPrefix: BasicObjects{Count: 1, Versions: 1},
		}},
}

var queryLimits = []int{50, 100, 1000}

var (
	databaseConn   = flag.String("database", os.Getenv("STORJ_TEST_COCKROACH"), "database to use for testing")
	benchmarkCount = flag.Int("benchmark", 6, "how many times to repeat a single query for benchmarking")
	filter         = flag.String("filter", "", "run only tests that match this regular expression")
	skipList       = flag.Bool("skip-list", false, "skip list objects benchmark")

	cpuprofile = flag.String("cpuprofile", "", "profile cpu")
)

func main() {
	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "could not create CPU profile: ", err)
		} else {
			defer func() { _ = f.Close() }()
			if err := pprof.StartCPUProfile(f); err != nil {
				_, _ = fmt.Fprintln(os.Stderr, "could not start CPU profile: ", err)
			}
			defer pprof.StopCPUProfile()
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	log, _ := zap.NewDevelopment()
	var err error
	switch flag.Arg(0) {
	case "info":
		err = Info(ctx, log)
	case "setup":
		err = WithDatabase(ctx, log, Setup)
	case "benchmark":
		err = WithDatabase(ctx, log, Benchmark)
	default:
		err = errors.New("unknown command")
	}

	_ = log.Sync()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
	}
}

// WithDatabase starts a new process with the database.
func WithDatabase(ctx context.Context, log *zap.Logger, process func(ctx context.Context, log *zap.Logger, db *metabase.DB) error) error {
	connstr := *databaseConn
	if connstr == "" {
		return errors.New("database connection missing, please specify `-database` flag")
	}

	config := metabase.Config{
		ApplicationName:  "metabase-listing-performance",
		MinPartSize:      2048,
		MaxNumberOfParts: 128,
		ServerSideCopy:   true,
	}

	config.ApplicationName += "-test"

	db, err := metabase.Open(ctx, log.Named("metabase"), connstr, config)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, db.Close()) }()

	if err := db.TestMigrateToLatest(ctx); err != nil {
		return errs.Wrap(err)
	}

	return process(ctx, log, db)
}

// DefaultProjectID is the project id used for testing.
var DefaultProjectID = uuid.UUID{'p', 'r', 'o', 'j'}

// Info prints details about the scenarios.
func Info(ctx context.Context, log *zap.Logger) error {
	for _, scenario := range scenarios {
		log.Info("Scenario "+scenario.Name, zap.Int("objects", scenario.Setup.Expected()))
	}
	return nil
}

// Setup (re)creates the data based on scenarios specified above.
func Setup(ctx context.Context, log *zap.Logger, db *metabase.DB) error {
	err1 := db.TestingDeleteAll(ctx)
	log.Info("delete all", zap.Error(err1))

	for _, scenario := range scenarios {
		log.Info("setting up scenario "+scenario.Name, zap.Int("objects", scenario.Setup.Expected()))
		data := scenario.Setup.Generate(nil)
		setBucketAndProject(data, DefaultProjectID, metabase.BucketName(scenario.Name))
		err := db.TestingBatchInsertObjects(ctx, data)
		if err != nil {
			return fmt.Errorf("%q failed to insert objects: %w", scenario.Name, err)
		}
	}

	err4 := db.UpdateTableStats(ctx)
	if err4 != nil {
		return errs.Wrap(err4)
	}

	return nil
}

// Benchmark runs benchmarks with the configured scenarios.
func Benchmark(ctx context.Context, log *zap.Logger, db *metabase.DB) (err error) {
	prefix := "bench-" + time.Now().Format("2006-01-02_15-04-05")
	var listFile *os.File

	if !*skipList {
		var err error
		listFile, err = os.Create(prefix + ".List.log")
		if err != nil {
			return errs.Wrap(err)
		}
		defer func() { _ = listFile.Close() }()
	}

	var rx *regexp.Regexp
	if *filter != "" {
		rx = regexp.MustCompile(*filter)
	}

	for _, scenario := range scenarios {
		slog := log.Named(scenario.Name)
		for _, pending := range []bool{false, true} {
			for _, recursive := range []bool{false, true} {
				for _, allVersions := range []bool{false, true} {
					for _, limit := range queryLimits {
						opts := metabase.ListObjects{
							ProjectID:   DefaultProjectID,
							BucketName:  metabase.BucketName(scenario.Name),
							Recursive:   recursive,
							Pending:     pending,
							AllVersions: allVersions,
							Limit:       limit,
						}

						conf := fmt.Sprintf("pending=%v,recursive=%v,all=%v,limit=%v", opts.Pending, opts.Recursive, opts.AllVersions, opts.Limit)
						testName := scenario.Name + "/" + conf
						if rx != nil && !rx.MatchString(testName) {
							continue
						}

						if !*skipList {
							err := benchmarkListObjects(ctx, slog, db, testName, opts, listFile)
							err = ignoreTimeoutOrCancel(err)
							if err != nil {
								return errs.Wrap(err)
							}
							_ = listFile.Sync()
						}

						if ctx.Err() != nil {
							return errs.Wrap(ctx.Err())
						}
					}
				}
			}
		}
	}
	return nil
}

// isCanceledOrTimeout returns true, when the error is a cancellation.
func isCanceledOrTimeout(err error) bool {
	return errs.IsFunc(err, func(err error) bool {
		return err == context.Canceled || err == context.DeadlineExceeded //nolint:errorlint,goerr113,err113
	})
}

// ignoreTimeoutOrCancel returns nil, when the operation was about canceling.
func ignoreTimeoutOrCancel(err error) error {
	if isCanceledOrTimeout(err) {
		return nil
	}
	return err
}

const maxTimePerBenchmark = 5 * time.Minute

func benchmarkListObjects(ctx context.Context, log *zap.Logger, db *metabase.DB, testName string, opts metabase.ListObjects, out io.Writer) error {
	if opts.Pending != opts.AllVersions {
		// not supported
		return nil
	}

	totalTime := time.Duration(0)

	for range repeat(*benchmarkCount) {
		startClock := time.Now()
		start := hrtime.Now()
		lctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		result, err := db.ListObjects(lctx, opts)
		cancel()
		finish := hrtime.Now()
		finishClock := time.Now()

		duration := finish - start
		if duration < 0 { // hrtime seems to not handle tsc wrapping nicely
			duration = finishClock.Sub(startClock)
		}
		totalTime += duration

		if err != nil {
			return errs.Wrap(err)
		}
		log.Info("ListObjects",
			zap.Duration("time", duration),
			zap.Int("result", len(result.Objects)),
			zap.Bool("pending", opts.Pending),
			zap.Bool("recursive", opts.Recursive),
			zap.Bool("all", opts.AllVersions),
			zap.Int("limit", opts.Limit),
		)

		line := fmt.Sprintf("Benchmark%v\t1\t%v ns/op", testName, duration.Nanoseconds())
		fmt.Println(line)
		_, _ = fmt.Fprintln(out, line)

		if totalTime > maxTimePerBenchmark {
			break
		}
	}

	return nil
}

// Generator is a generic implementation for generating raw objects.
type Generator interface {
	// Expected returns how many objects this generator generates.
	Expected() int
	// Generate generates objects.
	Generate([]metabase.RawObject) []metabase.RawObject
	// String returns textual description of the setup.
	String() string
}

// BasicObjects generates a number of objects with specified number of versions and pending.
type BasicObjects struct {
	Count    int
	Versions int
	Pending  int
}

// Expected returns how many objects this generator generates.
func (cfg BasicObjects) Expected() int { return cfg.Count * (cfg.Versions + cfg.Pending) }

// String returns textual description of the setup.
func (cfg BasicObjects) String() string {
	if cfg.Versions == 0 {
		return fmt.Sprintf("o%v{p%v}", cfg.Count, cfg.Pending)
	}
	if cfg.Pending == 0 {
		return fmt.Sprintf("o%v{v%v}", cfg.Count, cfg.Versions)
	}
	return fmt.Sprintf("o%v{v%v_p%v}", cfg.Count, cfg.Versions, cfg.Pending)
}

// Generate generates objects.
func (cfg BasicObjects) Generate(result []metabase.RawObject) []metabase.RawObject {
	result = slices.Grow(result, cfg.Expected())
	for i := range repeat(cfg.Count) {
		objectKey := strconv.Itoa(i)
		version := 10
		for range repeat(cfg.Versions) {
			version++
			result = append(result, mkObject(objectKey, version, metabase.CommittedVersioned))
		}
		for range repeat(cfg.Pending) {
			version++
			result = append(result, mkObject(objectKey, version, metabase.Pending))
		}
	}
	return result
}

// PrefixedObjects generates a number of objects with prefixes and objects between those prefixes.
type PrefixedObjects struct {
	Prefixes      int
	PerPrefix     BasicObjects
	BetweenPrefix BasicObjects
}

// Expected returns how many objects this generator generates.
func (cfg PrefixedObjects) Expected() int {
	return cfg.Prefixes * (cfg.PerPrefix.Expected() + cfg.BetweenPrefix.Expected())
}

// String returns textual description of the setup.
func (cfg PrefixedObjects) String() string {
	if cfg.BetweenPrefix.Expected() == 0 {
		return fmt.Sprintf("%v{x%v}", cfg.Prefixes, cfg.PerPrefix)
	}
	return fmt.Sprintf("%v{x%v_%v}", cfg.Prefixes, cfg.PerPrefix, cfg.BetweenPrefix)
}

// Generate generates objects.
func (cfg PrefixedObjects) Generate(result []metabase.RawObject) []metabase.RawObject {
	result = slices.Grow(result, cfg.Expected())

	for i := range repeat(cfg.Prefixes) {
		prefix := string([]rune{rune('a' + i)})

		start := len(result)
		result = cfg.PerPrefix.Generate(result)
		prependObjectKey(result[start:], prefix+"/")

		start = len(result)
		result = cfg.BetweenPrefix.Generate(result)
		prependObjectKey(result[start:], prefix+"z")
	}

	return result
}

func mkObject(objectKey string, version int, status metabase.ObjectStatus) metabase.RawObject {
	return metabase.RawObject{
		ObjectStream: metabase.ObjectStream{
			ObjectKey: metabase.ObjectKey(objectKey),
			Version:   metabase.Version(version),
			StreamID:  nextUUID(),
		},
		CreatedAt: time.Now(),
		Status:    status,
	}
}

// count is a helper to range over a number on Go 1.21 and older.
func repeat(n int) []struct{} { return make([]struct{}, n) }

func prependObjectKey(xs []metabase.RawObject, prefix string) {
	for i := range xs {
		xs[i].ObjectKey = metabase.ObjectKey(prefix) + xs[i].ObjectKey
	}
}

func setBucketAndProject(xs []metabase.RawObject, projectID uuid.UUID, bucketName metabase.BucketName) {
	for i := range xs {
		xs[i].ProjectID = projectID
		xs[i].BucketName = bucketName
	}
}

var uuidGenerate uint64

func nextUUID() uuid.UUID {
	uuidGenerate++
	return intToUUID(uuidGenerate)
}

func intToUUID(v uint64) (r uuid.UUID) {
	if v == 0 {
		v++
	}
	binary.LittleEndian.PutUint64(r[:], v)
	return r
}

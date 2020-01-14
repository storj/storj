// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/zeebo/errs"

	"storj.io/common/testcontext"
	"storj.io/storj/storage"
)

const (
	maxProblems = 10

	// the largest and deepest level-2 directory in the dataset
	largestLevel2Directory = "Peronosporales/hateless/"

	// the directory in the dataset with the most immediate children
	largestSingleDirectory = "Peronosporales/hateless/tod/unricht/sniveling/Puyallup/"
)

var (
	// see https://github.com/storj/test-path-corpus
	longBenchmarksData = flag.String("test-bench-long", "", "Run the long benchmark suite against eligible KeyValueStores using the given paths dataset")

	noInitDb  = flag.Bool("test-bench-long-noinit", false, "Don't import the large dataset for the long benchmarks; assume it is already loaded")
	noCleanDb = flag.Bool("test-bench-long-noclean", false, "Don't clean the long benchmarks KeyValueStore after running, for debug purposes")
)

func interpolateInput(input []byte) ([]byte, error) {
	output := make([]byte, 0, len(input))
	var bytesConsumed int
	var next byte

	for pos := 0; pos < len(input); pos += bytesConsumed {
		if input[pos] == '\\' {
			bytesConsumed = 2
			if pos+1 >= len(input) {
				return output, errs.New("encoding error in input: escape at end-of-string")
			}
			switch input[pos+1] {
			case 'x':
				if pos+3 >= len(input) {
					return output, errs.New("encoding error in input: incomplete \\x escape")
				}
				nextVal, err := strconv.ParseUint(string(input[pos+2:pos+4]), 16, 8)
				if err != nil {
					return output, errs.New("encoding error in input: invalid \\x escape: %v", err)
				}
				next = byte(nextVal)
				bytesConsumed = 4
			case 't':
				next = '\t'
			case 'n':
				next = '\n'
			case 'r':
				next = '\r'
			case '\\':
				next = '\\'
			default:
				next = input[pos+1]
			}
		} else {
			next = input[pos]
			bytesConsumed = 1
		}
		output = append(output, next)
	}
	return output, nil
}

// KVInputIterator is passed to the BulkImport method on BulkImporter-satisfying objects. It will
// iterate over a fairly large list of paths that should be imported for testing purposes.
type KVInputIterator struct {
	itemNo     int
	scanner    *bufio.Scanner
	fileName   string
	err        error
	reachedEnd bool
	closeFunc  func() error
}

func newKVInputIterator(pathToFile string) (*KVInputIterator, error) {
	kvi := &KVInputIterator{fileName: pathToFile}
	pathData, err := os.Open(pathToFile)
	if err != nil {
		return nil, errs.New("Failed to open file with test data (expected at %q): %v", pathToFile, err)
	}
	var reader io.Reader = pathData
	if strings.HasSuffix(pathToFile, ".gz") {
		gzReader, err := gzip.NewReader(pathData)
		if err != nil {
			return nil, errs.Combine(
				errs.New("Failed to create gzip reader: %v", err),
				pathData.Close())
		}
		kvi.closeFunc = func() error { return errs.Combine(gzReader.Close(), pathData.Close()) }
		reader = gzReader
	} else {
		kvi.closeFunc = pathData.Close
	}
	kvi.scanner = bufio.NewScanner(reader)
	return kvi, nil
}

// Next should be called by BulkImporter instances in order to advance the iterator. It fills in
// a storage.ListItem instance, and returns a boolean indicating whether to continue. When false is
// returned, iteration should stop and nothing is expected to be changed in item.
func (kvi *KVInputIterator) Next(ctx context.Context, item *storage.ListItem) bool {
	if !kvi.scanner.Scan() {
		kvi.reachedEnd = true
		kvi.err = kvi.scanner.Err()
		return false
	}
	if kvi.err != nil {
		return false
	}
	kvi.itemNo++
	parts := bytes.Split(kvi.scanner.Bytes(), []byte("\t"))
	if len(parts) != 3 {
		kvi.err = errs.New("Invalid data in %q on line %d: has %d fields", kvi.fileName, kvi.itemNo, len(parts))
		return false
	}
	k, err := interpolateInput(parts[1])
	if err != nil {
		kvi.err = errs.New("Failed to read key data from %q on line %d: %v", kvi.fileName, kvi.itemNo, err)
		return false
	}
	v, err := interpolateInput(parts[2])
	if err != nil {
		kvi.err = errs.New("Failed to read value data from %q on line %d: %v", kvi.fileName, kvi.itemNo, err)
		return false
	}
	item.Key = storage.Key(k)
	item.Value = storage.Value(v)
	item.IsPrefix = false
	return true
}

// Error() returns the last error encountered while iterating over the input file. This must be
// checked after iteration completes, at least.
func (kvi *KVInputIterator) Error() error {
	return kvi.err
}

func openTestData(tb testing.TB) *KVInputIterator {
	tb.Helper()
	inputIter, err := newKVInputIterator(*longBenchmarksData)
	if err != nil {
		tb.Fatal(err)
	}
	return inputIter
}

// BenchmarkPathOperationsInLargeDb runs the "long benchmarks" suite for KeyValueStore instances.
func BenchmarkPathOperationsInLargeDb(b *testing.B, store storage.KeyValueStore) {
	if *longBenchmarksData == "" {
		b.Skip("Long benchmarks not enabled.")
	}

	ctx := testcontext.New(b)
	defer ctx.Cleanup()

	initStore(b, ctx, store)

	doTest := func(name string, testFunc func(*testing.B, *testcontext.Context, storage.KeyValueStore)) {
		b.Run(name, func(bb *testing.B) {
			for i := 0; i < bb.N; i++ {
				testFunc(bb, ctx, store)
			}
		})
	}

	doTest("DeepRecursive", deepRecursive)
	doTest("DeepNonRecursive", deepNonRecursive)
	doTest("ShallowRecursive", shallowRecursive)
	doTest("ShallowNonRecursive", shallowNonRecursive)
	doTest("TopRecursiveLimit", topRecursiveLimit)
	doTest("TopRecursiveStartAt", topRecursiveStartAt)
	doTest("TopNonRecursive", topNonRecursive)

	cleanupStore(b, ctx, store)
}

func importBigPathset(tb testing.TB, ctx *testcontext.Context, store storage.KeyValueStore) {
	// make sure this is an empty db, or else refuse to run
	if !isEmptyKVStore(tb, ctx, store) {
		tb.Fatal("Provided KeyValueStore is not empty. The long benchmarks are destructive. Not running!")
	}

	inputIter := openTestData(tb)
	defer func() {
		if err := inputIter.closeFunc(); err != nil {
			tb.Logf("Failed to close test data stream: %v", err)
		}
	}()

	importer, ok := store.(BulkImporter)
	if ok {
		tb.Log("Performing bulk import...")
		err := importer.BulkImport(ctx, inputIter)

		if err != nil {
			errStr := "Provided KeyValueStore failed to import data"
			if inputIter.reachedEnd {
				errStr += " after iterating over all input data"
			} else {
				errStr += fmt.Sprintf(" after iterating over %d lines of input data", inputIter.itemNo)
			}
			tb.Fatalf("%s: %v", errStr, err)
		}
	} else {
		tb.Log("Performing manual import...")

		var item storage.ListItem
		for inputIter.Next(ctx, &item) {
			if err := store.Put(ctx, item.Key, item.Value); err != nil {
				tb.Fatalf("Provided KeyValueStore failed to insert data (%q, %q): %v", item.Key, item.Value, err)
			}
		}
	}
	if err := inputIter.Error(); err != nil {
		tb.Fatalf("Failed to iterate over input data during import. Error was %v", err)
	}
	if !inputIter.reachedEnd {
		tb.Fatal("Provided KeyValueStore failed to exhaust input iterator")
	}
}

func initStore(b *testing.B, ctx *testcontext.Context, store storage.KeyValueStore) {
	b.Helper()

	if !*noInitDb {
		// can't find a way to run the import and cleanup as sub-benchmarks, while still requiring
		// that they be run once and only once, and aborting the whole benchmark if import fails.
		// we don't want the time it takes to count against the first sub-benchmark only, so we
		// stop the timer. however, we do care about the time that import and cleanup take, though,
		// so we'll at least log it.
		b.StopTimer()
		tStart := time.Now()
		importBigPathset(b, ctx, store)
		b.Logf("importing took %s", time.Since(tStart).String())
		b.StartTimer()
	}
}

func cleanupStore(b *testing.B, ctx *testcontext.Context, store storage.KeyValueStore) {
	b.Helper()
	if !*noCleanDb {
		tStart := time.Now()
		cleanupBigPathset(b, ctx, store)
		b.Logf("cleanup took %s", time.Since(tStart).String())
	}
}

type verifyOpts struct {
	iterateOpts   storage.IterateOptions
	doIterations  int
	batchSize     int
	expectCount   int
	expectLastKey storage.Key
}

func benchAndVerifyIteration(b *testing.B, ctx *testcontext.Context, store storage.KeyValueStore, opts *verifyOpts) {
	problems := 0
	iteration := 0

	errMsg := func(tmpl string, args ...interface{}) string {
		errMsg1 := fmt.Sprintf(tmpl, args...)
		return fmt.Sprintf("[on iteration %d/%d, with opts %+v]: %s", iteration, opts.doIterations, opts.iterateOpts, errMsg1)
	}

	errorf := func(tmpl string, args ...interface{}) {
		b.Error(errMsg(tmpl, args...))
		problems++
		if problems > maxProblems {
			b.Fatal("Too many problems")
		}
	}

	fatalf := func(tmpl string, args ...interface{}) {
		b.Fatal(errMsg(tmpl, args...))
	}

	expectRemaining := opts.expectCount
	totalFound := 0
	var lastKey storage.Key
	var bytesTotal int64
	lookupSize := opts.batchSize

	for iteration = 1; iteration <= opts.doIterations; iteration++ {
		results, err := iterateItems(ctx, store, opts.iterateOpts, lookupSize)
		if err != nil {
			fatalf("Failed to call iterateItems(): %v", err)
		}
		if len(results) == 0 {
			// we can't continue to iterate
			fatalf("iterateItems() got 0 items")
		}
		if len(results) > lookupSize {
			fatalf("iterateItems() returned _more_ items than limit: %d>%d", len(results), lookupSize)
		}
		if iteration > 0 && results[0].Key.Equal(lastKey) {
			// fine and normal
			results = results[1:]
		}
		expectRemaining -= len(results)
		if len(results) != opts.batchSize && expectRemaining != 0 {
			errorf("iterateItems read %d items instead of %d", len(results), opts.batchSize)
		}
		for n, result := range results {
			totalFound++
			bytesTotal += int64(len(result.Key)) + int64(len(result.Value))
			if result.Key.IsZero() {
				errorf("got an empty key among the results at n=%d!", n)
				continue
			}
			if result.Key.Equal(lastKey) {
				errorf("got the same key (%q) twice in a row, not on a lookup boundary!", lastKey)
			}
			if result.Key.Less(lastKey) {
				errorf("KeyValueStore returned items out of order! %q < %q", result.Key, lastKey)
			}
			if result.IsPrefix {
				if !result.Value.IsZero() {
					errorf("Expected no metadata for IsPrefix item %q, but got %q", result.Key, result.Value)
				}
				if result.Key[len(result.Key)-1] != byte('/') {
					errorf("Expected key for IsPrefix item %q to end in /, but it does not", result.Key)
				}
			} else {
				valAsNum, err := strconv.ParseUint(string(result.Value), 10, 32)
				if err != nil {
					errorf("Expected metadata for key %q to hold a decimal integer, but it has %q", result.Key, result.Value)
				} else if int(valAsNum) != len(result.Key) {
					errorf("Expected metadata for key %q to be %d, but it has %q", result.Key, len(result.Key), result.Value)
				}
			}
			lastKey = result.Key
		}
		if len(results) > 0 {
			opts.iterateOpts.First = results[len(results)-1].Key
		}
		lookupSize = opts.batchSize + 1 // subsequent queries will start with the last element previously returned
	}
	b.SetBytes(bytesTotal)

	if totalFound != opts.expectCount {
		b.Fatalf("Expected to read %d items in total, but got %d", opts.expectCount, totalFound)
	}
	if !opts.expectLastKey.IsZero() {
		if diff := cmp.Diff(opts.expectLastKey.String(), lastKey.String()); diff != "" {
			b.Fatalf("KeyValueStore got wrong last item: (-want +got)\n%s", diff)
		}
	}
}

func deepRecursive(b *testing.B, ctx *testcontext.Context, store storage.KeyValueStore) {
	opts := &verifyOpts{
		iterateOpts: storage.IterateOptions{
			Prefix:  storage.Key(largestLevel2Directory),
			Recurse: true,
		},
	}

	// these are not expected to exhaust all available items
	opts.doIterations = 500
	opts.batchSize = storage.LookupLimit
	opts.expectCount = opts.doIterations * opts.batchSize

	// verify with:
	//     select encode(fullpath, 'escape') from (
	//         select rank() over (order by fullpath), fullpath from pathdata where fullpath > $1::bytea
	//     ) x where rank = ($2 * $3);
	// where $1 = largestLevel2Directory, $2 = doIterations, and $3 = batchSize
	opts.expectLastKey = storage.Key("Peronosporales/hateless/tod/extrastate/firewood/renomination/cletch/herotheism/aluminiferous/nub")

	benchAndVerifyIteration(b, ctx, store, opts)
}

func deepNonRecursive(b *testing.B, ctx *testcontext.Context, store storage.KeyValueStore) {
	opts := &verifyOpts{
		iterateOpts: storage.IterateOptions{
			Prefix:  storage.Key(largestLevel2Directory),
			Recurse: false,
		},
		doIterations: 1,
		batchSize:    10000,
	}

	// verify with:
	//     select count(*) from list_directory(''::bytea, $1::bytea) ld(fp, md);
	// where $1 is largestLevel2Directory
	opts.expectCount = 119

	// verify with:
	//     select encode(fp, 'escape') from (
	//         select * from list_directory(''::bytea, $1::bytea) ld(fp, md)
	//     ) x order by fp desc limit 1;
	// where $1 is largestLevel2Directory
	opts.expectLastKey = storage.Key("Peronosporales/hateless/xerophily/")

	benchAndVerifyIteration(b, ctx, store, opts)
}

func shallowRecursive(b *testing.B, ctx *testcontext.Context, store storage.KeyValueStore) {
	opts := &verifyOpts{
		iterateOpts: storage.IterateOptions{
			Prefix:  storage.Key(largestSingleDirectory),
			Recurse: true,
		},
	}

	// verify with:
	//     select count(*) from pathdata
	//         where fullpath > $1::bytea and fullpath < bytea_increment($1::bytea);
	// where $1 = largestSingleDirectory
	opts.expectCount = 18574

	// verify with:
	//     select convert_from(fullpath, 'UTF8') from pathdata
	//         where fullpath > $1::bytea and fullpath < bytea_increment($1::bytea)
	//         order by fullpath desc limit 1;
	// where $1 = largestSingleDirectory
	opts.expectLastKey = storage.Key("Peronosporales/hateless/tod/unricht/sniveling/Puyallup/élite")

	// i didn't plan it this way, but expectedCount happens to have some nicely-sized factors for
	// our purposes with no messy remainder. 74 * 251 = 18574
	opts.doIterations = 74
	opts.batchSize = 251

	benchAndVerifyIteration(b, ctx, store, opts)
}

func shallowNonRecursive(b *testing.B, ctx *testcontext.Context, store storage.KeyValueStore) {
	opts := &verifyOpts{
		iterateOpts: storage.IterateOptions{
			Prefix:  storage.Key(largestSingleDirectory),
			Recurse: false,
		},
		doIterations: 2,
		batchSize:    10000,
	}

	// verify with:
	//     select count(*) from list_directory(''::bytea, $1::bytea) ld(fp, md);
	// where $1 is largestSingleDirectory
	opts.expectCount = 18574

	// verify with:
	//     select encode(fp, 'escape') from (
	//         select * from list_directory(''::bytea, $1::bytea) ld(fp, md)
	//     ) x order by fp desc limit 1;
	// where $1 = largestSingleDirectory
	opts.expectLastKey = storage.Key("Peronosporales/hateless/tod/unricht/sniveling/Puyallup/élite")

	benchAndVerifyIteration(b, ctx, store, opts)
}

func topRecursiveLimit(b *testing.B, ctx *testcontext.Context, store storage.KeyValueStore) {
	opts := &verifyOpts{
		iterateOpts: storage.IterateOptions{
			Recurse: true,
		},
		doIterations: 100,
		batchSize:    10000,
	}

	// not expected to exhaust items
	opts.expectCount = opts.doIterations * opts.batchSize

	// verify with:
	//     select encode(fullpath, 'escape') from (
	//         select rank() over (order by fullpath), fullpath from pathdata
	//     ) x where rank = $1;
	// where $1 = expectCount
	opts.expectLastKey = storage.Key("nonresuscitation/synchronically/bechern/hemangiomatosis")

	benchAndVerifyIteration(b, ctx, store, opts)
}

func topRecursiveStartAt(b *testing.B, ctx *testcontext.Context, store storage.KeyValueStore) {
	opts := &verifyOpts{
		iterateOpts: storage.IterateOptions{
			Recurse: true,
		},
		doIterations: 100,
		batchSize:    10000,
	}

	// this is pretty arbitrary. just the key 100 positions before the end of the Peronosporales/hateless/ dir.
	opts.iterateOpts.First = storage.Key("Peronosporales/hateless/warrener/anthropomancy/geisotherm/wickerwork")

	// not expected to exhaust items
	opts.expectCount = opts.doIterations * opts.batchSize

	// verify with:
	//     select encode(fullpath, 'escape') from (
	//         select fullpath from pathdata where fullpath >= $1::bytea order by fullpath limit $2
	//     ) x order by fullpath desc limit 1;
	// where $1 = iterateOpts.First and $2 = expectCount
	opts.expectLastKey = storage.Key("raptured/heathbird/histrionism/vermifugous/barefaced/beechdrops/lamber/phlegmatic/blended/Gershon/scallop/burglarproof/incompensated/allanite/alehouse/embroilment/lienotoxin/monotonically/cumbersomeness")

	benchAndVerifyIteration(b, ctx, store, opts)
}

func topNonRecursive(b *testing.B, ctx *testcontext.Context, store storage.KeyValueStore) {
	opts := &verifyOpts{
		iterateOpts: storage.IterateOptions{
			Recurse: false,
		},
		doIterations: 1,
		batchSize:    10000,
	}

	// verify with:
	//     select count(*) from list_directory(''::bytea, ''::bytea);
	opts.expectCount = 21

	// verify with:
	//     select encode(fp, 'escape') from (
	//         select * from list_directory(''::bytea, ''::bytea) ld(fp, md)
	//     ) x order by fp desc limit 1;
	opts.expectLastKey = storage.Key("vejoces")

	benchAndVerifyIteration(b, ctx, store, opts)
}

func cleanupBigPathset(tb testing.TB, ctx *testcontext.Context, store storage.KeyValueStore) {
	if *noCleanDb {
		tb.Skip("Instructed not to clean up this KeyValueStore after long benchmarks are complete.")
	}

	cleaner, ok := store.(BulkCleaner)
	if ok {
		tb.Log("Performing bulk cleanup...")
		err := cleaner.BulkDeleteAll(ctx)

		if err != nil {
			tb.Fatalf("Provided KeyValueStore failed to perform bulk delete: %v", err)
		}
	} else {
		inputIter := openTestData(tb)
		defer func() {
			if err := inputIter.closeFunc(); err != nil {
				tb.Logf("Failed to close input data stream: %v", err)
			}
		}()

		tb.Log("Performing manual cleanup...")

		var item storage.ListItem
		for inputIter.Next(ctx, &item) {
			if err := store.Delete(ctx, item.Key); err != nil {
				tb.Fatalf("Provided KeyValueStore failed to delete item %q during cleanup: %v", item.Key, err)
			}
		}
		if err := inputIter.Error(); err != nil {
			tb.Fatalf("Failed to iterate over input data: %v", err)
		}
	}
}

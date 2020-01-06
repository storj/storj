// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

// Tests whether the uplink tool correctly times out when one of the storage nodes it's talking to
// suddenly stops responding. In particular, this currently tests that happening during a Delete
// operation, because that is where we have observed indefinite hangs before.

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/memory"
)

var (
	numTries      = flag.Int("num-tries", 20, "number of tries to cause a hang")
	bucketName    = flag.String("bucket", "bukkit", "name of bucket to use for test")
	deleteTimeout = flag.Duration("timeout", 60*time.Second, "how long to wait for a delete to succeed or time out")

	fileSize memory.Size = 5 * memory.MiB

	tryAgain = errs.New("test needs to run again")
)

func init() {
	flag.Var(&fileSize, "file-size", "size of test file to use")
}

type randDefaultSource struct{}

func (randSource *randDefaultSource) Read(p []byte) (int, error) {
	return rand.Read(p)
}

func makeRandomContentsFile(path string, size memory.Size) (err error) {
	outFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		err = errs.Combine(err, outFile.Close())
	}()
	if _, err := io.CopyN(outFile, &randDefaultSource{}, int64(size)); err != nil {
		return err
	}
	return nil
}

type uplinkRunner struct {
	execName  string
	configDir string
	logLevel  string
}

// Run runs the uplink executable with the given arguments, and hands back its
// output as well as an error if there were any problems with the execution or if
// the uplink exited non-zero.
func (ur *uplinkRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmdArgs := []string{"--config-dir", ur.configDir, "--log.level", ur.logLevel}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.CommandContext(ctx, ur.execName, cmdArgs...)
	return cmd.CombinedOutput()
}

// skip the first four whitespace-delimited fields and keep the rest
var lsOutputRegexp = regexp.MustCompile(`(?m)^\s*(?:\S+\s+){4}(.*)$`)

func (ur *uplinkRunner) doesRemoteExist(remotePath string) (bool, error) {
	pathParts := strings.Split(remotePath, "/")
	if len(pathParts) < 2 {
		return false, errs.New("invalid remote path %q", remotePath)
	}
	bucketAndDir := strings.Join(pathParts[:len(pathParts)-1], "/")
	filenamePart := []byte(pathParts[len(pathParts)-1])
	output, err := ur.Run(nil, "ls", bucketAndDir)
	if err != nil {
		return false, err
	}
	for _, matches := range lsOutputRegexp.FindAllSubmatch(output, -1) {
		if bytes.Equal(matches[1], filenamePart) {
			return true, nil
		}
	}
	return false, nil
}

func storeFileAndCheck(uplink *uplinkRunner, srcFile, dstFile string) error {
	if _, err := uplink.Run(nil, "cp", srcFile, dstFile); err != nil {
		return errs.New("Could not copy file into storj-sim network: %v", err)
	}
	if exists, err := uplink.doesRemoteExist(dstFile); err != nil {
		return errs.New("Could not check if file exists: %v", err)
	} else if !exists {
		return errs.New("Copied file not present in storj-sim network!")
	}
	return nil
}

func stallNode(ctx context.Context, proc *os.Process) {
	// send node a SIGSTOP, which causes it to freeze as if being traced
	proc.Signal(syscall.SIGSTOP)
	// until the context is done
	<-ctx.Done()
	// then let the node continue again
	proc.Signal(syscall.SIGCONT)
}

func deleteWhileStallingAndCheck(uplink *uplinkRunner, dstFile string, nodeProc *os.Process) error {
	ctx, cancel := context.WithTimeout(context.Background(), *deleteTimeout)
	defer cancel()

	go stallNode(ctx, nodeProc)

	output, err := uplink.Run(ctx, "rm", dstFile)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			// (uplink did not time out, but this test did)
			return errs.New("uplink DID NOT time out waiting for stalled node 0 while issuing a delete")
		}
		return errs.New("Unexpected error trying to delete file %q from storj-sim network: %v", dstFile, err)
	}
	if exists, err := uplink.doesRemoteExist(dstFile); err != nil {
		return errs.New("Failed to check if remote file %q was deleted: %v", dstFile, err)
	} else if exists {
		return errs.New("Deleted file still present in storj-sim network!")
	}
	if strings.Contains(string(output), "context deadline exceeded") {
		// the uplink correctly timed out when one of the target nodes was stalled! all is well
		return nil
	}
	// delete worked fine, which means our stall didn't hit at the right time and we need to try again
	return tryAgain
}

func runTest() error {
	// check run environment
	configDir := os.Getenv("GATEWAY_0_DIR")
	if configDir == "" {
		return errs.New("This test should be run under storj-sim test ($GATEWAY_0_DIR not found).")
	}
	nodePid, err := strconv.Atoi(os.Getenv("STORAGENODE_0_PID"))
	if err != nil {
		return errs.New("Empty or invalid $STORAGENODE_0_PID: %v", err)
	}
	nodeProc, err := os.FindProcess(nodePid)
	if err != nil {
		return errs.New("No such process %v! $STORAGENODE_0_PID is wrong", nodePid)
	}

	// set up test
	uplink := &uplinkRunner{
		execName:  "uplink",
		configDir: configDir,
		logLevel:  "error",
	}
	tempDir, err := ioutil.TempDir("", "storj-test-network-stalls.")
	if err != nil {
		return err
	}
	bucket := "sj://" + *bucketName
	srcFile := filepath.Join(tempDir, "to-storj-sim")
	dstFile := bucket + "/in-storj-sim"
	if err := makeRandomContentsFile(srcFile, fileSize); err != nil {
		return errs.New("could not create test file with random contents: %v", err)
	}
	if _, err := uplink.Run(nil, "mb", bucket); err != nil {
		return errs.New("could not create test bucket: %v", err)
	}
	defer func() {
		// explicitly ignoring errors here; we don't much care if they fail,
		// because this is best-effort
		_, _ = uplink.Run(nil, "rm", dstFile)
		_, _ = uplink.Run(nil, "rb", bucket)
	}()

	// run test
	for i := 0; i < *numTries; i++ {
		fmt.Printf("%d\n", i)

		if err := storeFileAndCheck(uplink, srcFile, dstFile); err != nil {
			return err
		}

		err := deleteWhileStallingAndCheck(uplink, dstFile, nodeProc)
		if err == nil {
			// success!
			break
		}
		if err != tryAgain {
			// unexpected error
			return err
		}
	}

	// clean up test. this part isn't deferred and run unconditionally because
	// we want to inspect things when the test has failed.
	return os.RemoveAll(tempDir)
}

func main() {
	flag.Parse()

	if err := runTest(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("SUCCESS")
}

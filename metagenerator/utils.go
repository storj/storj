package metagenerator

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Netflix/go-expect"
	"github.com/google/goterm/term"
	"storj.io/common/uuid"
)

func prettyPrint(data interface{}) {
	b, _ := json.Marshal(data)

	var out bytes.Buffer
	json.Indent(&out, b, "", "  ")
	fmt.Println(out.String())
}

func putFile(record *Record) error {
	localPath := filepath.Join("/tmp", strings.ReplaceAll(record.Path, "/", "_"))
	record.Path = "sj://" + Label + record.Path

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

	return os.Remove(localPath)
}

func deleteFile(record *Record) error {
	cmd := exec.Command("uplink", "rm", record.Path)
	cmd.Dir = clusterPath
	out, err := cmd.CombinedOutput()
	fmt.Println(string(out))

	return err
}

func UplinkSetup(satelliteAddress, apiKey string) {
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
	c.Send(Label + "\n")
	c.ExpectString("Enter API key or Access grant:")
	c.Send(apiKey + "\n")
	c.ExpectString("Satellite address:")
	c.Send(satelliteAddress + "\n")
	c.ExpectString("Passphrase:")
	c.Send(Label + "\n")
	c.ExpectString("Again:")
	c.Send(Label + "\n")
	c.ExpectString("Would you like to disable encryption for object keys (allows lexicographical sorting of objects in listings)? (y/N):")
	c.Send("y\n")
	c.ExpectString("Would you like S3 backwards-compatible Gateway credentials? (y/N):")
	c.Send("y\n")
	fmt.Println(term.Greenf("Uplink setup done"))
}

func GeneratorSetup(sharedValues float64, bS, wN, tR int, apiKey, dbEndpoint, metaSearchEndpoint, mode string) (projectId string, db *sql.DB, ctx context.Context) {
	// Connect to CockroachDB
	var err error
	db, err = sql.Open("postgres", dbEndpoint)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to database: %v", err))
	}
	ctx = context.Background()

	if mode == ApiMode {
		//Create bucket
		cmd := exec.Command("uplink", "mb", "sj://benchmarks")

		out, err := cmd.CombinedOutput()
		if err != nil && !strings.Contains(string(out), "bucket already exists") {
			panic(err.Error())
		}
		projectId = GetProjectId(ctx, db).String()
	} else {
		pId, _ := uuid.New()
		projectId = pId.String()
	}

	// Initialize batch generator
	batchGen := NewBatchGenerator(
		db,
		sharedValues,
		bS,
		wN,
		tR,
		GetPathCount(ctx, db),
		projectId,
		apiKey,
		mode,
		metaSearchEndpoint,
	)

	// Generate and insert/debug records
	startTime := time.Now()

	if err := batchGen.GenerateAndInsert(ctx, totalRecords); err != nil {
		panic(fmt.Sprintf("failed to generate records: %v", err))
	}

	fmt.Printf("Generated %v records in %v\n", tR, time.Since(startTime))
	return
}

func Clean() {
	//Remove bucket
	cmd := exec.Command("uplink", "rb", "sj://"+Label, "--force")
	cmd.Dir = clusterPath

	out, err := cmd.CombinedOutput()
	fmt.Println(string(out))
	if err != nil {
		panic(err.Error())
	}
}

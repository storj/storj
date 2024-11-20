package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"testing"

	"storj.io/storj/metagenerator"
)

const (
	apiKey    = "15XZjcVqxQeggDyDpPhqJvMUB6NtQ1CiuW6mAwzRAVNE5gtr7Yh12MdtqvVbYQ9rvCadeve1f2LGiB53QnFyVV9CTY5HAv3jtFvtnKiVvehh4Dz9jwYx6yhV5bD1wGBrADuKCkQxa"
	projectId = "9088e8cc-d344-4767-8e07-901abc2734b6"
)

/*
var totalRecords = []int{
	100_000,
	1_000_000,
	10_000_000,
	100_000_000,
}
*/

func setupSuite(tb testing.TB) func(tb testing.TB) {
	// Connect to CockroachDB
	db, err := sql.Open("postgres", defaultDbEndpoint)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to database: %v", err))
	}
	ctx := context.Background()

	log.Println("setup suite")
	tR, _ := strconv.Atoi(os.Getenv("TR"))
	wN, _ := strconv.Atoi(os.Getenv("WN"))
	bS, _ := strconv.Atoi(os.Getenv("BS"))
	metagenerator.GeneratorSetup(sharedFields, bS, wN, tR, apiKey, projectId, defaultMetasearchAPI, db, ctx)

	// Return a function to teardown the test
	return func(tb testing.TB) {
		log.Println("teardown suite")
		metagenerator.CleanTable(ctx, db)
		db.Close()
	}
}

func BenchmarkSimpleQuery(b *testing.B) {
	teardownSuite := setupSuite(b)
	defer teardownSuite(b)
	for _, n := range metagenerator.MatchingEntries {
		if totalRecords > n {
			break
		}
		b.Run(fmt.Sprintf("matching_entries_%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				val := fmt.Sprintf("benchmarkValue_%v", n)
				b.ResetTimer()
				res, err := metagenerator.SearchMeta(metagenerator.Request{
					Path: fmt.Sprintf("sj://%s/", metagenerator.Label),
					Match: map[string]any{
						"field_" + val: val,
					},
				}, apiKey, projectId, defaultMetasearchAPI)
				b.StopTimer()

				if err != nil {
					panic(err)
				}
				var resp metagenerator.Response
				err = json.Unmarshal(res, &resp)
				if err != nil {
					panic(err)
				}
				fmt.Printf("Got %v entries\n", len(resp.Results))
			}
		})
	}
}

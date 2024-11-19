package main

import (
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

func setupSuite(tb testing.TB) func(tb testing.TB) {
	log.Println("setup suite")
	tR, _ := strconv.Atoi(os.Getenv("TR"))
	wN, _ := strconv.Atoi(os.Getenv("WN"))
	bS, _ := strconv.Atoi(os.Getenv("BS"))
	db, ctx := metagenerator.GeneratorSetup(sharedValues, bS, wN, tR, apiKey, projectId, defaultDbEndpoint, defaultMetasearchAPI)

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

	b.ResetTimer()
	err := metagenerator.SearchMeta(metagenerator.Query{
		Path: fmt.Sprintf("sj://%s/", metagenerator.Label),
		Match: map[string]any{
			"field_0": "purple",
		},
	}, apiKey, projectId, defaultMetasearchAPI)

	if err != nil {
		panic(err)
	}
}

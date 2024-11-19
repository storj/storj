package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"testing"

	"storj.io/storj/metagenerator"
)

var (
	apiKey    string
	projectId string
)

func init() {
	// Refactor to depend on mode
	// satelliteAddress := os.Getenv("SA")
	// metagenerator.UplinkSetup(satelliteAddress, apiKey)
	apiKey = os.Getenv("AK")
}

func setupSuite(tb testing.TB) func(tb testing.TB) {
	log.Println("setup suite")
	tR, _ := strconv.Atoi(os.Getenv("TR"))
	wN, _ := strconv.Atoi(os.Getenv("WN"))
	bS, _ := strconv.Atoi(os.Getenv("BS"))
	pId, db, ctx := metagenerator.GeneratorSetup(sharedValues, bS, wN, tR, apiKey, defaultDbEndpoint, defaultMetasearchAPI, metagenerator.DbMode)
	projectId = pId

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
			"filed_0": "purple",
		},
	}, apiKey, projectId, defaultMetasearchAPI)

	if err != nil {
		panic(err)
	}
}

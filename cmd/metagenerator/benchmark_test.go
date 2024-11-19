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
	satelliteAddress := os.Getenv("SA")
	apiKey = os.Getenv("AK")
	metagenerator.UplinkSetup(satelliteAddress, apiKey)
}

func setupSuite(tb testing.TB) func(tb testing.TB) {
	log.Println("setup suite")
	tR, _ := strconv.Atoi(os.Getenv("TR"))
	wN, _ := strconv.Atoi(os.Getenv("WN"))
	bS, _ := strconv.Atoi(os.Getenv("BS"))
	projectId = metagenerator.GeneratorSetup(sharedValues, bS, wN, tR, apiKey, defaultDbEndpoint, defaultMetasearchAPI)

	// Return a function to teardown the test
	return func(tb testing.TB) {
		log.Println("teardown suite")
		metagenerator.Clean()
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

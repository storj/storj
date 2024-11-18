package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"testing"
)

var (
	bG     *BatchGenerator
	apiKey string
)

func init() {
	satelliteAddress := os.Getenv("SA")
	apiKey = os.Getenv("AK")
	uplinkSetup(satelliteAddress, apiKey)
}

func setupSuite(tb testing.TB) func(tb testing.TB) {
	log.Println("setup suite")
	tR, _ := strconv.Atoi(os.Getenv("TR"))
	wN, _ := strconv.Atoi(os.Getenv("WN"))
	bS, _ := strconv.Atoi(os.Getenv("BS"))
	bG = generatorSetup(bS, wN, tR, apiKey)

	// Return a function to teardown the test
	return func(tb testing.TB) {
		log.Println("teardown suite")
		clean()
	}
}

func BenchmarkSimpleQuery(b *testing.B) {
	teardownSuite := setupSuite(b)
	defer teardownSuite(b)

	b.ResetTimer()
	err := bG.searchMeta(Query{
		Path:  fmt.Sprintf("sj://%s/", label),
		Query: `{"field_0":"purple"}`,
	})

	if err != nil {
		panic(err)
	}
}

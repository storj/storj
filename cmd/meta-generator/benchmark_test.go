package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"testing"
)

const (
	apiKeyB           = "15XZmgxFDiGo486PHG3QS7FfMEM4UmGieqKva8Xw5cw7pWa4QmwpYBRtCdfz7EQgpe97Tt3DUyiki38aor9AjDB5YU9nxa5ALyLsi5LjfZ2fMc7m5cs9SFFDuEWSGBWfRZcrEbxgb"
	satelliteAddressB = "12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4@satellite-api:7777"
)

var bG *BatchGenerator

func init() {
	uplinkSetup(satelliteAddressB, apiKeyB)
}

func setupSuite(tb testing.TB) func(tb testing.TB) {
	log.Println("setup suite")
	tR, _ := strconv.Atoi(os.Getenv("TR"))
	wN, _ := strconv.Atoi(os.Getenv("WN"))
	bS, _ := strconv.Atoi(os.Getenv("BS"))
	bG = generatorSetup(bS, wN, tR, apiKeyB)

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

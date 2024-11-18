package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"testing"
)

const (
	apiKeyB           = "15XZi7qUgrz28jZbgaMEZJ5TXa1WgfmDitAAyAdUm42d9eG6MfZvEQbEfV7xjbTgsZCnbftmD7qpK5VpokVMPaahmZiXHqs9xffnRo3wrram8BuMhKZJ7gPRexAvDrUHtD1H8EWF5"
	satelliteAddressB = "12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4@satellite-api:7777"
)

func init() {
	uplinkSetup(satelliteAddressB, apiKeyB)
}

func setupSuite(tb testing.TB) func(tb testing.TB) {
	log.Println("setup suite")
	tR, _ := strconv.Atoi(os.Getenv("TR"))
	wN, _ := strconv.Atoi(os.Getenv("WN"))
	bS, _ := strconv.Atoi(os.Getenv("BS"))
	generatorSetup(bS, wN, tR, apiKeyB)

	// Return a function to teardown the test
	return func(tb testing.TB) {
		log.Println("teardown suite")
		clean()
	}
}

func BenchmarkSimpleQuery(b *testing.B) {
	teardownSuite := setupSuite(b)
	defer teardownSuite(b)

	fmt.Println("some tests")
}

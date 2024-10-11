// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"fmt"
	"math"
	mathrand "math/rand"
	"os"
)

// main implements a simple (and slow) prime number generator.
func main() {
	dest := bytes.Buffer{}

	_, err := dest.WriteString(`// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

// Run "go run ./genprimes" to regenerate these values.
var primes = []uint64{
`)
	if err != nil {
		panic(err)
	}

	min := uint64(1 << 32)
	requiredPrimes := 32
	for {
		n := mathrand.Uint64()
		if n < min {
			continue
		}
		prime := true
		squareRoot := uint64(math.Floor(math.Sqrt(float64(n))))
		for i := uint64(2); i < squareRoot; i++ {
			if n%i == 0 {
				prime = false
				break
			}

		}
		if prime {
			fmt.Println("found a prime", n)
			requiredPrimes--
			_, err = dest.WriteString(fmt.Sprintf("   %d,\n", n))
			if err != nil {
				panic(err)
			}
		}
		if requiredPrimes == 0 {
			break
		}
	}
	_, err = dest.WriteString("}")
	if err != nil {
		panic(err)
	}
	err = os.WriteFile("primes.go", dest.Bytes(), 0644)
	if err != nil {
		panic(err)
	}
}

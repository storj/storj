// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2_test

import (
	"fmt"
	"io"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"storj.io/storj/internal/sync2"
)

func ExampleThrottle() {
	throttle := sync2.NewThrottle()
	var wg sync.WaitGroup

	// consumer
	go func() {
		defer wg.Done()
		totalConsumed := int64(0)
		for {
			available, err := throttle.ConsumeOrWait(8)
			if err != nil {
				return
			}
			fmt.Println("- consuming ", available, " total=", totalConsumed)
			totalConsumed += available

			// do work for available amount
			time.Sleep(time.Duration(available) * time.Millisecond)
		}
	}()

	// producer
	go func() {
		defer wg.Done()

		step := int64(8)
		for total := int64(64); total >= 0; total -= step {
			err := throttle.ProduceAndWaitUntilBelow(step, step*3)
			if err != nil {
				return
			}

			fmt.Println("+ producing", step, " left=", total)
			time.Sleep(time.Duration(rand.Intn(8)) * time.Millisecond)
		}

		throttle.Fail(io.EOF)
	}()

	wg.Wait()

	fmt.Println("done", throttle.Err())
}

func TestThrottleBasic(t *testing.T) {
	throttle := sync2.NewThrottle()
	var stage int64
	c := make(chan error, 1)

	// consumer
	go func() {
		consume, _ := throttle.ConsumeOrWait(4)
		if v := atomic.LoadInt64(&stage); v != 1 || consume != 4 {
			c <- fmt.Errorf("did not block in time: %d / %d", v, consume)
			return
		}

		consume, _ = throttle.ConsumeOrWait(4)
		if v := atomic.LoadInt64(&stage); v != 1 || consume != 4 {
			c <- fmt.Errorf("did not block in time: %d / %d", v, consume)
			return
		}
		atomic.AddInt64(&stage, 2)
		c <- nil
	}()

	// slowly produce
	time.Sleep(time.Millisecond)
	// set stage to 1
	atomic.AddInt64(&stage, 1)
	_ = throttle.Produce(8)
	// wait until consumer consumes twice
	_ = throttle.WaitUntilBelow(3)
	// wait slightly for updating stage
	time.Sleep(time.Millisecond)

	if v := atomic.LoadInt64(&stage); v != 3 {
		t.Fatalf("did not unblock in time: %d", v)
	}

	if err := <-c; err != nil {
		t.Fatalf(err.Error())
	}
}

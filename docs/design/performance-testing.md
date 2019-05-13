# Performance Testing

## Abstract

This document proposes an initial environment to perform and measure automated benchmark results we can observe trends over time. 


## Background

We want to ensure the performance for uploads and downloads remains competitive and ideally world class. Our code base is changing rapidly and we’ve experienced degrading performance that we don’t have any visibility into what changed or when the performance dropped. 

## Design

### Goals
* Keep it simple at first but give us something we can keep extending in the future. 
* Measure both uploads and downloads and optionally replacements (causes a delete). 
* Measure different workloads for both upload and download. 100 byte, 1MB, 10MB, 100MB and 1GB files. 
* Measure each test 10 times and report the distribution graph of 50%, 75%, 90%, 99% percentiles. 
* Author the tests using Go’s built-in benchmarking tools so that developers can run them locally against storage-sim for quick iteration for developer testing.

### Current Environment
Our current build environment supports running a deploy target that deploys a satellite and 100 storage nodes. We can reuse this environment for these performance tests.

### Benchmark Schedule
Currently the master branch deploys to a developer environment with 1 satellite and 100 storage nodes every merge to master. We can try to fit our performance benchmarks into those but it’s very possible the performance benchmarks will take longer to get worthwhile measurements. We may have to do deployments and measurements on a schedule. For example once every hour.

## Implementation

### Authoring
Writing a benchmark that we want to measure will be done in a Go standard benchmark function which gives us the following output.

Foo-40   3000000	   432 ns/op    2.31 MB/s     0 B/op	   0 allocs/op
Bar-40   3000000	   404 ns/op    4.95 MB/s     0 B/op	   0 allocs/op
Baz-40   3000000	   402 ns/op    9.94 MB/s     0 B/op	   0 allocs/op

For integration tests it’s possible to re-use the MB/s ability of the go benchmarks to report operations/second for easy graphing purposes when needed.

### Reporting
There is already a repository with easy to use R scripts that plot various graphs from standard Go benchmark results like the ones above in the authoring section. It also supports HdrHistogram files which are easy to output in Go benchmarks and will be key in understanding our performance results. These R scripts can be found here and produce the following types of graphs which can be run on any developer machine. We can store the results on Storj itself or locally and plot historical percentiles over time so we can monitor our results. These scripts make it trivial to process Go output and HdrHistogram output.

An example of how to report and graph latency distribution can be [found here](https://github.com/kellabyte/go-benchmarks/tree/master/queues)

![](https://raw.githubusercontent.com/kellabyte/go-benchmarks/master/results/hashing-histogram.png)
![](https://github.com/kellabyte/go-benchmarks/raw/master/results/hashing-multi.png)
```
make hashing

goos: linux
goarch: amd64
pkg: github.com/kellabyte/go-benchmarks/hashing
BenchmarkBlake2B/1-40       	 3000000	       432 ns/op	   2.31 MB/s	       0 B/op	       0 allocs/op
BenchmarkBlake2B/2-40       	 3000000	       404 ns/op	   4.95 MB/s	       0 B/op	       0 allocs/op
BenchmarkBlake2B/4-40       	 3000000	       402 ns/op	   9.94 MB/s	       0 B/op	       0 allocs/op
BenchmarkBlake2B/8-40       	 3000000	       397 ns/op	  20.13 MB/s	       0 B/op	       0 allocs/op
BenchmarkBlake2B/32-40      	 3000000	       384 ns/op	  83.33 MB/s	       0 B/op	       0 allocs/op
BenchmarkBlake2B/64-40      	 5000000	       337 ns/op	 189.87 MB/s	       0 B/op	       0 allocs/op
BenchmarkBlake2B/128-40     	 5000000	       303 ns/op	 422.35 MB/s	       0 B/op	       0 allocs/op
BenchmarkBlake2B/256-40     	 3000000	       508 ns/op	 503.54 MB/s	       0 B/op	       0 allocs/op
BenchmarkBlake2B/512-40     	 2000000	       989 ns/op	 517.55 MB/s	       0 B/op	       0 allocs/op
BenchmarkBlake2B/1024-40    	 1000000	      1660 ns/op	 616.83 MB/s	       0 B/op	       0 allocs/op
BenchmarkBlake2B/4096-40    	  200000	      6195 ns/op	 661.11 MB/s	       0 B/op	       0 allocs/op
BenchmarkBlake2B/8192-40    	  100000	     12368 ns/op	 662.31 MB/s	       0 B/op	       0 allocs/op
PASS
```
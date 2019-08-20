# Performance Testing

## Abstract

This document proposes an initial environment to perform and measure automated benchmark results we can observe trends over time. 

## Background

We want to ensure the performance for uploads and downloads remains competitive and ideally world class.
Our code base is changing rapidly we don’t have any visibility when performance significantly changes.

Currently we have `cmd/s3-benchmark` which runs some of the benchmarks, however it is not integrated into CI.

Goals:

* Keep it simple at first but give us something we can keep extending in the future. 
* Measure both uploads, downloads and deletes.
* Measure different workloads for both upload and download. 100 byte, 1MB, 10MB, 100MB and 1GB files. 
* Measure each test 10 times and report the distribution graph of 50%, 75%, 90%, 99% percentiles.
* Author the tests using Go’s built-in benchmarking tools such that developers can run them against storj-sim.

## Design

Our current build environment supports deploying a satellite with 100 storage nodes.
We can reuse this environment for performance tests.

The benchmarking may take siginificant amount of time, such that it would be too slow to run for each release.
We may need to run the benchmarks on schedule (e.g. every hour).

### Authoring

Writing a benchmark that we want to measure will be done in a Go standard benchmark function which gives us the following output.

```
Foo-40   3000000	   432 ns/op    2.31 MB/s     0 B/op	   0 allocs/op
Bar-40   3000000	   404 ns/op    4.95 MB/s     0 B/op	   0 allocs/op
Baz-40   3000000	   402 ns/op    9.94 MB/s     0 B/op	   0 allocs/op
```

For integration tests it’s possible to re-use the MB/s ability of the go benchmarks to report operations/second for easy graphing purposes when needed.

### Reporting

Ideally we would like visualizations for:
* split by the file-size,
* density distribution of the upload times.

Jenkins supports plotting [benchmarking results](https://wiki.jenkins.io/display/JENKINS/Benchmark+Plugin). [Guide to using the plugin](https://github.com/jenkinsci/benchmark-plugin/blob/master/doc/HOW_TO_USE_THE_PLUGIN.md).

There are alternate ways of visualizing:

* such as [kellabyte/go-benchmarks](https://github.com/kellabyte/go-benchmarks), which multiple R scripts for visualizaing benchmarks. An example for latency distribution can be [found here](https://github.com/kellabyte/go-benchmarks/tree/master/queues).
* [loov/hrtime](https://github.com/loov/hrtime), which uses Go code for plotting. This has less features than R, however would be easier to integrate with our current code-base.

For custom Jenkins output we can use [HTML Publisher](https://wiki.jenkins.io/display/JENKINS/HTML+Publisher+Plugin).

## Implementation

* Implement benchmarks
* Integrate benchmarks into Jenkins,
* Add visualization into Jenkins.

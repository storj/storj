segment-verify is a tool for verifying the segments.

## High Level Overview

segment-verify verifies segment status on storage nodes in a few stages:

1. First it loads the metabase for a batch of `--service.batch-size=10000` segments.
2. They are then distributed into queues using every storage nodes. It will preferentially choose nodes specified in `--service.priority-nodes-path` file, one storagenode id per line.
3. Then it will query each storage node a single byte for each segment. `--service.concurrency=1000` concurrent connections at a time are made.
4. Every segment will be checked `--service.check=3` times. However, any failed attempt (e.g. node is offline) is only retried once.
5. When there are failures in verification process itself, then those segments are written into `--service.retry-path=segments-retry.csv` path.
6. When the segment isn't found at least on one of the nodes, then it's written into `--service.not-found-path=segments-not-found.csv` file.

There are few parameters for controlling the verification itself:

``` sh
# This allows to throttle requests, to avoid overloading the storage nodes.
--verify.request-throttle minimum interval for sending out each request (default 150ms)
# When there's a failure to make a request, the process will retry after this duration.
--verify.order-retry-throttle duration     how much to wait before retrying order creation (default 50ms)
# This is the time each storage-node has to respond to the request.
--verify.per-piece-timeout duration        duration to wait per piece download (default 800ms)
# Just the regular dialing timeout.
--verify.dial-timeout duration             how long to wait for a successful dial (default 2s)
```

## Running the tool

```
segment-verify run range -low 0x00 -high 0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff --config-dir ./satellite-config-dir
```
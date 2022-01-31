# Slow Down and Retry

## Abstract

Our goals are to:

* Improve the user experience when the request limit is exceeded by allowing
  clients to gracefully recover
* Avoid duplication of the implemention work for slow down and retry logic

## Background

The satellite has a rate limit on API requests per second per project. When a
client exceeds the rate limit it is sent a `Too Many Requests` error. This kind
of error is a retriable, but requires the client to also implement some form of
backoff (e.g. exponential).

Without a retry this error becomes fatal and can cause operations to fail that
would otherwise succeed if performed at a slower pace. For example, long
listing operations that proceed too quickly can fail and have to be restarted
(e.g. observed often in the S3 gateway).

In particular large uploads are badly impacted because a single `Too Many
Requests` error would result in the entire upload having to be redone. Uploads
are also a compelling reason for putting the logic in libuplink itself because
only libuplink has the details necessary to retry at the appropriate level
(e.g. segment or node request).

## Design

Uplink internally should be able to handle retry for:
* every request to Metainfo API when error `Too Many Requests` is returned
* segment upload request (several requests combined)
* segment download request (several requests combined)

For most of Metainfo requests and `Too Many Requests` error we need simply apply retry and slow down logic without any additional constraints. We shouldn't assume any particular order of requests to Metainfo API. Each request should be handled independently and return error if will reach specific number of retries.

Special case to handle is Metainfo API `Batch` request which is currently used only during upload. This request takes a list of requests to execute on satellite side and returns responses from each of it. If one of requests will fail then on retry we should continue from first not executed request to finish whole operation. It's necessary because `Batch` request doesn't provide any kind of rollback in case of error while executing list of requests and we can be in a state where retrying from scratch won't be possible.

Another specific behavior for Metainfo `Batch` request is passing StreamID and SegmentID internally from one request response to another request that didn't set this field (server side). This functionality helps creating longer requests list to batch. Because of that while retrying `Batch` on client side we should remember that in some cases we need to pass StreamID/SegmentID from successfully executed requests into those which we will retry in batch.

Example batched requests:
```
  begin object
  make inline segment
  commit object
```
Request `begin object` creates StreamID needed by rest of requests. If batch will fail on `make inline segment` we need to pass StreamID manually to retried `make inline segment` and `commit object`.

In case of `Too Many Requests` during upload operation we should be able to retry batched requests as soon as we resolve point 1 from Open Issues. Other case for upload is when we fail to upload single segment to storage nodes then we should retry uploading whole segment from scratch. In future we can improve this logic to re-upload failed pieces to different storage nodes. We shouldn't retry segment upload on every error but only in case of few specific like insufficient number of successful puts, connection errors, etc.

In case of `Too Many Requests` during download operation we should be able to retry single Metainfo API requests. If downloading segment from storage nodes will fail we should retry downloading this single segment. Like with uploads we shouldn't retry segment download on every error.

As an addition to retry logic we need to implement at least basic backoff mechanism that will increase the waiting time (between retries) up to a certain threshold. This will prevent from overwhelming satellite with retry requests. For our needs exponential backoff looks to be reasonable solution. 

As an alternative for implementing exponential backoff on client side we can also consider to return with `Too Many Request` error information when client should try to repeat request. The HTTP protocol defines that in such situation server may include a `Retry-After` header ([429](https://httpstatuses.com/429)). We can use similar approach and return delay value more attached to current satellite load. One of major challenges with this approach is how correctly return `Retry-After` header for multiple concurrent connections for the same API key.

To avoid satellite side larger development for providing accurate `Retry-After` values we may start with client side exponential backoff implementation and later migrate into retry interval defined by satellite.

Additional rules:
* Every retried request should be processed by backoff logic.
* Retry and slow mechanism should be transient from libuplink consumer perspective except several necessary configurable options (see Implementation) and logging (see next point).
* Every retry should be logged (severity WARNING or lower) to give client details why retry occurs and how long it will take to repeat request. Logging is mandatory on client side. In case of satellite side retry logic logging all retries can generate substantial amount of entries so it needs to be well thought out.
* Applying retry and slow down logic shouldn't degrade libuplink overall performance and increase number of round trips to satellite.

## Implementation

Most of client side logic should be placed around `private/metainfo/client.go`. This is main and only client for making calls to Metainfo API. Ideally retry and slow down logic for this API should transparent for client consumers and no implementation details should be leaking.

Implementation for upload process is placed in `private/storage/segments/store.go` and there we should adjust code base to support retrying single segment upload.

Implementation can be divided into several steps:
1. Implement retry and slow down for Metainfo API requests, except `Batch` request
2. Implement retry and slow down for Metainfo API `Batch` requests
3. Implement retry and slow down for failed segment upload
4. Implement retry and slow down for failed segment download

Client side options:
* initial interval
* number of retries
* multipliter
* jitter (`sleep(interval + random(0, jitter))`)
* max interval

## Open issues

1. Currently, endpoints handled by DRPC in case of error are not returning value, only error. With Metainfo `Batch` call we would need to have partial results which requests were executed successfully. [SM-553](https://storjlabs.atlassian.net/browse/SM-553)
2. Can we reuse order limits from failed upload? Failed upload means when sending data to storage nodes was unsuccessful.
3. How do we handle concurrent users of the same APIKey?
4. What's the best place to implement segment download retry?
5. Where we should put `RetryAfter` field in Metainfo protobuf? Should we create `ResponseHeader` similar to existing `RequestHeader` and attach it to all responses?
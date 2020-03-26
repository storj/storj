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
listing operations that proceed to quickly can fail and have to be restarted
(e.g. observed often in the S3 gateway).

In particular large uploads are badly impacted because a single `Too Many
Requests` error would result in the entire upload having to be redone. Uploads
are also a compelling reason for putting the logic in libuplink itself because
only libuplink has the details necessary to retry at the appropriate level
(e.g. segment or node request).

## Design

Libuplink internally should be able to handle retry for:
* every request to Metainfo API when error `Too Many Requests` is returned
* segment upload request (several requests combined)
* segment download request (several requests combined)

For most of Metainfo requests and `Too Many Requests` error we need simply apply retry and slow down logic without any additional constraints.

Special case to handle is Metainfo API `Batch` request which is currently used only during upload. This request takes a list of requests to execute on satellite side and returns responses from each of it. If one of requests will fail then on retry we should continue from first not executed request to finish whole operation. It's necessary because `Batch` request doesn't provide any kind of rollback in case of error while executing list of requests and we can be in a state where retrying from scratch won't be possible.

Upload operation combines several requests:
```
  begin object upload
    (for remote segment)
      begin segment upload 
        send data to storage node
      commit segment upload
    (or for inline segment)
      make inline segment
    ... repeat for multiple segments
  commit object upload
```

Requests are batched:
1. begin object upload + (first begin segment upload or make inline segment)
2. commit segment upload + (begin segment upload or make inline segment) for multi-segment object
3. (last commit segment upload or last make inline segment) + commit object upload
4. special case for multi-segment object with last segment inline: commit segment upload + last make inline segment + commit object upload

In case of `Too Many Requests` during upload operation we should be able to retry batched requests as soon as we resolve point 1 from Open Issues. Other case for upload is when we fail to upload single segment to storage nodes then we should retry uploading whole segment from scratch. In future we can improve this logic to re-upload failed pieces to different storage nodes.

In case of `Too Many Requests` during download operation we should be able to retry single Metainfo API requests. If downloading segment from storage nodes will fail we should retry downloading this single segment.

As an addition to retry logic we need to implement at least basic backoff mechanism that will increase the waiting time (between retries) up to a certain threshold. This will prevent from overwhelming satellite with retry requests. For our needs exponential backoff looks to be reasonable solution. 

Additional rules:
* Every retried request should be processed by backoff logic.
* Retry and slow mechanism should be transient from libuplink consumer perspective except several necessary configurable options (see Implementation).
* Applying retry and slow down logic shouldn't degrade libuplink overall performance and increase number of round trips to satellite.

## Rationale

[A discussion of alternate approaches and the trade offs, advantages, and disadvantages of the specified approach.]

## Implementation

Options:
* initial interval
* max elapsed time or number of retries ??
* multipliter
* randomization factor
* max interval

## Open issues

1. Currently, endpoints handled by DRPC in case of error are not returning value, only error. With Metainfo `Batch` call we would need to have partial results which requests were executed successfully. 
2. Maybe we should treat Batch request as a one request from rate limit perspective.
3. Can we reuse order limits from failed upload? Failed upload means when sending data to storage nodes was unsuccessful. 
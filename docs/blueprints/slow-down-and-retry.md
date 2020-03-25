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

[A precise statement of the design and its constituent subparts.]

## Rationale

[A discussion of alternate approaches and the trade offs, advantages, and disadvantages of the specified approach.]

## Implementation

[A description of the steps in the implementation.]

## Wrapup

[Who will archive the blueprint when completed? What documentation needs to be updated to preserve the relevant information from the blueprint?]

## Open issues

[A discussion of issues relating to this proposal for which the author does not
know the solution. This section may be omitted if there are none.]

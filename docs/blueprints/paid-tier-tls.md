# Paid Tier TLS

## Abstract

The Edge team is building TLS support for statically-hosted sites that are served via Linksharing. This document outlines the way that the edge services will determine whether or not to generate/use a TLS certificate in serving requests, based on whether the API key used in the request is "free" or "paid".

## Background

TLS support is a feature we want to enable for paid-tier customers who are hosting static sites over Linksharing. The intention is to provide an additional incentive for users to add a payment method to their account.

A user currently becomes a paid-tier customer automatically by adding a credit card to their account in the Satellite GUI. Alternatively, a user who pays with STORJ Token is able to request to be upgraded to the paid tier by [submitting a request to support](https://supportdcs.storj.io/hc/en-us/requests/new). Paid-tier users are able to use the same amount of storage and bandwidth as free-tier users at no cost, but are able to create more than one project, and have substantially increased storage and bandwidth limits for each of their projects.

Linksharing currently does not have an easy way to retrieve information about whether an API key is free/paid tier. Its only communication with the satellite is done via metainfo/libuplink, in order to retrieve metadata about files being served over http/https.

## Design

At a high level, the design consists of:
* A new RPC endpoint on the satellite to handle edge -> satellite communications. This endpoint will take API key as an argument, and respond with a boolean indicating whether the API key is "paid tier".
* An outline of the schedule/trigger that the edge services will use to invoke the new endpoint, and determine whether to generate or renew a TLS certificate for a statically hosted site.

## Rationale

In theory, it would be possible to integrate this functionality into the existing "project info" endpoint on the metainfo rpc server. However, we discarded this approach for a couple reasons:

1. "paid tier" info is stored in the `users` table. Existing metainfo requests rely almost solely on `projects` table and cached information from that table, and the `projects` table does not contain information about free/paid tier. To integrate this functionality into the "project info" endpoint would require an additional request to the `users` table, or a `JOIN` to combine `users` and `projects` info, which would negatively (and unnecessarily) impact the performance of most requests to this endpoint.
2. TLS certificates will expire in 90 days by default, and we intend to check for renewal every 45 days. An additional request every 45 days should not have a notable impact on performance for static sites hosted with TLS.

## Implementation

### 1. Add protobuf for Edge -> Satellite communication

Add a protobuf to be shared by the edge services and satellite, so that the satellite is able to host an rpc server, and the edge services are able to invoke the new endpoint.

```
service UserInfo {
    rpc Get(GetUserInfoRequest) returns (GetUserInfoResponse);
}

message GetUserInfoRequest {
    metainfo.RequestHeader header = 1;
}

message GetUserInfoResponse {
    bool paid_tier = 1;
}
```

Implemented in https://github.com/storj/common/commit/c4f89fddfc5a41a0e8c9de8563a1032205c07756 and https://github.com/storj/common/commit/35ed6c701cb63afdb20a8c4222194e7d015314b6

### 2a. Create new satellite package to implement the protobuf

* Create a subpackage `satellite/console/userinfo`
* New type `UserInfoEndpoint` with constructor `NewUserInfoEndpoint`, taking `peer.DB.Console().Users()`, `peer.DB.Console().APIKeys()`, and `peer.DB.Console.Projects()` as dependencies.
* The new DRPC server/endpoint from the protbuf should be stubbed (non functional)
* Config added, similar to AllowedSatellites in `pkg/auth/peer.go` in `gateway-mt` repo - but rather than satellites, this will be a list of IDs/addresses of all production edge servers that should be allowed to use this endpoint

Tracked in
https://github.com/storj/storj/issues/5358

### 2b. Implement protobuf client-side on Linksharing

* Create DRPC client on Linksharing to invoke `UserInfo.Get` on the satellite
* If an incoming TLS request occurs for a static-hosted website on Linksharing,
    * If there is a cert for this api key already, and it is fewer than 45 days old, do nothing extra
    * If there is no cert and the api key is cached as "free tier", do nothing extra
    * If there is no cert for this api key (and api key is not in cache), _or_ if the cert is more than 45 days old, call `UserInfo.Get`.
        * If paid tier, generate a new cert that expires in 90 days.
        * If free tier, indicate that this api key is "free tier" in the cache

Tracked in
https://github.com/storj/gateway-mt/issues/273

#### Notes on Linksharing Cache:

Link Sharing Service has a cache for `TXT` records that will also be used for this purpose. It fetches the Access Key ID from the `TXT` record from DNS and resolves it through Auth Service; we will add resolution to also add tier (when applicable) to it.

It has already been implemented here: https://review.dev.storj.io/c/storj/gateway-mt/+/9109. However, some future-proofing still needs to be done as part of that or a follow-up change.

### 3. Implement DRPC endpoint to get user info on satellite

* Add config to enable the new RPC server on the Satellite API pod
* If enabled, create and start this server from `satellite/api.go`
* Implement the "user info" endpoint on the RPC server:
    1. Verify peer using the endpoint is in allowed config (added in step 2a
    2. Get api key info from `db.apikeys`
    3. Get project info from `db.projects` using api key info
    4. Get user info from `db.users`, using project info
    5. Then pass `true`/`false` based on paid tier status of user back to client
* Write a test for the new endpoint

Tracked in https://github.com/storj/storj/issues/5363

## Wrapup

The Integrations Team is responsible for archiving this document upon completion.

## Open issues


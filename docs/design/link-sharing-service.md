# Link Sharing Service

## Abstract

This design doc outlines a link sharing service that can be used by clients
to share content with others via a simple URL.

## Design

### Transport

The link sharing service supports HTTPS or HTTP. HTTP is intended for
development convenience only or when offloading TLS termination to a proxy
running on the *same host*. The link sharing service should *NOT* be deployed
behind an off-the-box TLS terminator unless the link between the terminator and
the link sharing service is adequately protected. See
[Security Considerations](#security-considerations) for details.

### Requests

The file hosting service handles requests of the following form:

`GET /<share-blob>/path/within/bucket`

The `share-blob` is url-safe base64 encoding of a `Share` protobuf, which
is defined as follows:

```
message EncryptionAccess {
	bytes key = 1;

	bytes encrypted_path_prefix = 2;
}

message Share {
	string satellite_url = 1;

	string api_key = 2;

	string bucket = 3;

	EncryptionAccess encryption_access = 4;
}
```

Between the share and the path, the link sharing service has all the
information it needs to stream data via uplink.

### Caching

The link sharing service does not attempt to cache data, metadata, etc. 

Presumably, with a small change to the uplink library, pieces downloaded from
storage node operators could presumably be cached locally on disk or via some
sort of shared cache (in the presence of horizontally scaled link sharing
services). At this time such caching seems premature and potentially 
harmful in that:

- It complicates the code.
- It complicates deployment.
- It decreases payout potential for storage node operators, reducing incentive.
- The whitepaper already describes a mechanism to deal with hot objects.

## Implementation

The following steps are taken to handle requests:

1. The share blob and bucket path are parsed from the request URL
2. The share is decoded
3. The project is opened using the satellite URL and API key provided by the share.
4. The bucket is opened using the bucket name and encryption access provided by the share.
5. An object reader is created using the unencrypted bucket path provided in the request URL.
6. The content is served back to the client.
7. The reader, bucket, and project are all closed.

## Security Considerations

By providing the share information to the link sharing service, clients are
implicitly giving the link sharing service access to the unencrypted data
reachable via the share. As such, care should be taken to *NEVER* store or
otherwise cache the information read from uplink. Additionally, request URLs
must *NEVER* be logged as they consist wholly of sensitive information.

In addition, Zero Trust Networking best practices call for each network link to
be secured as network boundaries are not good security boundaries. As such, the
link sharing service must *NEVER* be deployed behind a TLS terminator (e.g.
load balancer) where the link between the terminator and the link sharing
service is unprotected.

## Future Work

1. LetsEncrypt support for obtaining TLS certificates for the HTTPS server.
2. Provide CLI support to `uplink` for generating a share URL.

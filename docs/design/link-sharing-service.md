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

#### Get Link

`HEAD /<scope-blob>/<bucket>/<bucket path>`

This request is sent by clients who want to share a link to some bucket
data. It responds with a 302 with a `Location` header containing the URL
that clients can share with others.

For now, the `Location` header is set to the [Download Data](#download-data) URL
but provides a future proof mechanism for the link sharing service to provide
extra security features that don't expose the scope blob in the URL, for example.

The `scope-blob` is base58 encoding of a `Scope` protobuf, which
is defined as follows:

```
import "encryption_access.proto";

message Scope {
    string satellite_addr = 1;

    bytes api_key = 2;

    encryption_access.EncryptionAccess encryption_access = 3;
}
```

The `bucket` is the name of the bucket and the `bucket path` is the path
within the bucket to the file being shared.

#### Download Data

`GET /<scope-blob>/<bucket>/<bucket path>`

This request is sent by clients to download shared data.

See [Get Link](#get-link) for details about the path components.

Between the scope, the bucket, and the path, the link sharing service has all
the information it needs to stream data via uplink.

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
- Section 6.1 of the [whitepaper](https://storj.io/storjv3.pdf) already describes a mechanism to deal with hot objects.

## Sharing Flow

To share a link, clients:

1. Issue a [Get Link](#get-link) request, providing the scope blob, the bucket, and the bucket path.
2. Parse the `Location` header of the 302 response and share that with other parties.

To retrieve shared data, other parties:

1. Make a GET request to the location provided by the client. Currently, that
   location will be to the [Download Data](#download-data) URL.

## Implementation

The following steps are taken to download data:

1. The scope blob, bucket, and bucket path are parsed from the request URL
2. The scope is decoded
3. The project is opened using the satellite URL and API key provided by the scope.
4. The bucket is opened using the bucket name from the url and the encryption ctx provided by the scope.
5. An object is opened using the bucket path provided in the request URL.
6. The content is served back to the client.
7. The object, bucket, and project are all closed.

## Security Considerations

By providing the scope information to the link sharing service, clients are
implicitly giving the link sharing service access to the unencrypted data
reachable via the scope. As such, care should be taken to *NEVER* store or
otherwise cache the scope or information read from uplink. Additionally,
request URLs must *NEVER* be logged as they consist wholly of sensitive
information.

In addition, Zero Trust Networking best practices call for each network link to
be secured as network boundaries are not good security boundaries. As such, the
link sharing service must *NEVER* be deployed behind a TLS terminator (e.g.
load balancer) where the link between the terminator and the link sharing
service is unprotected.

## Future Work

1. LetsEncrypt support for obtaining TLS certificates for the HTTPS server.
2. Provide CLI support to `uplink` for generating a share URL.
3. Obfuscate the share URL so that scope data isn't leaked.

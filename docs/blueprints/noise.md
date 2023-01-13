# Noise over TCP (uplink to storage node)

## Abstract

This design doc discusses how we can achieve a large connection set up
performance improvement by exchanging our use of TLS for Noise_IK.

This design doc is scoped to communication between the Uplink and storage nodes
only.

## Background/context

### The problem

When compared to traditional datacenter storage platforms, Storj is
significantly more exposed to issues arising from distance. In particular,
whenever two peers need to exchange data in a round trip, the speed of light is
a serious concern. In a datacenter, a round trip between two nodes may take
500 us, or .5 ms. In the Storj context, a round trip to a node across the ocean
may take 150 ms, *300 times* slower. There is no way to fix this, this is a
fundamental physical law.

Because datacenter round trips are so fast, many protocols have not been
designed with an extreme allergy to round trips, but we must be. Every time we
wait for a packet round trip, we are adding 150ms, which for certain operations,
is our entire performance budget.

To do a small download, say 5KB for example, right now the Uplink will:
 * first establish a TCP connection with the Satellite (one packet round trip,
   TCP SYN, then TCP ACK),
 * then establish a TLS session over that TCP stream (another packet round trip,
   TLS Client Hello, then TLS Server Hello),
 * then send the request and receive a satellite response (a third round trip).
 * Now, the Uplink is finally able to start making requests to nodes! It has to
   do this all again.
 * first, establish a TCP connection with each node (packet round trip)
 * then, establish a TLS session over TCP (packet round trip)
 * then send the RPC request over DRPC (packet round trip)
 * THEN send the order (it's not part of the same request currently!!)
   (strictly speaking, this isn't inherently another round trip, though this
    behavior does significantly complicate using Noise, which requires a
    response to handshake packet 1 before sending packet 2).
 * finally, the node will start returning data.

If the Uplink is near the Satellite, the Satellite operations won't be the worst
part (though, considering most Uplinks don't operate in the same datacenter as
the Satellite, and the Satellite itself has inter-dc penalities due to Cockroach
coordination, it's not cheap). But most node operations do have a large amount
of traffic that goes over an ocean, and many node operations are not able to
be effectively connection pooled (though we've tried). Because the overall
request goes as fast as the slowest of 29 downloads, the likelihood that at
least one of the required nodes has a high round trip cost is high, and so this
is what is killing us.

### QUIC

Our first attempt to fix this was to roll out QUIC. QUIC is a TCP-like protocol
built on top of UDP packets (TCP is a protocol built on top of IP packets).
QUIC's solution is to combine the TLS and the TCP handshake into one.

Marton has proven that our QUIC project did indeed pay off, most of the time.
QUIC makes our overall performance between peers significantly faster due to
the elimination of just one handshake.

Unfortunately, having QUIC based on UDP means that slightly more requests fail
than with TCP (perhaps bad node operator setup, middleware dropping packets,
unclear). This means that our long tail cancelation has to wait for more nodes,
which makes us more susceptible to slowness. So overall, QUIC has worse long
tail variability, and is slower at higher percentiles.

It is currently not enabled by default.
We still intend to use QUIC or UDP more generally, but perhaps we can do
something else sooner.

### Do we need the TLS handshake?

QUIC eliminates one handshake by combining the TCP and TLS handshakes, but the
TLS handshake at all is a bummer. The reason the TLS handshake exists is for
both peers to exchange cryptographic information and confirm that they are
talking to the right peer over the right protocol, doing a protocol negotiation.

But we don't need that, certainly for nodes!

Storage nodes are already checking in with Satellites, and this provides an
opportunity for storage nodes to provide all of their cryptographic
configuration settings (public key, etc) in advance. When the Satellite tells
the Uplink which nodes to talk to, it can give the Uplink everything it needs
to skip the cryptographic handshake.

So, we should try and eliminate the cryptographic handshake altogether. We don't
need it, and we can avoid it, at least between Uplinks and nodes, in all cases.

### Do we need any handshakes?

If we don't need the TLS handshake, do we really even need the TCP handshake?
Also no. There's no reason the very first SYN packet for connection setup can't
include the request data, so that the very second packet back (what would have
been the ACK packet) can include the first bytes of the response. This would
take our current situation of 3 round trips between the Uplink and each node to
1, which saves 100-150ms off for each round trip eliminated.

Including data in the SYN packet is precisely what TCP_FASTOPEN does, which we
could enable, if we don't end up using something UDP-based.

For the purposes of this design doc though, suffice it to say that if this is
our ideal, we need to eliminate the TLS handshake entirely in almost all cases.

### What about the Satellite?

This design doc is focusing on the Uplink to storage node communication as a
way to get a foot in the door. Solving Uplink to Satellite communication is
more challenging, as we don't have a clean way of getting the Uplink the
Satellite's current Noise public key in advance. That is of course interesting
but saved for a future design doc. Satellite to storage node communications are
not in the hotpath and are thus not as high priority.

Luckily, the Uplink to Satellite communication flow benefits the most from
connection pooling.

## Design

### The Noise Framework

The Noise protocol framework is a relatively new entrant in the cryptographic
communication space. Using cryptographic design primitives like the ratchet
system Signal uses, the Noise protocol framework provides a tight,
straightforward set of cryptographic building blocks for building your own TLS.
In fact, the Noise protocol is what Wireguard uses.

A great intro to the Noise framework is this presentation by its creator:
https://www.youtube.com/watch?v=3gipxdJ22iM. The design doc is also fantastic:
https://noiseprotocol.org/noise.html

The Noise framework prioritizes simplicity and security above all else.

One of the ways the Noise framework radically reduces complexity is by
eliminating all of the negotiation features that TLS has. In TLS, the client and
the server negotiate on which cryptographic primitives can and should be used
for the session, and the client and server can choose based on what's available
between peers.

Noise does away with this. When you implement Noise, you pin down your
algorithms. Since Noise is a framework, here are some example Noise protocols
within the framework:

 * Noise_XX_25519_AESGCM_SHA256
 * Noise_N_25519_ChaChaPoly_BLAKE2s
 * Noise_IK_448_ChaChaPoly_BLAKE2b

 You choose one of these at connection dial time, and then there are no places
 in the code that allow a session to negotiate or switch. Both peers must agree
 on the protocol in advance.

 The Noise framework also lets you choose between a set list of potential
 "handshake  patterns" which describe what is exchanged in packets, when, and
 what security  properties you get. There are many possible handshake patterns,
 but for full duplex protocols, the Noise authors recommend XX or IK only. XX is
 much like TLS in that there is a handshake to exchange keys first. IK is a
 0-RTT handshake and requires the keys exchanged in advance, which we can do.
 Wireguard uses Noise_IK. Noise_IK is the handshake pattern that solves our
 problems here.

 The one (and only?) downside to Noise_IK is that it opens us up to replay
 attacks. See the Open issues section.

 We want to use Noise_IK. We probably want to benchmark and pick between:

  * Noise_IK_25519_ChaChaPoly_BLAKE2b
  * Noise_IK_25519_AESGCM_BLAKE2b

I think we're convinced BLAKE2b is faster and better for our cases than BLAKE2s,
SHA256, and SHA512, but I'm not convinced (given the existence of accelerated
encryption hardware) that ChaChaPoly is better than AESGCM, which we use
everywhere else.

### TCP vs UDP

We already have a project (QUIC) that seeks to eliminate the TCP handshake, and
TCP in general, and prepare the network for using other UDP-based protocols. It
has been a mixed bag. We should keep working on it! We have struggled to enable
QUIC by default due to these UDP-related issues.

This project is trying to take a parallel approach as a next step - what
happens if we keep TCP? Can we sidestep our UDP issues but still eliminate a
handshake in a robust way?

So, this project is going to be based on TCP. A later project may be to swap
TCP for another UDP based protocol (unencrypted QUIC, UDT, something else) or
to try and improve TCP using a technique like TCP_FASTOPEN, but
we can do that after getting all the cryptography correct for this one.

### Data flow

The broad picture is that when storage nodes check in, they will submit their
Noise public key and Noise configuration to the Satellite. When the Uplink asks
the Satellite who to talk to, the Satellite can return the Noise information
to the Uplink, and the Uplink can establish 0-RTT Noise_IK connections.

In tests for small files, this led to a consistent 1.5x overall speedup, which
is huge. With TCP_FASTOPEN in addition, the savings went to over 2.8x.

Noise_IK requests are at risk of replay attacks, so we don't want to enable
them by default everywhere. We need to audit each request for idempotency before
enabling it, but at least initially, Upload and Download requests from Uplinks
to Nodes would be exceptionally high value.

Uploads require that the node is able to validate cryptographically that the
peer it is talking to is the node id in question. Since we get that with TLS
but we won't get that with Noise DH25519 keys, the Node will need to send a
signed attestation by its Node key that the Noise key is indeed its public key
at the end of the upload. This can be precomputed and thus fast.

## Rationale

Seems good!

## Implementation

### Changes to the RPC server code

As a function of our migration from gRPC to DRPC, we already have a server-side
demultiplexer system built in - drpcmigrate.ListenMux. Even more luckily,
(and, actually, a situation planned with foresight) this multiplexing happens
outside of the TLS stream, so we can use this same demultiplexer for
differentiating between DRPC over TLS and DRPC over Noise.

The current prefix for DRPC over TLS is 8 bytes - `DRPC!!!1`. We can do a new
prefix, `DRPC!N!1`, to indicate Noise.

```
publicNoiseDRPCListener = noiseconn.NewListener(
    publicMux.Route("DRPC!N!1"), p.noiseConf)
go p.public.drpc.Serve(ctx, publicNoiseDRPCListener)
```

The server will need to generate a static DH key and persist it somewhere,
though it is okay if the DH key gets regenerated from time to time (process
start is probably fine TBH).

The server will also need to choose the encryption algorithm, hashing algorithm,
and DH curve to use.

It's worth mentioning that Noise has some guidance about cryptographic channel
binding. If you want a node to attest that it is indeed the peer on the other
end of a Noise channel, the best way for the node to attest that it is that
specific Noise session is for the peer to sign the handshake hash more than
anything else, exposed in this commit:
https://github.com/jtolio/noiseconn/commit/d7ec1a08b0b81c40754d83980ae63d1dfcc7c58b
See https://noiseprotocol.org/noise.html#channel-binding for more.

Potentially useful commits:
 * https://review.dev.storj.io/c/storj/drpc/+/9225
 * https://review.dev.storj.io/c/storj/storj/+/9224
 * https://review.dev.storj.io/c/storj/storj/+/9187

### Changes to Node Address / NodeURL structures

Oh man, pb.Node, pb.NodeAddress, and pb.NodeTransport have rotted significantly
from their original intentions. pb.NodeTransport still having GRPC flags in
there gives some sense that we got this all wrong.

In fact, the original intention of pb.Node has been almost entirely superceded
by NodeURL, which basically does the exact same thing as pb.Node.

The original intention of pb.Node was to specify a Node, along with the
information you'd need to securely dial it. That is now a NodeURL, which is
of the form

```
base58nodeid@host:port
```

This NodeURL, by virtue of having the node id, allows the client to know whether
they are securely talking to the right peer (the node id is the hash of the
validated certificate authority that signed the TLS leaf cert).

Unfortunately, the NodeURL as it stands is not enough to talk over Noise_IK,
since the RSA keys referenced by the NodeID cannot be used in Noise.

To be able to talk over Noise, the following things are needed:
 * The peer public key (32 bytes) (this is only needed for Noise_IK, so this
   could be optional for Noise_XX).
 * The peer's cipher suite selection (hashing, symmetric encryption, and
   Diffie-Helmann)
 * The peer's handshake pattern (we should always use IK for Nodes, but perhaps
   we'll want to support XX for Satellites).

To prevent a malicious Satellite from replacing Noise public keys with something
else, we'll additionally need

 * A signed signature, from the node's certificate chain and public key, signing
   that the Noise key is correct. Unfortunately, for validation, this requires
   also carrying around the node's certificate chain, so this is likely too
   large to include in a Node Address and will need to be something we provide
   on demand. It's unclear how much of a threat a malicious Satellite could even
   be, considering how much trust the Uplink puts in the Satellite.

Independent of anything else, pb.Node/pb.NodeAddress/pb.NodeTransport should
be refactored so that it is a 1-1 match with NodeURL. A NodeURL should be able
to be represented efficiently (not base58) as a pb.Node protobuf, and
human-readable as a NodeURL. There should be a lossless conversion routine that
converts a pb.Node to a NodeURL and back again.

At some future point, I'd also suggest that we should rename NodeURL to
something else since we're abusing URI syntax at best, but for now let's
stick with the NodeURL name to reduce churn.

This all implies some cleaned up types. Here's the new protobuf version of a
NodeURL:

```
message NodeURL {
  // the node id, not encoded
  bytes id = 1;

  // the address for communication with the node. This address must support
  // IPv4 TCP connections (and should support IPv4 UDP connections).
  // Address here implies a host/port pair joined via net.JoinHostPort style
  // logic.
  string address = 2;

  // noise settings. If provided, the node may support noise handshakes instead
  // of TLS over TCP or UDP.
  enum NoiseProtocol {
    NOISE_UNSET = 0;
    NOISE_IK_25519_CHACHAPOLY_BLAKE2B = 1;
    NOISE_IK_25519_AESGCM_BLAKE2B = 2;
  }
  NoiseProtocol noise_proto = 3; // this is explicitly not a set.
  bytes noise_pk = 4;
}

// This type is a note signed by a node that this public key is what they are
// using. NoiseSessionAttestation should be used instead where possible.
message NoiseKeyAttestation {
    bytes node_id = 1;
    bytes node_certchain = 2;
    bytes noise_public_key = 3;
    uint64 timestamp = 4;
    bytes signature_of_public_key_and_timestamp = 5;
}

// This type is a note signed by a node that this active Noise session really
// has them on the other end.
message NoiseSessionAttestation {
    bytes node_id = 1;
    bytes node_certchain = 2;
    bytes noise_handshake_hash = 3;
    bytes signature_of_handshake_hash = 4;
}
```

The intention of NodeTransport was to have a list of transports that the node
understood, so that clients could use newer transports on nodes that supported
them, but in practice this field has just fallen into complete disuse and we've
managed those issues with requiring recent versions on all nodes instead. That
said, it may still make sense to have a list of supported protocols in the
Node structure.

A `pb.NodeURL` can be serialized into a string NodeURL as follows:

```
base58nodeid@address?noise_proto=1&noise_pk=base58_noise_public_key
```

This should be backwards compatible with existing serialized NodeURLs. We should
evaluate if this format is parsable by existing NodeURL parsing code (assuming
it throws away query parameters).

AddressedOrderLimits should return filled in *pb.NodeURL.

### Changes to the RPC client code

Dialing to a node should take this new NodeURL structure, along with whether the
request is replay-attack safe.

```
DialNode(ctx context.Context, node *pb.NodeURL, replay_safe bool) (Conn, error)
Validate(node *pb.NodeURL, attestation *pb.NoiseKeyAttestation) (error)
ValidateSession(node *pb.NodeURL, attestation *pb.NoiseSessionAttestation) (error)
```

(If `replay_safe` is false, Noise_IK should not be used).

As an aside:

The rpc.Connector situation is a mess. For example:
 * TCPConnector's DialContextUnencrypted adds the DRPC specific header, which
   will make changing the header based on different Encryption strategies (TLS
   vs Noise) challenging.
 * HybridConnector does too much (premature generalization).
 * If someone provides their own DialContext, the Connector interface doesn't
   allow for the net.Dialer.Control style of adding something like TCP_FASTOPEN.
   Config.DialContext is unfortunately just one step removed from
   Dialer.Control, which means that if someone provides a Config.DialContext,
   then our library can no longer call Setsockopt on the socket before dialing.
   If we want to flexibly add TCP_FASTOPEN to most requests, then we need to be
   able to call Setsockopt before the connect() syscall happens, and that's only
   possible if you set Dialer.Control *before* DialContext is called.

We should get rid of all of the connector flexibility and have a single,
unconfigurable dialer type that dials with common/socket's BackgroundDialer.

Going forward, you should be able to ask an RPC dialer pool to:

```
DialNode(ctx context.Context, node *pb.NodeURL, replay_safe bool) (Conn, error)
```

and get back a valid Conn. The logic inside DialNode should:

1) Consider the QUIC rollout state from common/rpc/quic_rollout.go. If QUIC is
   enabled, we should use that. QUIC should be disabled.
2) If QUIC is disabled, but replay_safe is true and the pb.NodeURL has Noise
   information, the dial should happen over TCP over Noise.
3) Otherwise the dial should happen over TCP over TLS

The RPC pool needs to keep track of QUIC, TLS, and Noise connections separately.
In particular, Noise connections should be identified by the Noise public key
and Noise protocol from the pb.NodeURL Noise protocol enum.

Possibly useful commits:
 * https://review.dev.storj.io/c/storj/common/+/9219

### Changes to DRPC

DRPC should gain a feature that allows corking outgoing sends (forcing all
writes into a local buffer), and then uncorking, which tells the DRPC stream
to send the buffer with the next send. This is important because our existing
piecestore Download protocol sends two separate requests before the node can
start returning data, and we want both requests to go into the initial Noise
packet.

drpcstream.Options.ManualFlush is close, but we only want to change it
for a single packet, and we are possibly getting a conn from the connection
pool, so a per-conn ManualFlush is hard to use.

Possibly useful commits:
 * https://review.dev.storj.io/c/storj/drpc/+/9236
 * https://review.dev.storj.io/c/storj/uplink/+/9237

### Changes to the storage node

The storage node, using the new RPC server side code, should have Noise
configuration generated. It should submit this Noise information as part of its
contact checkin. It should provide a NoiseSessionAttestation.

Nodes should send NoiseSessionAttestations at the end of uploads so the Uplinks
can do node id piece validation.

Nodes should be extended to check if the initial Download request has an order
embedded and use it if so.

Possibly useful commits:
 * https://review.dev.storj.io/c/storj/uplink/+/9246
 * https://review.dev.storj.io/c/storj/storj/+/9245
 * https://review.dev.storj.io/c/storj/common/+/9197
 * https://review.dev.storj.io/c/storj/storj/+/9188

### Changes to the Satellite

On contact checkin, the Satellite should check for Noise information and
validate a NoiseSessionAttestation. If the NoiseSessionAttestation is valid, the
Satellite should persist the *pb.NodeURL information.

Note that a Node may submit a DNS hostname as opposed to a specific IP address.
Because we don't want uplinks to stress their local DNS resolution, the
Satellite should perform and cache the DNS resolution of recursive A and AAAA
lookups for any hostname here. Uplinks should not expect DNS resolution for
NodeURLs.

The upload and download selection caches should retrieve the *pb.NodeURL
information and add them to the AddressedOrderLimit structs that are sent to
Uplinks.

Possibly useful commits:
 * https://review.dev.storj.io/c/storj/common/+/9197
 * https://review.dev.storj.io/c/storj/common/+/9214
 * https://review.dev.storj.io/c/storj/storj/+/9200
 * https://review.dev.storj.io/c/storj/storj/+/9215

### Changes to the Uplink

Uplinks should be extended to use the *pb.NodeURL from the AddressedOrderLimits
and use the rpc Dialing that uses those.

Our existing piecestore Download protocol sends two separate requests before the
node can start returning data, and we want both requests to go into the initial
Noise packet. So, Uplinks should use a new DRPC feature to cork the first
Download request RPC send, so that the Download request goes out when the Uplink
sends the actual order request and both get written to the first Noise packet.

An alternative strategy would be to update the Uplink to send the first order
as part of the first request, but this is not backwards compatible with old
storage nodes.

Possibly useful commits:
 * https://review.dev.storj.io/c/storj/uplink/+/9246
 * https://review.dev.storj.io/c/storj/storj/+/9245
 * https://review.dev.storj.io/c/storj/uplink/+/9218
 * https://review.dev.storj.io/c/storj/uplink/+/9220
 * https://review.dev.storj.io/c/storj/common/+/9214
 * https://review.dev.storj.io/c/storj/storj/+/9221

## Other options

### TLS session resumption

Instead of Noise, we could still use TLS. TLS 1.3 has a feature called
session resumption. Session resumption negotiates a key after a first connection
that can be reused for zero roundtrip session setup if both peers remember each
other. The downside of this is that we wouldn't get zero roundtrip for the first
connection.

Erik asked if perhaps the Satellite could establish these keys in advance and
simply hand off the SSL connection session resumption information to the node.
This seems possible in theory. Open questions for me: would this work in general
for more than one connection? Is it safe to reestablish many connections to
a node from multiple Uplinks? Would we have to negotiation session resumption
information in advance per Uplink? There are a lot of cryptographic unknowns
here for me, but in principle this is essentially forcing TLS into the Noise_IK
shape.

Overall, I'm worried at how unusual this is, vs Noise IK, where what we're doing
is what it is designed for.

### QUIC session resumption

QUIC session resumption is exactly TLS 1.3 session resumption.

### Multiaddresses

We might want to consider migrating to https://github.com/multiformats/multiaddr
instead of NodeURLs. The challenge I see with multi-addresses is they seem
to let the multi-address specify much more about the connection than we want
to allow (whether or not TLS is used, what protocols are used, etc). The only
parts of the multi-address we want are the stuff that get us to having a
valid IP packet host, and maybe whether the peer has open TCP or UDP ports.
Multi-addresses do much more than that, and so we would be in a position where
we are restricting what we use in multi-addresses, though I suppose that's not
much different than URLs in general. I don't know what to do here other than
that I have NIH.

## Wrapup

## Related work

### github.com/jtolio/noiseconn

To get my proof of concept working, I wrote a library that is a useful
net.Conn wrapper that uses Noise. github.com/jtolio/noiseconn has good
performance and has been tested as part of the proof of concept for this
project.

### Tracing

 * https://review.dev.storj.io/c/storj/storj/+/9229

### TCP Fast Open

 * https://github.com/storj/storj/blob/main/docs/blueprints/tcp-fastopen.md

### Separated UDP address support

Cleaning up NodeURL is an opportunity to add more clarity around UDP addressing.
Here are some thoughts:

* We could have the pb.NodeURL structure maintain a separate UDP address in
  addition to the TCP address in there, or perhaps just a separate port. There's
  no strict requirement that a node use the same port for UDP and TCP, though
  of course that is what we currently require. Changes to pb.NodeURL are the
  right place to add this support, but it should likely not be part of this
  blueprint.

### IPv6 support

Cleaning up NodeURL is an opportunity to add more clarity around IPv6 support.
Here are some thoughts:

* Because customers might move from IPv4 to IPv6 networks and back, every
  storage node must be reachable by every network. At very least, this means
  every storage node must be reachable over IPv4 (IPv6 only networks should be
  able to hit IPv4 nodes over gateways). It's fine if nodes also support IPv6
  such that IPv6-supporting clients can reach IPv6 nodes over IPv6 natively,
  but if data is uploaded to IPv6, we don't want it to be only available if the
  client is on an IPv6 supporting network. So, unless a client explicitly opts
  into their data potentially only being available over IPv6, every storage node
  must support IPv4.
* For IPv6 support, we could either have the NodeURL list the IPv6 address in
  addition to the IPv4 address, much like the potential additional UDP
  information, or we could require that IPv6 node operators get a DNS entry
  that has both A and AAAA records, and then the Satellite fills in the
  appropriate address in returned *pb.NodeURLs included in AddressedLimitOrders,
  based on what the Uplink requested.

Again, probably not part of this blueprint.

## Open issues for future work

 * We need to double check that uploads and downloads are replay attack safe
   and make them so if not. Order serial numbers should protect against this.
 * We should evaluate what other commands are replay attack safe.
   Exists, RestoreTrash, Retain, and DeletePieces do not have serial checking,
   but are only made by Satellites. We may want to ensure these methods are
   not available over Noise_IK. DeletePieces is likely the only performance
   sensitive call here to consider.
 * We should have RPC clients keep a cache of Noise public key attestations.
   We won't have the Satellite public key initially, but perhaps if an RPC
   client has spoken with a Satellite before, the Satellite could have provided
   a NoiseKeyAttestation, and thus future connections could be over Noise. This
   would be especially useful for the Gateway-MT. We would need to audit which
   Satellite requests are replay attack safe.
 * This may be more of an issue for TCP_FASTOPEN, but we should double check
   that 0-RTT connection establishment doesn't open us up to amplification
   attacks (could an attacker spoof N bytes to us and get us to send >N bytes
   to a third party?)
 * How do we improve Uplink to Satellite communication? Access grants have
   Satellite node IDs, but not Noise public keys. Is it a good idea to put
   Noise public keys in Access grants? Should we use Noise_XX and deal with
   the handshake? That doesn't save us much, but perhaps our Noise ciphers are
   faster than TLS?

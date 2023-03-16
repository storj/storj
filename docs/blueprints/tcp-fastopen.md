# TCP_FASTOPEN

## Abstract

This design doc discusses how we can achieve a large connection set up
performance improvement by dialing TCP connections with and without
TCP_FASTOPEN enabled, selectively choosing the connection that established
the fastest.

## Background/context

### The problem

This design doc shares the same context as the Noise over TCP design doc.
See https://github.com/storj/storj/blob/main/docs/blueprints/noise.md for
details about the background and context.

The summary of the above is that we would see massive gains to our product if we
were able to eliminate connection setup handshakes, including the TCP connection
SYN/ACK/SYNACK handshake. Note that this design doc is not restricted to Uplink
to storage node communication only like the Noise over TCP design doc.

### TCP_FASTOPEN

Of course, TCP handshakes affect *everyone*, not just Storj. TCP_FASTOPEN is the
deployed, implemented, kernel feature (supported in Windows and Linux!) that
enables skipping TCP handshakes!

Here is a great, positive article about the design and motivation:
https://lwn.net/Articles/508865/

Unfortunately, it turns out that in practice, TCP_FASTOPEN has had a number of
adoption hurdles. Here is a great, negative article about what went wrong:
https://squeeze.isobar.com/2019/04/11/the-sad-story-of-tcp-fast-open/

Both are worth reading.

The downsides to TCP_FASTOPEN listed in the second article amount to the
following bullet points, along with my reasons for why they might not impact us
too badly:

* Imperfect initial planning: the Isobar article says that it was hard to roll
  out TCP_FASTOPEN, especially as the IANA's allocated option number differed
  from the experimental number. But it's 2023, and as far as I can tell
  everything uses the IANA allocated number now, so this is fine.
* Tracking concerns: We use TLS connections with certificates (or Noise
  connections with stable public keys), even with Uplinks. Any tracking that
  TCP_FASTOPEN enables is likely already possible. This is a concern we should
  think through but I don't believe TCP_FASTOPEN makes anything here
  fundamentally worse, especially for traffic between the Gateway-MT and storage
  nodes.
* Other performance improvements: yes, other options are available, and we're
  trying to use them! But this still affects us significantly, and HTTP/3 and
  TLS1.3 do not help us enough for us to not remain interested in this.

However, the fourth bullet point is a huge sticking point for us:

* Middleboxes: middleboxes drop TCP packets they don't understand, which means
  some connections will appear to die and never work when trying to use
  TCP_FASTOPEN. See "preliminary tests" below.

### Preliminary tests

So, I tried enabling this on a global test network of Uplinks, Satellites, and
storage nodes. I ran this on servers (storage nodes, Satellites):

```
sysctl -w net.ipv4.tcp_fastopen=3
```

And I added these patchsets:

 * https://review.dev.storj.io/c/storj/common/+/9252 (common/socket: enable tcp_fastopen_connect)
 * https://review.dev.storj.io/c/storj/storj/+/9251 (private/server: support tcp_fastopen)

It worked great! Awesome even! Shaved 150ms off most operations.
My review: A+ would enable again.

So, to try and get a better understanding of community support, we
submitted an earlier version of the design draft with a [Python
script to assist testing](https://gist.github.com/jtolio/57af177ae1fe5f0f255214b2c2ef90a1)
and failures were widespread. Many storage node operators had difficulty
using TCP packets with the FASTOPEN headers, and a wide variety of
indeterminate issues showed up. Lots of connections timed out and died.
Some operators had success, but many operators did not.

Worse - storage node operators pointed out that they don't have any
incentive to enable TCP_FASTOPEN since it may reduce the amount of uploads
they would otherwise receive, for all uploads that use a network path that
drops TCP_FASTOPEN packets. We need storage node operators to enable
kernel support, so we need a downside-free option for storage node operators.

So we can't rely on TCP_FASTOPEN working by itself.

## Design and implementation

But we can just try both, like we do with QUIC!

Here's the broad plan - we are going to just dial both standard TCP and
TCP_FASTOPEN in parallel for every connection and then pick the one that
worked faster. This will double the amount of dials we do, but won't
increase the amount of long-lived connections.

It turns out this is actually pretty complicated if Noise is in the
picture as well. With TCP_FASTOPEN and Noise together, the very first packet
will need to have request data inside of it, which means that we will
need to duplicate the request down to both sockets. This might not be
safe for the application.

We're going to focus on enabling this for both TLS and Noise, but not
for unencrypted connections.

So, the moving parts for this to work are:

 * Server-side socket settings and kernel config
 * Server-side request debouncing
 * Keeping track of what servers support debouncing
 * Duplicating dials and outgoing request writes (but not more than that!) and selecting
   a connection.
 * Client-side socket settings

### Socket settings and kernel config

So, we have to call Setsockopt on storage nodes and Satellites on the listening
socket to enable TCP_FASTOPEN, and we may need to tell the kernel with `sysctl`
to allow servers to use TCP_FASTOPEN.

The first step is accomplished with a small change
(https://review.dev.storj.io/c/storj/storj/+/9251). The second step is likely
an operator step that we will need storage node operators to do and include
in our setup instructions.

```
sysctl -w net.ipv4.tcp_fastopen=3
```

This may also need to be persisted in `/etc/sysctl.conf` or `/etc/sysctl.d`.

### Server-side request debouncing

The two types of messages we might see come off the network on a new socket
are a TLS client hello or a first Noise message.

In either case, we can quickly hash that message and see if we've already
seen that hash before. If we have, that means this duplicate message
came in second and we can close the socket and throw the message away.
Note that this does not provide any security guarantees or replay-attack
safety.

All TCP packets are subject to the IP
[TTL field](https://en.wikipedia.org/wiki/Time_to_live). In IPv4, it is
designed as a maximum time that a packet might live in the network, and
it should not last longer than something like 4 minutes. However, in
practice, the TTL field does not consider time and instead has often
been implemented as more of a hop-limit. IPv6 has been updated to
reflect its use as a hop limit. All that said, we can deduplicate
practically all second messages we receive with a small cache with a
memory on the order of 10 minutes.

Because it is not provably all packets, we should only use TCP_FASTOPEN
with Noise on replay safe requests (which we must do anyway with Noise_IK),
and we should only duplicate the TLS client hello with TCP_FASTOPEN and
not duplicate anything beyond it.

In summary: a small cache that considers the first message hashes
specifically of the Noise first packet or the TLS client hello, rejecting
duplicates, should be all we need here.

Relevant reviews:
 * https://review.dev.storj.io/c/storj/storj/+/9763

### Keeping track of what servers support debouncing

Once a server (node or Satellite) supports message debouncing, we need
to keep track of it, so we don't overwhelm nodes with duplicate messages
that don't know how to handle it efficiently.

This is pretty straightforward - we need to keep track of debounce
support per node in the Satellite DB.

We will simply assume at some later point that all Satellites support
debouncing.

Relevant reviews:
 * https://review.dev.storj.io/c/storj/common/+/9778
 * https://review.dev.storj.io/c/storj/storj/+/9779
 * https://review.dev.storj.io/c/storj/uplink/+/9930

### Duplicating dials and outgoing request writes

This is the hardest part of this plan.

Whenever we need a new connection to a peer, we are going to:

 * Immediately return a handle to a multidialer. Dialing will
   become a no-op and we will start dialing in the background.
 * In the background, dial one connection with standard TCP
 * Also backgrounded, dial a separate connection with
   TCP_FASTOPEN enabled. We will do this initially on Linux only
   with TCP_FASTOPEN_CONNECT, and then figure out how to use
   TCP_FASTOPEN. See the platform specific issues section about
   TCP_FASTOPEN_CONNECT.
 * Once something tries to write, we will copy the requested
   bytes and write to both sockets, once ready.
 * As soon as we need to read data for the first time, that is
   the cut off point. We will stop copying writes to both sockets.
   The first read to return data will be the connection we
   select and we will close the other one.

Relevant reviews:
 * https://review.dev.storj.io/c/storj/common/+/9858
 * https://review.dev.storj.io/c/storj/uplink/+/9859

### Client side socket settings

Clients are easier as they evidently don't need the `sysctl` call. They can be
implemented by calling Setsockopt on the sockets before the TCP dialer
calls connect, which Go now has functionality to allow (the Control option on
the net.Dialer). It can be done like this:
https://review.dev.storj.io/c/storj/common/+/9252

(but rolled into other changes)

You can see if TCP_FASTOPEN is working on the client side by running:
```
ip tcp_metrics show|grep cookie
```

### Consideration for clients

Clients may not be inside a network topology that allows for TCP_FASTOPEN.
A previous version of this design required the client to do something, but
by dialing both ways, this should work transparently for the client.

### Consideration for SNOs

Likewise, a previous version of this design required SNOs to consider whether
enabling TCP_FASTOPEN was advantageous. With the current design, it always
is.

### Platform specific issues

Linux supports a style of TCP_FASTOPEN on the client side called
TCP_FASTOPEN_CONNECT which is much better than standard TCP_FASTOPEN.
TCP_FASTOPEN by default requires including the initial bytes of the request in
a special syscall to the socket instead of calling connect(), which the Go
standard library does without really letting you do anything else.

https://github.com/torvalds/linux/commit/19f6d3f3c8422d65b5e3d2162e30ef07c6e21ea2

It may be that we can only easily achieve the benefits of TCP_FASTOPEN via
TCP_FASTOPEN_CONNECT for Linux-based clients. This is probably acceptable if
only due to the performance benefit Gateway-MT would gain.

### Cookie-based TCP_FASTOPEN

TCP_FASTOPEN's RFC has a provision for cookie-less connection setup, but
because we want to avoid amplification attacks, we should not use it,
without otherwise addressing amplification attacks.
Cookies provide amplification attack protection, which is important.
This proposal assumes the cookie-based TCP_FASTOPEN.

https://datatracker.ietf.org/doc/html/rfc7413

## Other options

### QUIC/UDT/another UDP protocol

We're asking TCP to avoid a handshake, but in truth, this is one of the reasons
we want our network to be configured to work with UDP packets, is so that we can
use a TCP-like session protocol on top of UDP without requiring handshakes where
we can avoid it.

TCP_FASTOPEN is nice because it otherwise uses TCP, but of course the major
downside is could confuse middleware boxes and make it so connections timeout
and die.

I don't know the relative rate between TCP_FASTOPEN and UDP-oriented connection
failure though. We suspect that UDP middleware problems have affected our
rollout of QUIC, so perhaps TCP_FASTOPEN won't be as challenging.

## Wrapup

## Related work

The Noise over TCP blueprint:
https://github.com/storj/storj/blob/main/docs/blueprints/noise.md



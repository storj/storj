# TCP_FASTOPEN

## Abstract

This design doc discusses how we can achieve a large connection set up
performance improvement by selectively enabling TCP_FASTOPEN when it makes
sense to do so.

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
* Middleboxes: it's true that due to middleboxes dropping TCP packets they don't
  understand, trying to use TCP_FASTOPEN may mean that some connections appear
  to die and never work. Evidently Chrome's attempts to use TCP_FASTOPEN were
  too challenging to keep track of which routes supported TCP_FASTOPEN and which
  didn't. But we have a different context! We don't need all routes to support
  TCP_FASTOPEN, just a majority of them. We may not want to use TCP_FASTOPEN
  between the Uplink and the Satellite in general, but we could likely reliably
  use it between the Gateway-MT and the Satellite, and between Uplinks and
  storage nodes. It's also 2023, and more middleboxes are likely to not be dumb.
* Tracking concerns: We use TLS connections with certificates (or Noise
  connections with stable public keys), even with Uplinks. Any tracking that
  TCP_FASTOPEN enables is likely already possible. This is a concern we should
  think through but I don't believe TCP_FASTOPEN makes anything here
  fundamentally worse, especially for traffic between the Gateway-MT and storage
  nodes.
* Other performance improvements: yes, other options are available, and we're
  trying to use them! But this still affects us significantly, and HTTP/3 and
  TLS1.3 do not help us enough for us to not remain interested in this.

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

## Design and implementation

So, we have to call Setsockopt on storage nodes and Satellites on the listening
socket to enable TCP_FASTOPEN, and we may need to tell the kernel with `sysctl`
to allow servers to use TCP_FASTOPEN.

The first step is accomplished with a small change
(https://review.dev.storj.io/c/storj/storj/+/9251). The second step is likely
an operator step that we will need storage node operators to do and include
in our setup instructions.

You can see if TCP_FASTOPEN is working on the client side by running:
```
ip tcp_metrics show|grep cookie
```

Clients are easier as they evidently don't need the `sysctl` call. They can be
implemented by calling Setsockopt on the sockets before the TCP dialer
calls connect, which Go now has functionality to allow (the Control option on
the net.Dialer). It can be done like this:
https://review.dev.storj.io/c/storj/common/+/9252

### Consideration for clients

Clients may not be inside a network topology that allows for TCP_FASTOPEN. In
these cases, clients will likely want to disable the feature. In these
scenarios, I would imagine we can identify the vast majority of cases with the
Satellite connection directly. If the Satellite connection has trouble, then we
should just disable TCP_FASTOPEN use.

Otherwise, if clients are inside a network topology that isn't dropping packets
with TCP_FASTOPEN, then they benefit the most from a network of storage nodes
that support it.

### Consideration for SNOs

SNOs also will not want to enable support for TCP_FASTOPEN unless their network
topology supports it (most seem to). Luckily, TCP_FASTOPEN is only attempted if
both the client and server signal that they support TCP_FASTOPEN, so SNOs who
keep TCP_FASTOPEN disabled won't have a complete failure for clients that are
trying.

SNOs will have a strong incentive to enable TCP_FASTOPEN if they can though -
our upload and download races will prefer nodes that finish faster, and
TCP_FASTOPEN eliminates hundreds of milliseconds of penalty. Nodes that enable
TCP_FASTOPEN are going to win way more upload/download races. We should help
node operators set up and configure TCP_FASTOPEN.

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



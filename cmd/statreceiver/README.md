# statreceiver

This package implements a Lua-scriptable pipeline processor for
[zeebo/admission](https://github.com/zeebo/admission) telemetry packets (like
[monkit](https://github.com/spacemonkeygo/monkit/) or something).

There are a number of types of objects involved in making this work:

 * *Sources* - A source is a source of packets. Each packet is a byte slice that,
   when parsed, consists of application and instance identification information
   (such as the application name and perhaps the MAC address or some other id
   of the computer running the application), and a list of named floating point
   values. There are currently two types of sources, a UDP source and a file
   source. A UDP source appends the current time as the timestamp to all
   packets, whereas a file source should have a prior timestamp to attach to
   each packet.
 * *Packet Destinations* - A packet destination is something that can handle
   a packet with a timestamp. This is either a packet parser, a UDP packet
   destination for forwarding to another process, or a file destination that
   will serialize all packets and timestamps for later replay.
 * *Metric Destinations* - Once a packet has been parsed, the contained metrics
   can get sent to a metric destination, such as a time series database, a
   relational database, stdout, a metric filterer, etc.

Please see example.lua for a good example of using this pipeline.

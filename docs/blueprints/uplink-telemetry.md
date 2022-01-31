# Uplink Telemetry

## Abstract

Our telemetry uses monkit to monitor various functions and other tasks.
Currently we do not collect any data from uplinks. This design doc proposes how
to collect telemetry data from uplinks.

## Background

Uplinks currently do not send any telemetry data. As we move into production we
want to collect this data from uplinks so we can monitor the aggregate health of
uplinks, include their data in distributed tracing applications, and generally
gain insight into the entire system end-to-end.

Uplinks present a challenge for collecting this data for a number of reasons:

1. Many uplink operations are short in duration, so a chore that periodically
   sends to the collector may not execute, or may miss many metrics.
2. We cannot control the configuration of uplinks that are simply using the
   library code, so if we rely on configuration to control where and when
   metrics are sent we will miss many metrics.

## Design

### Problem 1: When to send uplink telemetry data

We will tie the flushing of metrics to the uplink library's `OpenProject` and
`Project.Close` calls.

- `OpenProject` will start a periodic flush so that long-running and long-used
  projects will periodically flush metrics. It is anticipated that most uplink
  use-cases will not hold open projects long enough for this flush to occur. But
  if an uplink opens a project for a long duration to perform many operations,
  this periodic flush will ensure a steady flow of metrics. This will likely
  mean we store a loop on the project which will be stopped during project
  close.
- `Project.Close` will flush metrics to the collector and stop the periodic
  flushing mentioned above. The final flush will need to use the background
  context, and will need to be a blocking call to ensure the metrics are
  actually sent.

### Problem 2: How to configure uplinks to send telemetry data

We will hard-code the URL for our default collector, only to be used for release
builds. If users do not want to send telemetry information they can override
this setting with an empty string.

## Rationale

The advantages of this approach is that each uplink call will flush telemetry
data, regardless of whether we control the binary using the uplink code.

Disadvantages include that we need to hard-code the URL of where to send the
data. An alternate approach include:

- Have a service discovery feature on each satellite which returns the URL of
  where to send telemetry information. This allows other satellite operators to
  use uplink libarary without accidentally sending the data to us, but is more
  complicated.
- Send key metrics directly to a satellite endpoint after specific operations.
  This prevents the need for an additional URL for uplink telemetry, and makes
  it easy to track data per satellite. The disadvantage is this requires
  additional round-trips for operations.

## Implementation

Implementation should be simple, simply add the logic described above to the
uplink project.

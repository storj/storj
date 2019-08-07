# Uptime reputation caculation

## Abstract

Having a fair, simple and understandable way to calculate the uptime of _storagenodes (henceforth SNs)_ is crucial for not causing unfair disqualifications, clarifying how _storagenode operators (henceforth SNOs)_ should administrate and maintain their _SN_ for being align with the network requirements and last but not least have a clear proof of disqualification for _SN_ getting disqualified due to too many uptime checks failures.

This design doc propose a different way for calculating uptime check scores.

## Background

Currently our disqualification system is described in the [disqualification design doc](disqualification.md) which is based in the calculation of different _SN_ reputation scores as described in [node-selection design doc](node-selection.md).

The problem with the current uptime _SN_ reputation score calculation is that it isn't based on the actual time that a node is being offline, by cause of to the nature of the chosen model, causing that a _SN_ will get disqualified due its number of uptime checks success and failure which implies that new nodes can get disqualified having less uptime check failures than veteran nodes.

While we understand that a _SN_ that has remained in the network with a low uptime check failures could be considered a more reliable _SN_ than new ones which recently joined, we see, at the same time, that new _SN_ shouldn't be harsher pernalized than vetaran nodes due to uptime check failures because:

1. Any _SN_ (veteran and new ones) requires some time to be offline for maintenance and both should have the same maximum amount of time for it.
1. The end goal of the uptime checks is to ensure that the network has certain availability and retrievability and uptime check failures affects to them no matter on how long the _SN_ has remained in the network and how much data a _SN_ has, hence node with more or less data must be treated equally.

This design doc is focused in how to design and implement a fair, simple and understandable uptime reputation calculation after our initial approach didn't behave as expected causing the disqualification of several nodes without having a proof that they have failed enough uptime checks that the _SNs_ were offline long time enough which put in danger the minium level of the network availability and retrievability.

## Design

The new process for doing the uptime checks is based on once a _SN_ is selected for having the check, if the _SN_ fails the initial check, then it's tracked to periodically receive uptime checks until (a) it successfully pass an uptime check or (b) gets disqualified due to many uptime check failures in one month.

Once a _SN_ has failed the first uptime check, the _SN_ is considered offline and the subtotal time, which the _SN_ is considered offline, is the time elapsed between the first and the last failed uptime check. The total offline time of a _SN_ is the accumulation of the subtotals that the _SN_ has had during the ongoing month and when such total surpasses the maximum permitted percentage time, the _SN_ will get disqualified.

The maximum percentage of time that a _SN_ can be offline will be estalbished by the network commiment in terms of availability and retrievability during one month.

The process is divided in 2 parts:

1. Selecting _SNs_ for receiving uptime checks.
1. Periodically check the _SNs_ which has failed the first uptime check until they pass one or get disqualified.

For the first part, we foresee to use the current implemented mechanism with some minor modifications for excluding the _SNs_ which are being check by the second part.

The second part requires a totally new implementation which should be totally independent of any current satellite service for being able in the near future to run it separately for scalability purpose.


## Rationale

[A discussion of alternate approaches and the trade offs, advantages, and disadvantages of the specified approach.]

## Implementation

[A description of the steps in the implementation.]

## Open issues (if applicable)

[A discussion of issues relating to this proposal for which the author does not
know the solution. This section may be omitted if there are none.]

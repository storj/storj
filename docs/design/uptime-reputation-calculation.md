# Uptime reputation caculation

## Abstract

Having a fair, simple and understandable way to calculate the uptime of _storagenodes (henceforth SNs)_ is crucial for not causing unfair disqualifications, clarifying how _storagenode operators (henceforth SNOs)_ should administrate and maintain their _SN_ for being align with the network requirements and last but not least have a clear proof of disqualification for _SN_ getting disqualified due to too many uptime check failures.

This design doc proposes a different way for calculating uptime check scores.

## Background

Currently our disqualification system is described in the [disqualification design doc](disqualification.md) which is based in the calculation of different _SN_ reputation scores as described in [node-selection design doc](node-selection.md).

The problem with the current uptime _SN_ reputation score calculation is that it isn't based on the actual time that a node is being offline, by cause of to the nature of the chosen model, causing that an _SN_ will get disqualified due its number of uptime checks success and failure which implies that new nodes can get disqualified having less uptime check failures than veteran nodes.

While we understand that an _SN_ that has remained in the network with a low uptime check failures could be considered a more reliable _SN_ than new ones which recently joined, we see, at the same time, that new _SN_ shouldn't be harsher penalized than veteran nodes due to uptime check failures because:

1. Any _SN_ (veteran and new ones) requires some time to be offline for maintenance and both should have the same maximum amount of time for it.
1. The end goal of the uptime checks is to ensure that the network has certain availability and retrievability and uptime check failures affects to them no matter on how long the _SN_ has remained in the network and how much data an _SN_ has, hence node with more or less data must be treated equally.

This design doc is focused on how to design and implement a fair, simple and understandable uptime reputation calculation after our initial approach didn't behave as expected, causing the disqualification of several nodes without having a proof that they have failed enough uptime checks that the _SNs_ were offline long time enough which put in danger the minimum level of the network availability and retrievability.

## Design

The new process for doing the uptime checks is based on once an _SN_ is selected for having the check, if the _SN_ fails the initial check, then it's tracked to periodically receive uptime checks until (a) it successfully passes an uptime check or (b) gets disqualified due to many uptime check failures in one month.

Once an _SN_ has failed the first uptime check, the _SN_ is considered offline and the subtotal time, which the _SN_ is considered offline, is the time elapsed between the first and the last failed uptime check. The total offline time of an _SN_ is the accumulation of the subtotals that the _SN_ has had during the ongoing month and when such total surpasses the maximum permitted percentage time, the _SN_ will get disqualified.

The maximum percentage of time that an _SN_ can be offline will be established with the network commitment in terms of availability and retrievability during one month.

The process is divided in 2 parts:

1. Selecting _SNs_ for receiving uptime checks.
1. Periodically check the _SNs_ which has failed the first uptime check until they pass one or get disqualified.

For the first part, we foresee to use the current implemented mechanism with some minor modifications for excluding the _SNs_ which are being checked by the second part.

The second part requires a totally new implementation which should be totally independent of any current satellite service for being able in the near future to run it separately for scalability purpose.


## Rationale

The approach described in this document present the following advantages:

1. It's clear why an _SN_ get disqualified due to failed uptime checks.
1. It has a clear proof for an _SNOs_ why their _SNs_ get disqualified due to failed uptime check in case that they request it before the designed time to clean up the historical failed uptime checks data.
1. It allows to report to _SN_ at any how far is to be disqualified by failing uptime checks.

And the following disadvantages

1. It requires to store, for some period of time, historical for nodes which have failed at least one uptime check.
1. It requires more resources than the current implemented approach.
1. It doesn't calculate the exact time that an _SN_ is offline; once an _SN_ has a failed uptime check the time elapsed between the last failed uptime check and the next succeeded uptime check causes not to have the accurate offline time.
1. The _"score"_ obtained from uptime check failures, without any further modification, doesn't integrate with the current _SN reputation system_.

While one of our goals is to reduce the number of resources used by the satellite we see that the advantages beat the disadvantages considering that the current system got disabled after some nodes got disqualified unfairly and/or unclear.

On the other hand, we think that the number of resources required by this new system shouldn't be too much and we believe that we can have an implementation that scales up.

## Implementation

### DB

A database is required to store information about _SNs_ uptime check failures. This section presents the new required database schema and a brief explanation what it will store.

Only one database table is required and its schema is the following (pseudo code):

```sql
CREATE TABLE failed_uptime_checks (
    node_id       BYTEA NOT NULL,
    when          TIMESTAMP WITH TIMEZONE NOT NULL, -- When the SN failed the an uptime check until it succeeds a future uptime check
    back_online   TIMESTAMP WITH TIMEZONE, -- When SN has succeeded an uptime check after the last one failed
    disqualified  TIMESTAMP WITH TIMEZONE  -- When the SN has been disqualified by any disqualification system (uptimes, audits, etc.)
)
```

The table will be filled with _SNs_ which have failed at least one uptime check during the ongoing month.

Each row in the table indicates when an _SN_ failed the first uptime check (`when`), while the _SN_ not succeed any future uptime check (`back_online` is `NULL`); when the _SN_ is back online any future uptime check failure will insert a new row to mark a new period of time that the node is offline again.

When the _SN_ is disqualified by any disqualification system and the _SN_ isn't back online, the `disqualified` column of the row will be updated for not receiving more uptime checks because we don't do business with disqualified _SNs_.

### Algorithm

The algorithm is composed of 2 parts:

1. Select which _SNs_ will be checked for uptime, henceforth uptime check _SN_ selection.
1. Recheck the uptime of the _SNs_ which has failed the initial uptime check (part 1) during an interval period of time, henceforth uptime recheck loop.

#### Uptime Check _SN_ selection

This design doc doesn't change the _SN_ selection process for uptime checks therefore the current implemented mechanism with some minor modifications to classify those nodes which fail the uptime check in order of being recheck by the second part of the algorithm.

Once an _SN_ is selected for an uptime check, the current implemented uptime check mechanism must be modified to achieve the following algorithm:

1. If there is a row in the `failed_uptime_checks` table for the selected _SN_ with `back_online` set to `NULL` do nothing, otherwise follow with the next step.
1. Check the _SN_ as it's currently done; if it succeeds, then end, otherwise follow with the next step.
1. Insert a new row in the `failed_uptime_checks` using its node ID and setting the current timestamp to `when`.


#### Uptime Recheck Loop

This process should run independently from any current satellite process and it should run in a configurable time interval.

The algorithm for each time interval iteration is the following:

1. Select all the rows from `failed_uptime_checks` table which has `back_online` and `disqualification` columns set to `NULL` sorted by `when` in ascending order.
1. For each selected _SN_, retrieve the _SN_ address<sup>1</sup> and performs the uptime check.
    * If it fails
        1. Calculate its total offline time of the month, accumulating the offline time interval of each row corresponding to this _SN_ with `back_online` not being `NULL` and the current offline interval time calculated with the current timestamp and the `when` value of this row. If the total offline time doesn't exceed the established network limits end, otherwise follow with the next step.
        2. Update the `disqualified` column of the row with the current timestamp and disqualifies the _SN_<sup>2</sup>
    * If it succeeds
        1. Update the `back_online` column of the row with the current timestamp.


## Open issues

Currently we have the following issues and concerns which can affects somehow the implementation of this system:

1. Currently the uptime checks are combined with the audit check for calculating the total _SN_ reputation; after this implementation, the uptime check _"score"_ cannot be combined anymore as before.
1. The implementation of this system requires to interact with other satellite processes, <sup>1</sup> getting the _SN_ address and <sup>2</sup> disqualify a _SN_; for keeping this system uncoupled of those processes and being able to scale up the satellite, it should be a way to perform those operations through an interface which will remain once the satellite will be broken into different distributed system.
1. The implementation of the _uptime recheck loop_ mentions to retrieve a list of _SNs_ in order of being able to maintain the order, however, it may be a problem with it in order of holding the result set open too long, impacting the perfomance of database.
1. The implementation of _uptime recheck loop_ section should contain a protocol buffers definition in case that the process will run on a different machine; this part hasn't been elaborated because we may decide on not doing it for the first version but we should consider it during the implementation for being able to adapt it with the minimal modifications.

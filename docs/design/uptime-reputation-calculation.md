# Uptime reputation caculation

## Abstract

Having a fair, simple and understandable way to calculate the uptime of _storagenodes (henceforth SNs)_ is crucial for not causing unfair disqualifications, clarifying how _storagenode operators (henceforth SNOs)_ should administrate and maintain their _SN_ for being align with the network requirements and last but not least have a clear proof of disqualification for _SN_ getting disqualified due to too many uptime check failures.

This design doc proposes a different way for calculating uptime check scores.

## Background

Currently our disqualification system is described in the [disqualification design doc](disqualification.md) which is based in the calculation of different _SN_ reputation scores as described in [node-selection design doc](node-selection.md).

The problem with the current uptime _SN_ reputation score calculation is that it isn't based on the actual time that a node is being offline, by cause of to the nature of the chosen model, causing that an _SN_ will get disqualified due its number of uptime checks success and failure which implies that new nodes can get disqualified having less uptime check failures than veteran nodes.

While we understand that an _SN_ that has remained in the network with a low uptime check failures could be considered a more reliable _SN_ than new ones which recently joined, we see, at the same time, that new _SN_ shouldn't be harsher penalized than veteran nodes due to uptime check failures because:

1. Any _SN_ (veteran and new ones) requires some time to be offline for maintenance and both should have the same maximum amount of time for it.
1. The end goal of the uptime checks is to ensure that the network has certain availability and retrievability and uptime check failures affect to them no matter on how long the _SN_ has remained in the network and how much data an _SN_ has, hence node with more or less data must be treated equally.

This design doc is focused on how to design and implement a fair, simple and understandable uptime reputation calculation after our initial approach didn't behave as expected, causing the disqualification of several nodes without having a proof that they have failed enough uptime checks that the _SNs_ were offline long time enough which put in danger the minimum level of the network availability and retrievability.

## Design

The new process for doing the uptime checks is based on once an _SN_ is selected for having the check, if the _SN_ fails the initial check, then it's tracked to periodically receive uptime checks until (a) it successfully passes an uptime check or (b) gets disqualified due to be offline the maximum allowed by the established number of days (henceforh uptime check period) or it isn't needed to be checked anymore because it has been excluded from the network by any reason.

Once an _SN_ has failed the first uptime check, the _SN_ is considered offline and the subtotal time, which the _SN_ is considered offline, is the time elapsed between the first and the last failed uptime check. The total offline time of an _SN_ is the accumulation of the subtotals that the _SN_ has had during the ongoing uptime check period and when such total surpasses the maximum permitted percentage offline time, the _SN_ will get disqualified.

The maximum percentage of time that an _SN_ can be offline will be established with the network commitment in terms of availability and retrievability during one month.

The process is divided in 2 parts:

1. Selecting _SNs_ for receiving uptime checks.
1. Periodically check the _SNs_ which have failed the first uptime check until they pass one or get disqualified or get excluded from the network.

For the first part, we foresee to use the current implemented mechanism with some minor modifications for excluding the _SNs_ which are being checked by the second part.

The second part requires a totally new implementation which should be totally independent of any current satellite service for being able in the near future to run it separately for scalability purpose.


## Rationale

The approach described in this document present the following advantages:

1. It's clear why an _SN_ get disqualified due to failed uptime checks.
1. It has a clear proof for an _SNOs_ why their _SNs_ get disqualified due to failed uptime check in case that they request it before the designed time to clean up the historical failed uptime checks data.
1. It allows reporting to an _SN_, at any point, how far it's from disqualified by failing uptime checks.

And the following disadvantages

1. It requires the satellite to store, for some period of time, historical data for nodes which have failed at least one uptime check.
1. It requires more resources than the current implemented approach.
1. It doesn't calculate the exact time that an _SN_ is offline; once an _SN_ has a failed uptime check the time elapsed between the last failed uptime check and the next succeeded uptime check causes not to have the accurate offline time.
1. There isn't a _"score"_ obtained from uptime check failures which can be integrate with the current _SN reputation system_.

While one of our goals is to reduce the number of resources used by the satellite we see that the advantages beat the disadvantages considering that the current system got disabled after some nodes got disqualified unfairly and/or unclear.

On the other hand, we think that the number of resources required by this new system shouldn't be too much and we believe that we can have an implementation that scales up.

## Implementation

### DB

A database is required to store information about _SNs_ uptime check failures. This section presents the new required database schema and a brief explanation what it will store.

A database table is required for registering _SNs_ uptime check failures and its schema is the following (pseudo-code):

```sql
CREATE TABLE failed_uptime_checks (
    node_id       BYTEA NOT NULL,
    when          TIMESTAMP WITH TIMEZONE NOT NULL, -- When the SN failed the an uptime check until it succeeds a future uptime check
    back_online   TIMESTAMP WITH TIMEZONE,          -- When SN has succeeded an uptime check after the last one failed
    last_check    TIMESTAMP WITH TIMEZONE,          -- When the last uptime check was done
    count         NUMBER,                           -- Number of uptime rechecks
    disqualified  TIMESTAMP WITH TIMEZONE           -- When the SN has been disqualified by any disqualification system (uptimes, audits, etc.)
)
```

The table will be filled with _SNs_ which have failed at least one uptime check during the ongoing uptime check period.

Each row in the table indicates when an _SN_ failed the first uptime check (`when`), while the _SN_ didn't succeed any future uptime check (`back_online` is `NULL`); when the _SN_ is back online any future uptime check failure will insert a new row to mark a new period of time that the node is offline again.

The `last_check` column register when the last uptime check has been done; although we could roughly know how many checks an _SN_ has had using the `when` column and the interval recheck time, this value allows to (in importance order):

1. When the satellite starts due to some failure, if there were any entry in this table in the ongoing uptime check period with `back_online` and `disqualified` set to `NULL`, the satellite should start the rechecks for those and if they don't have a last contact after `last_check` but they pass the uptime check, we can use `last_check` value for setting it to `back_online` and not unfairly mark those _SNs_ with more offline time that they could have, due to the fact that the satellite had a failure.
1. It allows retrieving the next _SN_ to recheck without having to execute a query that returns multiple rows, hence have to maintain opened a result set during all the time that the checks can take.
1. It allows to precisely know when the last check happened.
 
The `count` column could be avoided and have a rough calculation of the number of checks that have been done until the _SN_ is back online or disqualified using the interval recheck time, however, this column precisely retains those and it's valuable for clarifying uptime failures disqualification to _SNs_ which require a proof. Furthermore, the interval recheck time can be altered during the same uptime check period and without tracking those alterations we won't have a good calculation of the number of checks.

When the _SN_ is disqualified by any disqualification system and the _SN_ isn't back online, the `disqualified` column of the row will be updated for not receiving more uptime checks because we don't do business with disqualified _SNs_.

Another database table is required to register when the uptime check periods have started, its schema is the following (pseudo-code):

```sql
CREATE TABLE uptime_check_periods (
    start   TIMESTAMP WITH TIMEZONE NOT NULL,

    PRIMARY KEY (start)
)
```

This table allows the satellite to be restarted and carries on with the uptime rechecks of the current uptime check period.

### Algorithm

The algorithm is composed of 2 parts:

1. Select which _SNs_ will be checked for uptime, henceforth uptime check _SN_ selection.
1. Recheck the uptime of the _SNs_ which has failed the initial uptime check (part 1) during an interval period of time, henceforth uptime recheck loop.

#### Uptime Check _SN_ selection

This design doc doesn't change the _SN_ selection process for uptime checks therefore the current implemented mechanism with some minor modifications to classify those nodes which fail the uptime check in order of being recheck by the second part of the algorithm.

Once an _SN_ is selected for an uptime check, the current implemented uptime check mechanism must be modified to achieve the following algorithm:

1. If there is a row in the `failed_uptime_checks` table for the selected _SN_ with `back_online` set to `NULL` do nothing, otherwise follow with the next step.
1. Check the _SN_ as it's currently done; if it succeeds, then end, otherwise follow with the next step.
1. Insert a new row in the `failed_uptime_checks` using its node ID and setting the current timestamp to `when` and `last_check`.


#### Uptime Recheck Loop

This process should run independently from any current satellite process and it should run in a configurable time interval.

The algorithm for each time interval iteration is the following:

1. Select the first row from `failed_uptime_checks` table which has `back_online` and `disqualified` columns set to `NULL` sorted by `last_check` in ascending order. If there is no row, ends (the process will be executed in the next interval).
1. For the selected _SN_, retrieve the last time that the _SN_ has contacted the satellite.
   If last contacted time is greater than the `last_check`, update the row setting `back_online` to such value and `last_check` to the current timestamp and go to 1, otherwise, continue.
1. Retrieves the _SN_ address<sup>1</sup> and performs the uptime check.
   If it succeeds, update the row setting `back_online` and `last_check` to the current timestamp and increment `count` and go to 1, otherwise continue.
1. Calculate its total offline time of the uptime check period, accumulating the offline time interval of each row corresponding to this _SN_ with `back_online` not being `NULL` and the current offline interval time calculated with the current timestamp and the `when` value of this row. If the total offline time doesn't exceed the established network limits go to 1, otherwise continue.
1. Update the row setting `disqualified` and `last_check` to the current timestamp and increment `count`; then disqualifies the _SN_<sup>2</sup>.


#### Configurable parameters

The following parameters will be configurable:

1. Uptime check period: It's the number of days where uptime check failures are accumulated for calculating the total offline time of _SNs_. Default 30 days.
1. Maximum allowed offline time. It's the percentage of the uptime check period that _SNs_ can be offline for not being disqualified. Default 0.05%.
1. Uptime recheck interval. It's the time interval that _SNs_ which failed the initial uptime check are rechecked until they are back online, disqualified or we stop to check them because they are not in the network anymore. Default 1 hour.

## Open issues

Currently we have the following issues and concerns which can affects somehow the implementation of this system:

1. Currently the uptime checks are combined with the audit check for calculating the total _SN_ reputation; after this implementation, the uptime check doesn't present a _"score"_ which can directly combined with.
1. The implementation of this system requires to interact with other satellite processes, <sup>1</sup> getting the _SN_ address and <sup>2</sup> disqualify a _SN_; for keeping this system uncoupled of those processes and being able to scale up the satellite, it should be a way to perform those operations through an interface which will remain once the satellite will be broken into different distributed system.
1. The implementation of _uptime recheck loop_ section should contain a protocol buffers definition in case the process run on a different machine; this part hasn't been elaborated because we may decide on not doing it for the first version but we should consider it during the implementation for being able to adapt it with the minimal modifications.

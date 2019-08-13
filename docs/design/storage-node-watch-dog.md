# Storage Node Watch Dog

## Abstract

The Storage Node Watchdog monitors storage nodes with periodic pings and restarts a storage node if a crash/unresponsiveness is detected.

## Background

Docker is currently used to maintain storage nodes and watchdog is being used to monitor the storage node health within the docker container.
We are migrating away from Docker and we need to implement a process for monitoring Storage Node health.

## Design

Storage Node
* Pulse endpoint on storage node which responds that it is running.

Watchdog Process
* Process runs on a loop periodically checking a Storage Node's pulse.
* Must be able to kill an already existing Storage Node process.
* Must be able to start a new Storage Node process.

## Implementation

1) Update grpc proto to have a Pulse endpoint on Storage Nodes.
2) Write function for receiving Pulse check requests on Storage Node.
3) Add timers to ea
4) Write pulse check service
    * Loop for periodically checking pulse of Storage Node
    * If the Storage Node can't be contacted we should try restarting the Storage Node process.
    * If the Storage Node doesn't respond for (30?) seconds we should assume the Storage Node is locked and restart the Storage Node process.

## Open issues (if applicable)

* Should the pulse endpoint be a private endpoint?
* Should we add timers that start at the beginning of each Storage Node endpoint, keep track of all the timers, and report in the pulse check if the timers have unusually high lengths of time?

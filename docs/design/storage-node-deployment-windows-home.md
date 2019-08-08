# Storage Node Automatic Updates and Installation for Windows

## Overview

As more Storage Node Operators join the network we must ensure these nodes have a mechanism to automatically keep their nodes software up to date.
If a Storage Node Operator fails to keep their node up to date with the minimum version required by satellites they will no longer be selected for upload or download requests.

Currently we are using Docker for automatic updates which means we require Docker to be installed.
Docker is being used for many other things as well, so we need to cover these cases as well.

## Goals

To deploy the automatic updates we need to handle the following cases that docker handles. Docker is not supported on windows home so we will ensure an automatic update system is built into the Storage nodes. 
We also need to ensure that it plays nicely with all the common anti-viruses and firewalls.

Docker is being used for:

* Automatic Updates (with watchtower)
* Restarting on crash
* Logging (kind of)

## Services

* Installer
    * Must be run with admin privileges.
    * Installs the automatic updater binary and error gui application. (msi)
    * Sets up automatic updater as a windows Service with sufficient privileges.
* Automatic Updater (binary)
    * Downloads storage node binary, Sets up storage node, and runs the watchdog process.
    * Doesn't start the storage node if storage node has not created a valid config.
    * Send error reports to satellite.
    * Writes update related errors to log file.
    * Watchdog process
        * Monitors storage node by periodically sending messages to pulse endpoint on storage node and waiting for responses.
        * Restarts the storage node if a crash/unresponsiveness is detected.
* Storage Node (binary)
    * Shares drive with satellite network.
    * Writes storage node operation related errors to log file
* Error gui application
    * Shows errors from log file
    * Notifies user of service errors.
    * Saves last reported error timestamp to a file for knowing if there are unread errors.

## Handling failures in Automatic Updates

General
* Windows firewall and other 3rd party firewalls can block storage node operations.
    * [isportallowed](https://docs.microsoft.com/en-us/windows/win32/api/netfw/nf-netfw-inetfwmgr-isportallowed) windows api function can be used to make sure we are allowed through firewall
    * Can we add code to detect if firewall is blocking storage node operations?
    * Unblock storage node operator in firewall settings.
    * Can we detect if windows firewall is running?

When downloading a new binary:
* download fails
    * Log error in log file if possible
    * Retry download on next cycle that checks if storage node is up to date.
* out-of-space for downloading
    * Log error in log file if possible
    * Retry download on next cycle that checks if storage node is up to date.
* filesystem read-only
    * Log error in log file if possible
    * Retry download on next cycle that checks if storage node is up to date.
* Man in the Middle attacks/corrupted binary
    * Log error in log file if possible
    * Verify binary hashes with message from version server and with output of a hashing algorithm (shasum256)
    * Retry download on next cycle that checks if storage node is up to date.

When swapping in a new binary:
* computer crashes during swapping
    * Log error in log file if possible
    * automatic updater checks binary version and reruns download/swap steps.
* deletion/stopping of the old binary fails.
    * Log error in log file if possible
    
When starting a new binary:
* out-of-space during migrations
    * Log error in log file if possible
    * automatic updater will try to rerun the binary on next cycle
* failure to start
    * Log error in log file if possible
    * automatic updater will try to rerun the binary on next cycle
* not yet configured
    * Log error in log file if possible
    * storage node will run a setup or describe out to fix the problem...?

When performing a gradual rollouts.
* bad gradual rollout. We know it's a bad rollout if our application stops working
    * Log error in log file if possible
    * if we did have a database migration, api/grpc change, or file system change in the latest update then wait for next update???
        * bad latest update might have had faulty database changes that will need to be migrated again.
    * otherwise rollback to previous version

### Starting Storage Node binary on OS Start-Up

We need to ensure that updater binary starts on computer start-up,
without logging into the system, and this updater binary launches the storage node This is achieved when installing via the msi.
Avoid triggering UAC.

### Restarting Storage Node binary on Crash / Problems

We need to ensure that storage node binary restarts after a crash.

* detect crashes and detect unresponsiveness
    * updater binary checks pulse of storage node binary with ipc messages. storage node will have a pulse endpoint and the updater hits that endpoint with timeouts.
    * A windows service can be configured to restart the service on fail/crash.
   
### Logging

* Log to disk.
* Rotate files.
* Compress old stuff.
* Delete really old stuff.
* Ensure we limit the size of logs...

### Resource Limits

Ensure we:
* can set limits to memory usage,
* can set limits to CPU usage.
* send graceful shutdown message and restart storage node if memory usage is too high.
* Windows specific apis also exist for limiting process memory and cpu usage.
* limit CPU might be to limit number of cores it runs on in go.

## Testing

Verify program can be run with windows defender firewall.

## Design Overview

msi installer

automatic updater

storage node
When starting a Storage Node:
* Main process starts an updater service which starts a (12 hour?) interval loop
* Updater service spawns a Storage Node process 
* Updater service Interval checks the current Storage Node version and compares with the version server ("https://version.alpha.storj.io/") 
* Updater service Downloads the Binary for the minimum version returned from the version server
* Updater service sends a message through a channel to Storage Node process to kill it
* Updater service spawns a new Storage Node process

### Rollout message structure

* each automatic updater process will poll our version server from time to time
* our version server will return some data of the following form:
```json
{
  "processes": {
    "storagenode": {
      "allowed_version_minimum": "0.3.4",
      "suggested_version": "0.5.1",
      "rollout": {
        "active": true,
        "target_version": "0.5.2",
        "rollout_seed": "04123bacde",
        "rollout_cursor": "40",
      }
    }
  }
}
```

* independent of an active rollout, a process will confirm that it at least meets the allowed version minimum. if it does not, it will proceed to upgrade to at least the suggested_version if it is not part of a rollout.
* if a rollout is active, it will hash its own node id with the rollout seed and compare that hash to the rollout cursor. if it sorts less then the rollout cursor it should upgrade to the rollout target version
* also, we need to make sure to add jitter. see http://highscalability.com/blog/2012/4/17/youtube-strategy-adding-jitter-isnt-a-bug.html. having every process restart and sleep 12 hours is a definite way to kill ourselves without adding some randomness back in.

## Implementation Milestones

* Design automatic updater service
    * Create and start an updater service that runs on an interval loop
    * Determine which libraries will be used for the binary downloading
      * https://github.com/rhysd/go-github-selfupdate
      * Vet dependencies
* Implement rhysd/go-github-selfupdate 
* create msi installer
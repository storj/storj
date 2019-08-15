# Storage Node Automatic Updater

## Abstract

Automatic Updater is a process separate from the Storage Node that automatically downloads the latest Storage Node binary to replace the current Storage Node binary.

## Background

As more Storage Node Operators join the network we must ensure these nodes have a mechanism to automatically keep their nodes software up to date.
If a Storage Node Operator fails to keep their node up to date with the minimum version required by satellites they will no longer be selected for upload or download requests.
Currently we are using Docker for automatic updates but we are migrating away from docker so we need to write out own automatic update utility for Storage Nodes.

## Design

* Contacts Version Server to determine if Storage Node needs to be updated.
* Downloads latest Storage Node binary.
* Validate binary signature.
* Replaces current Storage Node binary with latest Storage Node binary.
* Kills current Storage Node binary process.

### Rollout message structure

* Te Automatic Updater process will poll our version server from time to time
* Our version server will return some data of the following form:

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

* Independent of an active rollout, a process will confirm that it at least meets the allowed version minimum. if it does not, it will proceed to upgrade to at least the suggested_version if it is not part of a rollout.
* If a rollout is active, it will hash its own node id with the rollout seed and compare that hash to the rollout cursor. if it sorts less then the rollout cursor it should upgrade to the rollout target version
* Also, we need to make sure to add jitter. see http://highscalability.com/blog/2012/4/17/youtube-strategy-adding-jitter-isnt-a-bug.html. having every process restart and sleep 12 hours is a definite way to kill ourselves without adding some randomness back in.

## Implementation

* Create the automatic updater service package.
    * Main process runs on an interval loop.
* Determine which libraries will be used for the binary downloading
    * https://github.com/rhysd/go-github-selfupdate
    * Vet dependencies
* Implement rhysd/go-github-selfupdate 

### When downloading a new binary

Possible Errors:
* download fails
* out-of-space for downloading
* filesystem read-only
* Man in the Middle attacks/corrupted binary
    * Verify binary hashes with message from version server and with output of a hashing algorithm (shasum256)

### When swapping in a new binary

Possible Errors:
* computer crashes during swapping
    * automatic updater checks binary version and reruns download/swap steps.
* deletion/stopping of the old binary fails.
    
### When starting a new binary

Possible Errors:
* out-of-space during migrations
* failure to start
* not yet configured
    * storage node will run a setup or describe out to fix the problem...?

### When performing a gradual rollouts

Possible Errors:
* bad gradual rollout. We know it's a bad rollout if our application stops working
    * Log error in log file if possible
    * if we did have a database migration, api/grpc change, or file system change in the latest update then wait for next update???
        * bad latest update might have had faulty database changes that will need to be migrated again.
    * otherwise rollback to previous version
 
## Open issues (if applicable)

* Do we want to rollback if an update fails?
    * If yes then we need to make backwards migrations for every forward migration.
    * Otherwise we can wait until the version server requests a new update.
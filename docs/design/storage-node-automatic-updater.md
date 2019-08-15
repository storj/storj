# Storage Node Automatic Updater

## Abstract

Automatic Updater is a process that downloads the latest Storage Node binary and replaces the currently running one.

## Background

As more Storage Node Operators join the network we not keep their nodes up to date.
If a Storage Node doesn't meet the minimum version required by the satellites they will no longer be able to offer services to the network.
Currently we are using Docker for updates, but due to it's limitations with certain OS-s we need a better solution.

The Updater has several responsibilities:

1. figure out whether something needs to be updated with gradual rollout,
2. safely download the binaries,
3. safely update the binaries,
4. safely restart the binaries

## Design

The Updater has several steps it takes, contact version server, download, update, restart.

### Checking for updates

Update check will regularly, with jitter, contact Version Server, which responds with a message:

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

When there is a newer version is available it needs to calculate whether it needs to update. To check whether rollout has reached this node it needs to calculate `hash(rollout_seed, node_id) < rollout_cursor`. This exact behavior may differ for canary nodes, which always get the latest version.

* The update check must verify that it is a trusted server.
* The update check should have a jitter to avoid a stampeding herd. See http://highscalability.com/blog/2012/4/17/youtube-strategy-adding-jitter-isnt-a-bug.html for more information.

Possible problems:
* bad gradual rollout. We know it's a bad rollout if our application stops working
    * Log error in log file if possible
    * if we did have a database migration, api/grpc change, or file system change in the latest update then wait for next update???
        * bad latest update might have had faulty database changes that will need to be migrated again.
    * otherwise rollback to previous version

### Downloading the binaries

Once we have decided on a new version we need to download the new version. We will download the appropriate release from a trusted server beside the current binary (instead of temporary directory).

Once we have successfully downloaded we must verify that the binary signature is valid.

* The downloaded file may be quarantined by the anti-virus or blocked by the firewall.

Possible problems:
* downloading could fail
* out-of-space for downloading
* filesystem read-only
* corrupted binary
* Man in the Middle attacks
    * verify binary hashes and the binary signature

### Updating the binaries

To update the binaries we can take two approaches. 

1. Rename `storagenode.exe` into `storagenode.old.<release>.exe`
2. Rename `storagenode.<release>.exe` into `storagenode.exe`.
3. Restart the service using Windows API.

Alternatively this could be:

1. Stop the service using Windows API.
2. Rename `storagenode.exe` into `storagenode.old.<release>.exe`
3. Rename `storagenode.<release>.exe` into `storagenode.exe`.
4. Start the service using Windows API.

Usually automatic updaters prefer the first approach because it allows for inplace updating of the same binary that is doing the updating.

Possible problems:
* computer crashes during swapping
    * automatic updater checks binary version and reruns download/swap steps.
* deletion/stopping of the old binary fails.
* out-of-space during migrations
* failure to start
* not yet configured
    * storage node will run a setup or describe out to fix the problem...?
* anti-virus or other protection prevents new binary from starting

If the service fails to start then we should try to report and/or correct the issue.

## Implementation

* Create an automatic updater.
* Update version server to contain rollout information.
* Make automatic updater part of installer.
* Write document how to start and stop rollouts.
* Test the system with anti-viruses and firewalls.

## Open issues (if applicable)

* Do we want to rollback if an update fails?
    * If yes then we need to make backwards migrations for every forward migration.
    * Otherwise we can wait until the version server requests a new update.

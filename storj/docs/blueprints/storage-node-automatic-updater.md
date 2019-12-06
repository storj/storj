# Storage Node Automatic Updater

## Abstract

Automatic Updater is a process that downloads the latest Storage Node binary and replaces the currently running one.

## Background

As more Storage Node Operators join the network we not keep their nodes up to date.
If a Storage Node doesn't meet the minimum version required by the satellites they will no longer be able to offer services to the network.
Currently we are using Docker for updates, but due to it's limitations with certain OS-s we need a better solution.

The Updater has several responsibilities:

1. Figure out whether something needs to be updated with gradual rollout.
1. Safely download the binaries.
1. Safely update the binaries.
1. Safely restart the binaries.

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
        "rollout_cursor": "40"
      }
    }
  }
}
```

When there is a newer version is available it needs to calculate whether it needs to update. To check whether rollout has reached this node it needs to calculate `hash(rollout_seed, node_id) < rollout_cursor`. This exact behavior may differ for canary nodes, which always get the latest version.

* The update check must verify that it is a trusted server.
* The update check should have a jitter to avoid a stampeding herd. See http://highscalability.com/blog/2012/4/17/youtube-strategy-adding-jitter-isnt-a-bug.html for more information.

### Canary releases

Canary nodes are the first storage nodes that will receive a new release. They will receive it even before first nodes in the gradual rollout. Canary nodes are on the front line on the risk of receiving a bad new release. Only after confirming that canary nodes are behaving correctly after the update, the gradual rollout will begin.

It shall be possible for Storage Node Operator to opt-in to the canary release channel. We expect that those will be mostly Storj employees and enthusiastic community members.

Canary nodes will query the Version Server on a different designated URL to find the latest canary version.

### Downloading the binaries

Once we have decided on a new version we need to download the new version. We will download the appropriate release from a trusted server beside the current binary (instead of temporary directory).

Once we have successfully downloaded we must verify that the binary signature is valid.

Possible problems:
* Downloading could fail.
* Out-of-space for downloading.
* Filesystem is read-only.
* Corrupted binary.
* Man in the Middle attacks.
    * Verify binary hashes and the binary signature.
* Downloaded file may be quarantined by the anti-virus or blocked by the firewall.

### Updating the binaries

To update the binaries we can take two approaches. 

1. Rename `storagenode.exe` into `storagenode.old.<release>.exe`.
1. Rename `storagenode.<release>.exe` into `storagenode.exe`.
1. Restart the service using Windows API.
1. Delete `storagenode.old.<release>.exe`.


Alternatively this could be:

1. Stop the service using Windows API.
1. Rename `storagenode.exe` into `storagenode.old.<release>.exe`
1. Rename `storagenode.<release>.exe` into `storagenode.exe`.
1. Start the service using Windows API.
1. Delete `storagenode.old.<release>.exe`.

Usually automatic updaters prefer the first approach because it allows for inplace updating of the same binary that is doing the updating.

Possible problems:
* Computer crashes during swapping.
    * Automatic updater checks binary version and reruns download/swap steps.
* Deletion/stopping of the old binary fails.
* Out-of-space during migrations.
* Failure to start.
* Not yet configured.
    * Storage node will run a setup or describe out to fix the problem...?
* Anti-virus or other protection prevents new binary from starting.

If the service fails to start then we should try to report and/or correct the issue.

### Rollbacks

There will be cases when, despite our best efforts, we will release a bad version. In such case, storage nodes which got the update will malfunction.

We won't support rolling back to the previous version.

To mitigate the risk, we will have canary releases and gradual release rollout. If a problem with the new version is identified, we will:

1. Stop the canary releases - set the canary channel to the previous last known good version.
1. Stop the gradual release rollout.
1. Prepare a new patch version with the fix.
1. Update the canary channel to the new patch version.
1. If canary nodes report successful fix, restart the gradual release rollout with the new patch version.

## Implementation

Initially, we can implement a basic version of the auto-updater that matches the docker watchtower in features, so we can start supporting Windows Home sooner than waiting to implement all features described in this document.

Basic auto-updater:
* Update the version server to return suggested version for storage nodes.
* Create an automatic updater that just checks the suggested version and updates the binary.
* Make automatic updater part of installer.
* Test the system with anti-viruses and firewalls.

Later we can add:
* Canary releases
  * Update the version server with a canary release channel.
  * Update the auto-updater with a feature to opt-in to the canary release channel.
* Rollout updates
  * Update the version server to include rollout information.
  * Update the auto-updater to check the rollout information.
  * Write document how to start and stop rollouts.

## Open issues

* Should we try to update new and small nodes first to further mitigate the impact of bad releases?
  * Storage Node Operators of new nodes are expected to check their logs more frequently.
  * Is this possible at all using the jitter?
* We need to define the Web API of the Version Server.

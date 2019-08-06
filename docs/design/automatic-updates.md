# Storage Node Automatic Updates

## Overview

As more Storage Node Operators join the network we must ensure these nodes have a mechanism to automatically keep their nodes software up to date.
If a Storage Node Operator fails to keep their node up to date with the minimum version required by satellites they will no longer be selected for upload or download requests.

Currently we are using Docker for automatic updates which means we require Docker to be installed.
Docker is being used for many other things as well, so we need to cover these cases as well.

## Goals

Docker is being used for:

* Automatic Updates (with watchtower)
* Restarting on crash
* Logging (kind of)

To deploy the automatic updates we need to handle these cases.

We must ensure the automatic update system we build into the Storage nodes works on all OS we support. We also need to ensure that it plays nicely with all the common anti-viruses and firewalls.

### Automatic Updates

Finding out the minimum version and latest stable.

What if:
* firewall blocks it.

Download a new binary.
What if:
* anti-virus / firewall blocks it,
* update fails,
* out-of-space for downloading,
* filesystem read-only,
* MITM attacks.

Swapping in a new binary. 
What if:
* anti-virus blocks it,
* filesystem read-only,
* computer crashes during swapping,
* deletion/stopping of the old binary fails.

Starting a new binary.
What if:
* out-of-space during migrations,
* failure to start.

Gradual Rollouts.
What if:
* bad gradual rollout.

Rollbacks for local update failures.

Stampeding herd for updates and crashes.

Notifying the user about an update.

Turning off automatic-updates.

### Starting Storage Node binary on OS Start-Up

We need to ensure that storage node binary starts on computer start-up, without logging into the system.

### Restarting Storage Node binary on Crash / Problems

We need to ensure that storage node binary restarts after a crash.

How do we:
* detect crashes?
* detect unresponsiveness?

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

## Design

### Windows

UWP maybe?

Install Updater / Watchdog Service as with sufficient privileges.

#### Testing

With top used 2 anti-virus & firewalls.

### Linux

### OS X

## Design Overview

When starting a Storage Node:
* Main process starts an updater service which starts a (12 hour?) interval loop
* Updater service spawns a Storage Node process 
* Updater service Interval checks the current Storage Node version and compares with the version server ("https://version.alpha.storj.io/") 
* Updater service Downloads the Binary for the minimum version returned from the version server
* Updater service sends a message through a channel to Storage Node process to kill it
* Updater service spawns a new Storage Node process

## Implementation Milestones

* Determine which libraries will be used for the binary downloading
  * https://github.com/rhysd/go-github-selfupdate
* Vet dependencies
* Create and start an updater service that runs on an interval loop
* Implement rhysd/go-github-selfupdate 
* Tests
  
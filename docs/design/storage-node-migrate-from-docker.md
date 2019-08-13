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
* Watchdog process (binary)
    * Monitors storage node by periodically sending messages to pulse endpoint on storage node and waiting for responses.
    * Restarts the storage node if a crash/unresponsiveness is detected.
* Rotating Logger
    * Log to disk.
    * Rotate files.
    * Compress old stuff.
    * Delete really old stuff.
    * Ensure we limit the size of logs...

## Testing

Verify program can be run with windows defender firewall and at least one other 3rd party firewall.

## Design Overview

General
* Windows firewall and other 3rd party firewalls can block storage node operations.
    * [isportallowed](https://docs.microsoft.com/en-us/windows/win32/api/netfw/nf-netfw-inetfwmgr-isportallowed) windows api function can be used to make sure we are allowed through firewall
    * Can we add code to detect if firewall is blocking storage node operations?
    * Unblock storage node operator in firewall settings.
    * Can we detect if windows firewall is running?

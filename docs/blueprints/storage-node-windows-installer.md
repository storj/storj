# Storage Node Installer for Windows

## Abstract

This design doc outlines how we setup Storage Node on Windows.

## Background

Currently we are using Docker to maintain Storage Nodes.
Unfortunately Docker is not supported for Windows Home, which is popular OS among Storage Node Operators.
Similarly, Docker ends up using more resources than running natively on Windows.

Therefore, it would be nice to be able to run Storage Node without Docker.

For an easy setup process we would need an installer that:
1. Sets up Storage Node to run in background.
1. Sets up automatic updates.
1. Ensures we log everything properly.

We need to take care that we:
* Avoid triggering UAC unnecessarily.
* Start storage node before user login.
* Restart on crashes.

## Design

The high-level idea is to create an installer using the [WiX toolset](https://wixtoolset.org) and make Storage Node a Windows service.

The installer shall:
* Display a GUI for collecting user configuration about:
  * Wallet address
  * Email
  * External address/port
  * Advertised bandwidth 
  * Advertised storage
  * Identity directory
  * Installation directory
  * Storage directory
* Install the storage node binary in the installation directory.
* Generate `config.yaml` file with the user configuration.
* Register the storage node binary as a Windows Service: https://wixtoolset.org/documentation/manual/v3/xsd/util/serviceconfig.html
* Create a firewall exception for port 28967: https://wixtoolset.org/documentation/manual/v3/xsd/firewall/firewallexception.html
* Create shortcut for opening the Dashboard
* Add installation directory to user PATH (optional)
* Open the dashboard when the installation is complete (optional)
* Install the auto-updater binary and run it as a Windows service (if auto-updater is implemented at this point)

We must ensure that both the storage node binary and the MSI installer are signed with Storj code-sign certificate to avoid warning popups to users.

## Rationale

The initial idea was to use [go-msi](https://github.com/mh-cbon/go-msi) for creating the Windows installer. It utilizes the WiX toolset to build the actual installer.

There are a number of reasons to use the WiX toolset directly instead of go-msi:
* More flexibility as we can access all WiX toolset features directly instead of through the go-msi wrapper
* One less dependency
* The go-msi project is stall for 2 years. None of the issues and question has been answered for the last year.
* The go-msi project lacks support for Windows Services and Firewall Rules, although the WiX toolset natively supports them.

## Implementation

1. Implement MSI installer using the WiX toolset.
1. Update the build process to build and sign the MSI installer. 
1. Ensure the MSI installer works properly
   * Install Storage Node using the MSI installer.
   * Ensure binaries run on Windows startup, and it runs as a background process when not logged in.
   * Ensure UAC(s) is not triggered after installation.
   * Ensure that storage node restarts automatically after a crash
   * Verify that service writes to the logs properly

## Open issues

* Consider writing an uninstaller.
  * MSI package support unintsalling too. We must test to check what files are left on disk after uninstall.
* How do we prevent UAC from triggering?
  * Hopefully, code-signing will prevent UAC.
* Consider implementing `ServiceMain` in the storage node binary
  * We should ensure that the storage node process shutdowns gracefully on "Stop Service" and "Restart Service" events. Otherwise, we should handle them in the ServiceMain.
  * See [golang/sys](https://github.com/golang/sys/blob/master/windows/svc/example/service.go) for example.

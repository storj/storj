# Storage Node Installer for Windows

## Abstract

This design doc outlines how we will complete an installation process for a storage node on the Windows operating system. 
We need to ensure that updater binary starts on computer start-up, without logging into the system. 
We will build an msi for installing the necessary files onto the operating system. 
We will set up the installed binary to run as a Windows service.

## Background

Docker is currently used to maintain storage nodes.
Docker however does not run on Windows, yet Windows is a popular operating system among potential storage node operators.
We need to install the storage node onto the user's operating system and configure the operating system to run the storage node as a Windows service. 
We also need to avoid triggering UAC so that the storage node process can run in the background while the user is logged out.

## Design

### MSI
* Install [go-msi](https://github.com/mh-cbon/go-msi)
* Create a wix.json file like [this one](https://github.com/mh-cbon/go-msi/blob/master/wix.json)
* Apply a GUID with `go-msi set-guid`, you must do it once only for each app.
* Run `go-msi make --msi your_program.msi --version 0.0.2`

### service register script
* Modify storage node/automatic updater startup code to implement [ServiceMain](https://docs.microsoft.com/en-us/windows/win32/api/winsvc/nc-winsvc-lpservice_main_functiona).
* See [golang/sys](https://github.com/golang/sys/blob/master/windows/svc/example/service.go)
* [sc.exe](https://docs.microsoft.com/en-us/windows/win32/api/winsvc/nc-winsvc-lpservice_main_functiona) can be used to create a Windows background service.
    * Quotation marks around the binary path, and a space after the `binPath=` are required.
    
## Rationale

We need to ensure we can do the following:
* Limit memory usage
* Limit CPU usage
* Limit the number CPU cores the storage node process can use.
* Storage node binary restarts after a crash.
* Avoid triggering UAC.

Registering the storage node process as a windows service enables the following:
* Send a graceful shutdown message and restart storage node if memory usage is too high.
* Limit the process memory and cpu usage.
* Detect if the storage node process has stopped and restart the storage node process.

## Implementation

1) Modify the storage node startup to implement ServiceMain so that the binary is considered a Windows Service.
2) Modify the automatic updater startup to implement ServiceMain so that the binary is considered a Windows Service.
3) Create script for registering binary as a Windows Background Service.
4) Create wix.json for msi.
5) Create msi.
6) Install Storage Node using msi.
7) Ensure binaries run on Windows startup, and it runs as a background process when not logged in.
8) Ensure UAC is not triggered after installation.
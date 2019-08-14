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
We also need to avoid triggering UAC (user access control) so that the storage node process can run in the background while the user is logged out.

We need to ensure we can do the following:
* Storage node binary restarts after a crash.
* Avoid triggering UAC.

Registering the storage node process as a windows service enables the following:
* Send a graceful shutdown message and restart storage node if memory usage is too high.
* Limit the process memory and cpu usage.
* Detect if the storage node process has stopped and restart the storage node process.

## Design

### MSI (Microsoft Installer Package)
* Install developer dependency [go-msi](https://github.com/mh-cbon/go-msi)
   * Needs to be installed on the build server.
* Create a wix.json file like [this one](https://github.com/mh-cbon/go-msi/blob/master/wix.json)
   * go-msi requires a wix.json file to determine what the installer should do.
   * The MSI must create a desktop shortcut for that opens the dashboard
   ** Automatically open the dashboard at the end of the installation
   * Add the Storage Node binary to the windows USER PATH 
   ** This makes it so that when you open the command line you can run any of the Storage Node commands
   * The wix.json file must include the following user perameters:
   ** Wallet Address
   ** Email
   ** Address/ Port
   ** Bandwidth 
   ** Storge
   ** Identity directory
   ** Storge Directory
* Apply a GUID with `go-msi set-guid`, you must do it once only for each app.
   * This GUID is used to identify the windows process.
* Run `go-msi make --msi your_program.msi --version 0.0.2`
   * This generates the MSI
* MSI must install storage node binary.
* MSI must install Automatic Updater binary.

### Service Register Script
The service register script needs to be registered the storgae node serivces as a windows serivces so that they can be ran in the background.

* Modify storage node/automatic updater startup code to implement [ServiceMain](https://docs.microsoft.com/en-us/windows/win32/api/winsvc/nc-winsvc-lpservice_main_functiona).
   * See [golang/sys](https://github.com/golang/sys/blob/master/windows/svc/example/service.go)
* [sc.exe](https://docs.microsoft.com/en-us/windows/win32/api/winsvc/nc-winsvc-lpservice_main_functiona) can be used to create a Windows background service.
    * Quotation marks around the binary path, and a space after the `binPath=` are required.
    * The installer needs to run this command once the binaires are installed. 

## Implementation

1) Modify the storage node startup to implement ServiceMain so that the binary is considered a Windows Service.
2) Create script for registering binary as a Windows Background Service. (sc.exc command)
3) Create wix.json file for building an MSI.
4) Update the build process to include the creation of the MSI. 
5) Ensure the windows installer is working properly
  * Install Storage Node using msi.
  * Ensure binaries run on Windows startup, and it runs as a background process when not logged in.
  * Ensure UAC(s) is not triggered after installation.
  * Ensure that storage node restarts automatically after a crash 

## Open Issues/ Comments (if applicable)

* Consider writing an uninstaller.
* How do we prevent UAC from triggering?
* Consider using wix without go-mis.
* We need to sign both the binaries and the installer, make sure its done for the MSI.

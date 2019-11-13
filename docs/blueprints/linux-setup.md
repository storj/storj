# Storage node installation and update on Linux

## Abstract

The design document outlines the steps needed to ease installation and provide auto-update of storage nodes on Linux without the use of Docker.


## Background


The idea is to have two main components, like for Windows: the storage node binary and a storage node updater that will take care of updating the storage node binary according to the rollout version mechanism.

The parts we need:
- storagenode as a service
- a system for updating the storagenode binary aka. the updater (with rollout versioning support)
- a system for updating the updater 
- a way to collect the configuration data from the user during the installation
- packaging to ship the above

Linux systems are not unified as windows is. Hence, we have to make choices that will allow us to cover most linux users, with special regards to the raspbian system.

Most commonly used linux distributions seem to be (there are no reliable stats):
- debian-based (Debian, Ubuntu, Mint, Kali, Raspbian)
- Red Hat-based (Fedora)
- Arch Linux (Manjaro is on the rise according to [distrowatch](https://distrowatch.com/dwres.php?resource=popularity)).

All these distributions are shipped with systemD as a service manager.




## Design

- support for debian packaging, dirty way
- well-formed debian package
- convert to RPM (may need some adaptations: for instance there are no debconf-like for RPMs, we will need to implement a post-install script to gather user inputs)
- create the manpage (automatically?)
- When installing the package, it should prompt for user input for:
  - Wallet address
  - Email
  - External address/port
  - Advertised bandwidth
  - Advertised storage
  - Identity directory
  - Installation directory
  - Storage directory
- Generate `config.yaml` file with the user configuration.

## Rationale

### storagenode service
As stated earlier, systemD is the commonly used service manager. It is the default on raspbian, debian, ubuntu, redhat, archlinux.
Hence, we should use systemD for building our storagenode service.

### Installation
#### Custom installer
Packaging in its simplest form would be tar.gz with an installation binary. This solution would be simple for us, but represents an annoyance for the user as our application would not be managed by their package manager.

#### Packages
A package is an archive file containing the application and metadata for indicating to the package manager how to install it. 
Its format depends on the used package manager.
The most common formats are:
- deb for debian-based distributions
- rpm for red hat-based distributions
- .tar.xz for arch

Guidelines about how to build packages are provided for the most commonly used package formats:
- deb: https://go-team.pages.debian.net/packaging.html
- rpm: https://docs.fedoraproject.org/en-US/packaging-guidelines/Golang/
- arch: https://wiki.archlinux.org/index.php/Go_package_guidelines


The process for building a package is as follows:
    - make a source package
    - compile it to get binary packages.
Only the binary package is used by the user for installation. It is not a recommended pratice to directly integrate binaries.

Building the source package is the most difficult part. But once it is done, we can use tools such as [fpm](https://github.com/jordansissel/fpm/wiki) to convert it to other package formats.

We could also use a tool such as [nfpm](https://github.com/goreleaser/nfpm) to build the deb and rpm formats. But we would then be limited to these two formats until nfpm provides others (or develop that part). We could also face difficulties with it as it is still in development and the maintainer warns that some features are not yet available.

To generate the deb package, we could try the [dh-make-golang](https://github.com/Debian/dh-make-golang) tool.

The package could then be distributed:
- by direct download
- from our own repository
- in a user repository if it follows the guidelines

For retrieving user inputs we should use debconf for Debian packaging. There are no debconf-like for rpm, so we would have to implement a post-install script for it.

#### Agnostic Packaging
There are [3 major agnostic packaging system](https://www.ostechnix.com/linux-package-managers-compared-appimage-vs-snap-vs-flatpak/) for linux: AppImage, FlatPak and Snap. As AppImage and FlatPak are more desktop application oriented, we choose to focus on Snap.

##### Snap
[Snaps](https://snapcraft.io/first-snap#go) are containerised software packages. They auto-update daily and work on a variety of Linux distributions. They also revert to the previous version if an update fails. This feature would make it necessary to find out how to implement the rollout versioning.

From the [snap documentation](https://snapcraft.io/docs/go-applications), it seems pretty straightforward to package an application. Snaps are defined in a yaml file. Running an application as a service is done only by specifying "daemon: simple" in the application description. 
This would make us save the work of building a storage node service.

Snaps can then be published in the snapcraft [app store](https://snapcraft.io/). In the store, we would able to monitor the number of installed snaps. It is possible to [host our own store](https://ubuntu.com/blog/howto-host-your-own-snap-store) but that the snap daemon only handles one repository. Therefore, the use of Canonical's store seems mandatory. Snaps integrate well with [github](https://snapcraft.io/build).
A snap inside the store can be published in multiple versions in different [channels](https://snapcraft.io/docs/channels).

Snaps have been known for suffering a long start-up time, but it has been [improved](https://snapcraft.io/blog/snap-startup-time-improvements).

#### Comparison between the package and the snap solutions
Here is a table summarizing the differences between debian packaging system and snaps taken ([source](https://snapcraft.io/blog/a-technical-comparison-between-snaps-and-debs))

| Package	| Debian	| Snap
| --- | --- | ----
| Format	| Ar archive	| SquashFS archive
| Signature verification	| Y (often not used)	| Y
| Package manager	| dpkg (low-level) Different higher-level  managers available	| snap
| Front-end	| Many	| Snap Store
| Installation	| Files copied to /	| Snap uncompressed and mounted as loopback device
| Dependencies	| Shared	| Inside each snap or content snaps
| Automatic updates	| semi-automatic	| Y
| Transactional updates	| N	| Y
| Multiple installs in parallel	| N	| Y
| Multiple versions	| N	| Y
| Security confinement	| Limited	| Y
| Disk footprint	| Smaller	| Larger
| Application startup time	| Default	| Typically longer

In our case, the disk footprint comparison does not stand, as 'go' bundles all dependencies.
For comparison, with a storagenode binary of 85 MB quickly packaged we get:
- snap: 15 MB
- deb: 16.8 MB

We are thinking of using native packaging for the following reasons:
- snap is platform agnostic, but still needs snapd to be installed
- some linux users are reluctant to use snap
- covering deb and rpm packaging would make us cover most used distributions
- with proper packaging, we could directly be included in the distributions

### Updater
The updater could either be a service or a cron job.


### Updater-updater

## Implementation
### Debian package
- Use nfpm to generate a "draft" package. This will be a binary package (not following debian guidelines). Nevertheless, it will allow us to implement and test in parallel the different parts of the packaging solution.
- Implement a systemD service running storagenode binary.
    - https://vincent.bernat.ch/en/blog/2017-systemd-golang
    - https://vincent.bernat.ch/en/blog/2018-systemd-golang-socket-activation
- Implement the storage node updater
- Create the debconf script that will gather user inputs and saves the config.yaml file in the configuration folder.
    - http://www.fifi.org/doc/debconf-doc/tutorial.html
- create the man page

### Repository
- serve the package using repropro
    - https://wiki.debian.org/DebianRepository/SetupWithReprepro
    - or use an already existing docker image (like https://github.com/bbinet/docker-reprepro)

### RPM
- Generate a script to gather user input: https://superuser.com/questions/408852/is-it-possible-to-get-users-input-during-installation-of-rpm

### Tests
- Manual testing:
    - test the installation of the downloaded debian package on different debian-based distributions: debian, ubuntu and raspbian
    - test the installation using a PPA on these configurations.
- Implement tests using debconf unattended installation
    - [debconf(7) man page](https://manpages.debian.org/testing/debconf-doc/debconf.7.en.html)

### Continuous Integration


## Wrapup

[Who will archive the blueprint when completed? What documentation needs to be updated to preserve the relevant information from the blueprint?]

## Open issues

[A discussion of issues relating to this proposal for which the author does not
know the solution. This section may be omitted if there are none.]

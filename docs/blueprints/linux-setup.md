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

All these distributions are shipped with systemd as a service manager.




## Design
The installer will be a debian package. We choose to auto-update the binary, even if this will make the package not following debian guidelines
- When installing the package, it should prompt for user input for:
  - Wallet address
  - Email
  - External address/port
  - Advertised storage
  - Identity directory
  - Storage directory
- Generate `config.yaml` file with the user configuration.

The default value for these directories can be defined using the [XDG Base Directory](https://wiki.archlinux.org/index.php/XDG_Base_Directory).

We choose to reuse the storagenode-updater and the recovery mechanism used in windows. They will be daemonized using systemd. The storagenode updater will auto-update. A recovery will be triggered if the updated updater service fails to restart.
We will use debconf to retrieve user data.

The debian package will NOT contain the storagenode and storagenode-updater binaries. They will be downloaded as part of the post-installation script. A separate git repository will be created for holding the debian package.

Once we get a fully working debian package, we can convert it to the RPM format using the fpm tool. There are no debconf-like for RPMs, we will need to implement a post-install script to gather user inputs.

The debian package will be available by direct download and on a APT repository that users can add to their package manager source list. The repository will be managed using reprepro. Each time the repository is modified, it commits the static content to a dedicated git repository.

## Rationale

### storagenode service
As stated earlier, systemd is the commonly used service manager. It is the default on raspbian, debian, ubuntu, redhat, archlinux.
Hence, we should use systemd for building our storagenode service.

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

Only the binary package is used by the user for installation. It is not a recommended practice to directly integrate binaries.

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

## Implementation
### Debian package
- create a storj debian git
- Implement the debian package skeleton using dh-make. It will create a debian directory that contains all the necessary files to generate the package using dpkg-buildpackage. Commit it to the storj debian git.
- Modify the `debian/rules` file to embed a `config.yaml` template file and create a `/var/lib/storj/storagenode/bin` directory (where the binaries will be put).
- Create the script that will check the storagenode and storagenode-updates latest available versions, download them, put them in the `/var/lib/storj/storagenode/bin`
- create the `storj-storagenode` system user in the post-installation script (`debian/postinst`). It should own the `/var/lib/storj/storagenode/bin` directory. It should be able to write the storage directory.
- Implement a systemd service running storagenode binary. It will be installed by calling `dh_installsystemd` in `debian/rules`
    - https://vincent.bernat.ch/en/blog/2017-systemd-golang
    - https://vincent.bernat.ch/en/blog/2018-systemd-golang-socket-activation
- Create the debconf script that will gather user inputs and saves the config.yaml file in the configuration folder.
    - http://www.fifi.org/doc/debconf-doc/tutorial.html
- Check that the node updater runs on Linux; adapt it if needed.
- Implement a service running the storage node updater
- Adapt the recovery mechanism. SystemD has a `OnFailure` directive.
- create the man page
- add a menu entry
    - https://www.debian.org/doc/packaging-manuals/menu.html/ch3.html
- changelog
- write a Linux installation and auto-update contributor guide
- the storagenode and storagenode-updater binaries for Linux should be added to the assets [here](https://github.com/storj/storj/releases/).

### Repository
- Create a dockerfile that serves the package using reprepro
    - https://wiki.debian.org/DebianRepository/SetupWithReprepro

### RPM
- Generate a script to gather user input: https://superuser.com/questions/408852/is-it-possible-to-get-users-input-during-installation-of-rpm

### Tests
- Manual testing:
    - test the installation of the downloaded debian package on different debian-based distributions: debian, ubuntu and raspbian
    - test the installation using a PPA on these configurations.
- Implement tests using debconf unattended installation
    - [debconf(7) man page](https://manpages.debian.org/testing/debconf-doc/debconf.7.en.html)

### Continuous Integration
- Write a Dockerfile that builds the package.
- Adapt the reprepro Dockerfile to commit to a git repository the content of the apt repository.

### Storage Node Docker Image
We still need to support docker images. The Docker image we provide should make use of the debian packaging system, so that they auto-update using the rollout versioning.

## Wrapup
- As a first step and as part of the PoC, the git repository and the debian package skeleton will be created.
- The PoC will create the user and the directories, download a binary (will not check for the latest) and install a basic storagenode systemd service.
- The PoC will also contain first Dockerfile for the reprepro repository.

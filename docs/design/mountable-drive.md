# Mountable Drive (FUSE Integration)

## Abstract

Goal of this project is to provide utility that will give users ability to interact with Storj network. Mounted directory will behave like any other directory and will give easy access to the network for less experienced users.

## Current Status

The current implementation is not production ready. Its build on the top of `libuplink`. It provides wide set of functionalities but it's poorly tested and requires more work to make it independent from our main code repository. 

Supported platforms:
* Linux (mainly tested)
* OSX (not tested, but should work)
* Windows (not working)

Utility is called `storj-mount` and it's mounting existing bucket to empty directory on file system. With current version `storj-mount` is reusing `uplink` configuration directory.

Example usage:
`storj-mount sj://bucket-with-songs ~/my-music`

## Functions

### Implemented

* copy file from/to mounted bucket
* copy directory from/to mounted bucket
* open file from mounted bucket (supports streaming)
* create a directory (also nested directories)
* get file attributes
* removing files/directories

### Not Implemented

* create file in mounted bucket
* append/modify existing file
* rename file/directory
* move file/directory

### Potential Issues

* Thread safety - file handles may have concurrent reads and writes and internal state should probably be protected by mutexes
* Nonsequential reads and writes - the offset of each request must be checked and handled appropriately. Supporting nonsequential writes is out of scope for this project, and should be handled if an unexpected offset comes in by throwing an error.

## Next steps

* bugfixing/polishing implementation
* automatic tests
* implementing missing functions
* support for Windows
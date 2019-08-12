# Rotating Logger for Storage Node

## Abstract

Storage Node needs a logger that can log to disk, rotate files, compress old files, delete really old files, and ensure a limit on the size of logs.

## Background

Currently log files are agglomerating and are completely unmanaged. Storage nodes need to be able to keep the log files under control to prevent logs from using an exorbitant amount of the storage node's hard drive space.

## Design

Logger Service that runs at storage node startup
* Write files to disk when log function is called.
* Rotate the current file being used for logging by current date and log file size.
* Compress log files older than 3(?) days.
* Delete compressed files older than 30(?) days.
* Limit Log files to 10Kb(?).

We can potentially use [natefinch/lumberjack](https://github.com/natefinch/lumberjack) to maintain/rotate/compress/delete logs

## Implementation

* Create a logger service that runs on storage node startup.
* Write log messages to logger.
* logger service loop rotates logs every day or when the current log file exceeds the log file size limit.

## Open issues (if applicable)

See `(?)` in [Design](##Design)

# Storage Node Satellite Selection

## Abstract

This document details an enhanced method of Satellite selection and maintenance
for Storage Node operators.

## Background.

With the removal of Kademlia, Storage Nodes need a way to identify and select
Satellite's with whom to interact. The implementation of Satellite selection is
currently accomplished via a list of whitelisted Satellite URLs in the
configuration file. The list defaults to well-known satellites hard-coded into
the storage node binary. This method is simple and easy to configure at first
time setup, but unfortunately requires manual maintenance of the list going
forward.

The ideal solution would be just as easy to set up in the common case while
removing the burden of future maintenance.

## Design

The proposed design is to discover trusted Satellites from externally
maintained lists from trusted sources with the ability to manually trust/block
Satellites.

### Concepts

#### Satellite URL

A Satellite URL holds all the information needed to contact and identify a
Satellite. It is comprised of an optional scheme (i.e. `storj://`), an optional
ID, and an address.

The ID can be a full ID or just a prefix. A prefix **SHOULD** be at least 8
characters long in order to contain enough entropy to be useful. The ID is used
to verify the connected peer and **SHOULD** be set to avoid connecting to an
unintended peer.

The address **MUST** contain both a host and port for the purposes of this
feature.

The following are all examples of valid Satellite URLs:

```
us-central-1.tardigrade.io:7777
12EayRS2@us-central-1.tardigrade.io:7777
12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@us-central-1.tardigrade.io:7777
storj://us-central-1.tardigrade.io:7777
storj://12EayRS2@us-central-1.tardigrade.io:7777
storj://12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@us-central-1.tardigrade.io:7777
```

#### Trusted Satellite List

The Trusted Satellite List is a text document where each line represents the
Satellite URL of a trusted Satellite.

Satellite URLs in the Trusted Satellite List **MUST** contain a full ID.

```
12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@us-central-1.tardigrade.io:7777
121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@asia-east-1.tardigrade.io:7777
```

### Storage Node Configuration

The Storage Node configuration for Satellite selection is a list of one or more
entries, where each entry is one of the following:

* Trusted Satellite List URL
* Trusted Satellite URL
* Untrusted Satellite URL

#### Trusted Satellite List URL Entry

This entry contains a URL where a Trusted Satellite List can be downloaded.

Supported schemes are `file://`, `http://` and `https://`.  When
not using `file://` it **SHOULD** be an `https://` URL to ensure transport
security and prevent a person-in-the-middle from tampering with the list.

Examples:

```
https://www.tardigrade.io/trusted-satellites
file:///some/path/to/trusted-satellites.txt
```

#### Trusted Satellite URL Entry

This entry contains the URL to an explicitly trusted Satellite. The format of
the entry is a Satellite URL. The URL **SHOULD** contain an ID. A
partial ID **MAY** be used if it simplifies UX.

#### Untrusted Satellite URL Entry

This entry contains the URL to an explicitly untrusted Satellite. The format of
the entry is a Satellite URL prefixed with a `!`. Since the ID portion of the
Satellite URL is ignored for blocking purposes, the blocked Satellite URL
**SHOULD** contain just the address, like so:

```
!us-central-1.tardigrade.io:7777
```

### Building the List of Trusted Satellite URLs

To build the list of trusted Satellite URLs each entry in the configuration is
traversed in order, parsed, and processed accordingly:

1. If the entry begins with a `!`, it represents an untrusted Satellite URL entry. It is aggregated into the `untrusted` list, which is used later.
1. If the entry begins with `file://`, `http://`, or `https://`, it is a Trusted Satellite List URL. The URL is used to fetch a list of Trusted Satellite URLs that are aggregated into the `trusted` list.
1. If the entry begins with `storj://`, or otherwise does not use a scheme, it is a trusted Satellite URL entry. It is aggregated into the `trusted` list.
1. If an entry does not match any of the above, it is a configuration error.

If a Trusted Satellite List cannot be fetched a warning should be logged. If
available, the last known copy from the Trusted Satellite List URL should be
used. Storage Nodes should attempt to persist the downloaded lists. If they
cannot, a warning should be logged.

When aggregating Satellite URLs, the following rules **MUST** be followed:

* A Satellite URL in the `trusted list` is considered _authoritative_ if it
  matches either of the following criteria:
    * Configured via a Trusted Satellite URL entry
    * Configured via a Trusted Satellite List and has a matching DNS root or
      IP address with the URL used to download the list.
* Satellite URL _IDs_ are equivalent if they are equal, or one is a prefix of the
  other (including an empty ID).
* Satellite URLs are equivalent if the address portions are equal.
* When aggregating equivalent Satellite URLs with _equivalent IDs_, the Satellite
  URL with the longer ID is preferred over one with a shorter (or no) ID.
* When aggregating equivalent Satellite URLs with non-equivalent IDs, the
  _authoritative_ Satellite URL wins. If neither or both are _authoritative_,
  the one aggregated first wins.

After all configuration entries have been processed, each URL in the `trusted`
list is compared to the `untrusted` list. If there is an equivalent Satellite
URL in the `untrusted` list, the Satellite URL is removed from the `trusted
list`. The ID portion of the Satellite URL is ignored for purposes of
comparison with the `untrusted` list.

#### Example

Consider the following Trusted Satellite List URLs and their contents:

* `https://foo.test/trusted-satellites`

```
1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx@a.foo.test:7777
1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx@b.bar.test:7777
1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx@c.baz.test:7777
1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx@d.buz.test:7777
1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx@d.quz.test:7777
```

* `https://bar.test/trusted-satellites`

```
2xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx@a.foo.test:7777
2xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx@b.bar.test:7777
2xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx@c.baz.test:7777
2xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx@d.buz.test:7777
```

Now consider the following configuration:

```
1xxxxxxxx@e.quz.test:7777
https://foo.test/trusted-satellites
0xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx@a.foo.test:7777
0xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx@b.bar.test:7777
https://bar.test/trusted-satellites
!buz.test:7777
```

The following list of trusted Satellite URLs is produced:

```
1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx@e.quz.test:7777
1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx@a.foo.test:7777
0xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx@b.bar.test:7777
1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx@c.baz.test:7777
```

`1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx@e.quz.test:7777` is
selected because even though `1xxxxxxxx@e.quz.test:7777` was _authoritative_
and also came first,
`1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx@e.quz.test:7777` contained
a full ID that was longer than the prefix.

`1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx@a.foo.test:7777` is
selected because `foo.test` is authoritative for this entry and it therefore
cannot be overridden by `bar.test`. It also wins over
`0xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx@a.foo.test:7777` even
though that entry is authoritative since it was aggregated first.

`0xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx@b.bar.test:7777` is
selected because explicit trusted Satellite URL entries are authoritative so it
replaces the `b.bar.test` entry provided by `foo.test`. It also wins over
`2xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx@b.bar.test:7777` even
though that entry also authoritative since it was aggregated first.

`1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx@c.baz.test:7777` wins because
it was aggregated first and no other entry is authoritative.

No `*@d.buz.test:7777` entries are selected because they have been explicitly
blocked.

### Rebuilding the List of Trusted Satellite URLs

The list of trusted Satellite URLs should be recalculated daily (with some jitter).

### Backwards Compatability

The old piecestore configuration (i.e. `piecestore.OldConfig`) currently contains a
comma separated list of trusted Satellite URLs (`WhitelistedSatellites`). It
defaults to the current list of known good satellites. On startup, if the new
configuration is unset, then the old configuration should be used to form
a fixed set of trusted Satellite URLs.

## Open Issues

* How long should storage nodes use cached/persisted lists for? Should lists be persisted at all?
* If aggregation yields no URLs (list URL unreachable) should we default to anything? How should this be reported?
* If block listing removes all URLs, how should this be reported?
* Can we safely auto-migrate storage nodes into this new method of management?
* How long does the storage node wait before garbage collecting pieces from Satellites it no longer trusts? Should this be manual operation?

## To Do

* Implement an endpoint at `https://www.tardigrade.io/trusted-satellites` to return the default list of trusted Satellites.
* Implement a `trust.ListConfig` configuration struct which:
  * Contains the list of entries (with a release default of a single list containing `https://www.tardigrade.io/trusted-satellites`)
  * Contains a refresh interval
  * Maintains backwards compatability with `WhitelistedSatellites` in `piecestore.OldConfig`
* Implement `storj.io/storj/storagenode/trust.List` that:
  * Consumes `trust.ListConfig` for configuration
  * Performs the initial fetching and building of trusted Satellite URLs
  * Updates according to the refresh interval (with jitter)
* Refactor `storj.io/storj/storagenode/trust.Pool` to use `trust.List`

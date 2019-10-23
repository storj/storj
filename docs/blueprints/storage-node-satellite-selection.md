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

The proposed design uses one or more Trusted Satellite Lists combined with
an explicit allow and block list.

### Satellite URL

A Satellite URL holds all the information needed to contact and identify a
Satellite. It is comprised of an optional scheme (i.e. `storj://`), an optional
ID, and an address.

The ID can be a full ID or just a prefix. The ID is used to verify the
connected peer and **SHOULD** be set to avoid connecting to an unintended peer.

The address **MUST** contain both a host and port for the purposes of this feature.

The following are all examples of valid Satellite URLs:

```
us-central-1.tardigrade.io:7777
12EayRS2@us-central-1.tardigrade.io:7777
12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@us-central-1.tardigrade.io:7777
storj://us-central-1.tardigrade.io:7777
storj://12EayRS2@us-central-1.tardigrade.io:7777
storj://12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@us-central-1.tardigrade.io:7777
```

### Trusted Satellite List

The Trusted Satellite List is a text document where each line represents the
Satellite URL of a trusted Satellite, like so:

```
12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@us-central-1.tardigrade.io:7777
121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@asia-east-1.tardigrade.io:7777
```

### Trusted Satellite List URLs

A Trusted Satellite List URL is a URL where a Trusted Satellite List
can be downloaded. It **SHOULD** be an HTTPS URL to ensure transport security
and prevent a person-in-the-middle from tampering with the list.

For example:

```
https://www.tardigrade.io/trusted-satellites
```

### Allow List

The Allow List contains Satellite URLS of explicitly trusted Satellites.

### Block List

The Block List contains Satellite URLs of explicitly blocked Satellites.

### Building the List of Trusted Satellite URLs

To build the list of trusted Satellite URLs, the following steps are performed,
in order:

1. Trusted Satellite Lists are downloaded from all Trusted Satellite List URLs and aggregated _in the order_ they are specified in the configuration.
1. URLs from the Allow List are then aggregated into the trusted list.
1. URLs in the Block List are removed from the trust list, if present. URLs
   in the Block list that are not present in the trust list are ignored.

If a Trusted Satellite List cannot be fetched a warning should be logged. If
available, the last known copy from the Trusted Satellite List URL should be
used. Storage Nodes should persist the downloaded lists.

When aggregating Satellite URLs, the following rules **MUST** be followed:

* Satellite URL IDs are equivalent if they are equal, or one is a prefix of the
  other (including an empty ID).
* Satellite URLs are equivalent if the address portions are equal.
* When aggregating equivalent Satellite URLs with equivalent IDs, the Satellite
  URL with the longer ID is preferred over one with a shorter (or no) ID.
* When aggregating equivalent Satellite URLs with non-equivalent IDs, the new
  URL wins.
* When blocking URLs, ID equivalence is ignored. In other words, URLs in the
  trust list are equivalent with one in the Block List, they are removed.

#### Examples

##### Longest ID Preferred

List A:

```
abcd@foo.com:7777
foo.com:7777
```

List B:

```
ab@foo.com:7777
```

Results in:

```
abcd@foo.com:7777
```

##### Last URL Wins When ID is not Equivalent

List A:

```
abcd@foo.com:7777
```

List B:

```
ef@foo.com:7777
```

Results in:

```
ef@foo.com:7777
```

##### URL in the Block List

List A:

```
abcd@foo.com:7777
```

Block List:

```
foo.com:7777
```

Results in:

```

```

### Refreshing the List

The list of trusted Satellite URLs should be refreshed daily (with some jitter).

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

## To Do

* Implement an endpoint at `https://www.tardigrade.io/trusted-satellites` to return the default list of trusted Satellites.
* Implement a `trust.ListConfig` configuration struct which:
  * Contains a list of Trusted Satellite List URLs (with a release default of `https://www.tardigrade.io/trusted-satellites`)
  * Contains the Allow List
  * Contains the Block List
  * Contains a refresh interval
  * Maintains backwards compatability with `WhitelistedSatellites` in `piecestore.OldConfig`
* Implement `storj.io/storj/storagenode/trust.List` that:
  * Consumes `trust.ListConfig` for configuration
  * Performs the initial fetching and aggregation of trusted Satellite URLs
  * Updates according to the refresh interval (with jitter)
* Refactor `storj.io/storj/storagenode/trust.Pool` to use `trust.List`

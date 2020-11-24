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
Satellite. It is comprised of an optional scheme (i.e. `storj://`), an ID, and
an address.

The address **MUST** contain both a host and port for the purposes of this
feature.

The following are all examples of valid Satellite URLs:

```
12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@us-central-1.tardigrade.io:7777
storj://12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@us-central-1.tardigrade.io:7777
```

The following are invalid Satellite URLs (missing or partial IDs):

```
us-central-1.tardigrade.io:7777
12EayRS2@us-central-1.tardigrade.io:7777
storj://us-central-1.tardigrade.io:7777
storj://12EayRS2@us-central-1.tardigrade.io:7777
```

#### Trusted Satellite List

The Trusted Satellite List is a text document where each line represents the
Satellite URL of a trusted Satellite.

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

Supported schemes are `file://`, `http://` and `https://`.  When using HTTP an
`https://` URL should be preferred over an `http://` URL to ensure transport
security and prevent a person-in-the-middle from tampering with the list.

Examples:

```
https://www.tardigrade.io/trusted-satellites
file:///some/path/to/trusted-satellites.txt
```

#### Trusted Satellite URL Entry

This entry contains the URL to an explicitly trusted Satellite. The format of
the entry is a Satellite URL.

#### Untrusted Satellite Entry

This entry contains the URL to an explicitly untrusted Satellite. The format of
the entry is a `!` followed by one of the following:

* Satellite ID followed by an `@` (to distinguish it from a host)

```
!121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@
```

* Satellite host. If the host is a domain, then subdomains are also untrusted
  (i.e. `!tardigrade.io` will block `us-central-1.tardigrade.io`)

```
!tardigrade.io
!us-central-1.tardigrade.io
!us-east-1.tardigrade.io
```

* Satellite URL

```
!121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@us-central-1.tardigrade.io:7777
```

### Building the List of Trusted Satellite URLs

To build the list of trusted Satellite URLs each entry in the configuration is
traversed in order, parsed, and processed accordingly:

1. If the entry begins with a `!`, it represents an untrusted Satellite entry. It is added to the `untrusted` list, which is used later.
1. If the entry begins with `file://`, `http://`, or `https://`, it is a Trusted Satellite List URL. The URL is used to fetch a list of Trusted Satellite URLs that are added into the `trusted` list.
1. If the entry begins with `storj://`, or otherwise does not use a scheme, it is a trusted Satellite URL entry. It is added into the `trusted` list.
1. If an entry does not match any of the above, it is a configuration error.

If a Trusted Satellite List cannot be fetched a warning should be logged. If
available, the last known copy from the Trusted Satellite List URL should be
used. Storage Nodes should attempt to persist the downloaded lists. If they
cannot, a warning should be logged.

After all configuration entries have been processed, each URL in the `trusted`
list is checked against the `untrusted` list and removed if it matches an entry
. An `untrusted` entry matches a URL using the following criteria:

* When the `untrusted` entry is just a Satellite ID, it matches any URL with
  that ID.
* When the `untrusted` entry is just a host, it matches any URL with the same
  host. If the host is a domain name, then the entry also matches URLs within a
  subdomain of that host.
* When the `untrusted` entry is a full Satellite URL, it matches any URL that
  is equal.

After the `trusted` list has been pruned, the remaining URLs are aggregated
according to the following rules:

* A Satellite URL is considered _authoritative_ if it matches either of the
  following criteria:
    * Configured via a Trusted Satellite URL entry
    * Configured via a `file://` URL
    * Configured via an `https://` or `http://` Trusted Satellite List URL AND matches the domain or is a subdomain of the domain name in the Trusted Satellite List URL.
* Satellite URLs are equivalent if the address portions are equal.
* When aggregating equivalent Satellite URLs (i.e. address matches) with
  differing IDs, the _authoritative_ Satellite URL wins. If neither or both are
  _authoritative_, the one aggregated first wins.


#### Example

Consider the following Trusted Satellite List URLs and their contents. For
brevity sake, the full ID of each URL is being shortened (real configurations
**MUST** specify the full ID).

* `file:///path/to/some/trusted-satellites.txt`

```
1@bar.test:7777
```

* `https://foo.test/trusted-satellites`

```
2@f.foo.test:7777
2@buz.test:7777
2@qiz.test:7777
5@ohno.test:7777
```

* `https://bar.test/trusted-satellites`

```
3@f.foo.test:7777
3@bar.test:7777
3@baz.test:7777
3@buz.test:7777
3@quz.test:7777
```

* `https://baz.test/trusted-satellites`

```
4@baz.test:7777
4@qiz.test:7777
4@subdomain.quz.test:7777
```

Now consider the following configuration:

```
- !quz.test
- file:///path/to/some/trusted-satellites.txt
- https://foo.test/trusted-satellites
- https://bar.test/trusted-satellites
- https://baz.test/trusted-satellites
- 0@f.foo.test:7777
- !2@qiz.test:7777
- !5
```

After expanding each entry, we have the following unaggregated `trusted` list:

```
1@bar.test:7777                   (authoritative due to file:// URL)
2@f.foo.test:7777                 (authoritative due to foo.test domain)
2@buz.test:7777
2@qiz.test:7777
5@ohno.test:7777
3@f.foo.test:7777
3@bar.test:7777                   (authoritative due to bar.test domain)
3@baz.test:7777
3@buz.test:7777
3@quz.test:7777
4@baz.test:7777                   (authoritative due to baz.test domain)
4@qiz.test:7777
4@subdomain.quz.test:7777
0@f.foo.test:7777                 (authoritative due to explicit configuration)
```

And the following `untrusted` list:

```
quz.test
2@qiz.test:7777
5@
```

The `trusted` list is pruned with the `untrusted` list, leaving the following `trusted` list:

```
1@bar.test:7777                   (authoritative due to file:// URL)
2@f.foo.test:7777                 (authoritative due to foo.test domain)
2@buz.test:7777
3@f.foo.test:7777
3@bar.test:7777                   (authoritative due to bar.test domain)
3@baz.test:7777
3@buz.test:7777
4@baz.test:7777                   (authoritative due to baz.test domain)
4@qiz.test:7777
0@f.foo.test:7777                 (authoritative due to explicit configuration)
```

We aggregate from top to bottom (i.e. in the order they were specified/fetched)
and are left with the following:

```
1@bar.test:7777
2@f.foo.test:7777
2@buz.test:7777
4@baz.test:7777
4@qiz.test:7777
```

* `1@bar.test:7777` was selected because even though `3@bar.test:7777` was also
  authoritative, `1@bar.test:7777` came first.
* `2@f.foo.test:7777` was selected because it was authoritative over
  `3@f.foo.test:7777` and came before `0@f.foo.test`.
* `2@buz.test:7777` was selected because it came before `3@buz.test:7777` (neither was authoritative)
* `4@baz.test:7777` was selected it was authoritative over `3@baz.test:7777`, even though the latter came first.
* `4@qiz.test:7777` was selected because it was the only URL for `qiz.test:7777`

### Rebuilding the List of Trusted Satellite URLs

The list of trusted Satellite URLs should be recalculated daily (with some jitter).

### Backwards Compatibility

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
  * Maintains backwards compatibility with `WhitelistedSatellites` in `piecestore.OldConfig`
* Implement `storj.io/storj/storagenode/trust.List` that:
  * Consumes `trust.ListConfig` for configuration
  * Performs the initial fetching and building of trusted Satellite URLs
  * Updates according to the refresh interval (with jitter)
* Refactor `storj.io/storj/storagenode/trust.Pool` to use `trust.List`

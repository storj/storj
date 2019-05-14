# Title: Uplink Contexts

## Abstract

This design document proposes the data we need to store inside of a share in order to access files. Currently, the uplink configuration yaml file includes distinct entries for the API key, the satellite url, and the root encryption key. This proposal is to bundle an API key, a satellite url, and a list of revealed paths into a context (much like a kubectl context) and call that bundle a share.

## Background

In order to access objects, one must provide an API key to a given satellite. This API key is tied to a project which allows the satellite to look up the specific bucket being requested and do the things it needs to do. API keys are moving to be macaroons where they can be limited to only allow reads or writes, or expire, etc. Those features can easily be implemented and don't interact much the overall uplink command experience: you have access to do some actions or you don't.

There are complications when it comes to restricting to a whitelist of buckets and/or path prefixes inside of those buckets. For example, if a macaroon is limited to only allow access to paths beneath `sj://bucket/a/b`, then it would be impossible to list the root. Additionally, the clients and servers all operate on _encrypted_ paths, making it impossible for the server to issue the list and filter, because one would have to reveal the root encryption key for the bucket for the client to be able to decrypt the names.

One could attempt to cause restricted macaroons to be "mounted". For example, if a bucket contains `sj://bucket/a/b/c` then the listing of the root would return "c". Unfortunately, this does not compose well with having multiple allowed paths in a macaroon: which "root" do you mean? It also causes confusion because the paths of objects inside of a bucket becomes a local rather than global property, even though the clients and servers must speak with the global view at all times.

Additionally, there's a single global key per project. That's less than ideal for a situation like having distinct prod and staging buckets: you don't want them both using the same encryption keys so that giving access to staging does not leak the encryption keys for prod. It also doesn't allow one to re-key a bucket if a share is leaked. Even if the macaroon inside is revoked, it may be seen as a risk that the key was leaked because they are now depending on the satellite operator for the safety of their data.

## Design

Uplink will operate on a set of "contexts". Each context contains an API key, a satellite url, and a list of bucket shares. In Go,

```go
type Context struct {
    APIKey        string // Or perhaps uplink.APIKey
    SatelliteURL  string
    BucketShares  []BucketShare
}
```

A bucket share contains an encrypted path, the matching unencrypted path, the key beginning at that path, and the bucket the path is contained in. In Go,

```go
type BucketShare struct {
    Bucket          string
    EncryptedPath   string
    UnencryptedPath string
    EncryptionKey   []byte
}
```

<blockquote>
For concreteness, the following is an example UX around uplink with contexts. It is expected to evolve, and is just used to help with understanding.
</blockquote>

When setting up uplink for the first time, a default context is created, and the command will prompt you for some credentials. For example,

```
$ uplink setup
A new "default" context has been created.

Please respond with the type of information you have:
1. API key from a satellite
2. Exported context
> 1

API Key: <user pastes key>
Satellite URL: <user puts in or picks satellite like now>
```

After a context has been created, bucket information must be added. For example,

```
$ uplink import [optionally specify a context with --context=default]

Please respond with the type of information you have:
1. Bucket with root passphrase
2. Bucket share
> 1

Bucket name: <user inputs bucket name>
Passphrase: <user inputs passphrase>

$ uplink import
Please respond with the type of information you have:restricted
1. Bucket with root passphrase
2. Bucket share
> 2

Bucket share: <user pastes shared bucket (RevealedPath)>
```

When a client attempts to interact with a bucket, it finds the latest added matching unencrypted path and appends the rest of the path, encrypted with the key the revealed path specifies, to the revealed path's encrypted path. For example, if one had this set of `BucketShare` objects:

```go
paths := []BucketShare{
    {Bucket: "x", EncryptedPath: "/e1/e2/e3", UnecryptedPath: "/a/b/c", EncryptionKey: "key1"},
    {Bucket: "x", EncryptedPath: "/e1",       UnecryptedPath: "/a",     EncryptionKey: "key2"},
    {Bucket: "x", EncryptedPath: "/e4",       UnecryptedPath: "/f",     EncryptionKey: "key3"},
}
```

Then if one asked for `sj://x/a/d`, the 2nd path would be the longest match and `"key2"` would be used to encrypt `d` and it would be appened to `"/e1"`. Similarly, if one asked for `sj://x/a/b/c/f`, then `"key2"` would be used to encrypt `b/c/f` and it would be appened to `"/e1"`. If one asked for `sj://x/f/g`, then `"key3"` would be used. `"key1"` would never be used as it is subsumed by the entry with `"key2"`.

## Rationale

This proposal is not a radical departure of the current method, just an extension to allowing a collection of paths to compose a bucket at any unencrypted spot. Thus, it shoudn't require any changes on the server side, reducing risk and allowing for earlier shipping.

We could allow for "longest" matching. For example, in the above set of revealed paths one could use `"key1"` for `sj://x/a/b/c/d`. This begs a question about consistency. In principle be able to derive `"key1"` from the information provided by `"key2"`. If they disagree, an error could be thrown. This design takes the position that, these issues are trivial because it is very unlikely that such a share would be constructed, and even if it were, it would disagree.

The design does mean that if you create a new bucket on the web, you must also create the bucket in your clients. There's no way around this without also having a project level key that all bucket keys are derived from, which makes key revocation for a specific bucket difficult, as the same key must be derived from the project key and bucket name. Since users are expected to manage their own encryption keys, and since creating a bucket from the command line can add the associated bucket share, this extra effort is justified.

## Implementation

In order to implement this, the above structs would be added to the code base and uplink would be changed to be configured with a context. After that, the apis to construct paths and perform lists would have to be changed to consult the bucket shares. The list implementation must synthesize responses when listing a prefix of an encrypted path. For example, if one listed `sj://x`, both `"a"` and `"f"` would be returned without consulting the server. Concurrently with any work after uplink has been changed to be configured with a context, the command can be iterated upon to design the context import, export, management, and share workflows.

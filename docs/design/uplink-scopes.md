# Title: Uplink Scopes

## Abstract

This design document proposes the data we need to store inside of a share in order to access files. Currently, the uplink configuration yaml file includes distinct entries for the API key, the satellite url, and the root encryption key. This proposal is to bundle an API key, a satellite url, and a list of revealed paths into a scope, much like a kubectl context.

## Background

In order to access objects, one must provide an API key to a given satellite. This API key is tied to a project which allows the satellite to look up the specific bucket being requested and do the things it needs to do. API keys are moving to be macaroons where they can be limited to only allow reads or writes, or expire, etc. Those features can easily be implemented and don't interact much the overall uplink command experience: you have access to do some actions or you don't.

There are complications when it comes to restricting to a whitelist of buckets and/or path prefixes inside of those buckets. For example, if a macaroon is limited to only allow access to paths beneath `sj://bucket/a/b`, then it would be impossible to list the root. Additionally, the clients and servers all operate on **encrypted** paths, making it impossible for the server to issue the list and filter, because one would have to reveal the root encryption key for the bucket for the client to be able to decrypt the names.

## Design

Uplink will operate on a set of "scopes". Each scope contains an API key, a satellite url, an optional project key, and a list of bucket shares. In Go,

```go
type Scope struct {
    APIKey               *uplink.APIKey
    SatelliteURL         string
    ProjectEncryptionKey *storj.Key
    BucketShares         []BucketShare
}
```

A bucket share contains an encrypted path, the matching unencrypted path, the key beginning at that path, and the bucket the path is contained in. In Go,

```go
type BucketShare struct {
    Bucket          string
    EncryptedPath   storj.Path
    UnencryptedPath storj.Path
    EncryptionKey   *storj.Key
}
```

If a client attempts to interact with a bucket, and no `BucketShare` matches, and there is a project key set, a key for that bucket is derived using the bucket name and the project key.

When a client attempts to interact with a bucket, it finds the longest matching unencrypted path, breaking ties by using the later one, and appends the rest of the path, encrypted with the key the bucket share specifies, to the bucket share's encrypted path. For example, if one had this set of `BucketShare` objects:

```go
paths := []BucketShare{
    {Bucket: "x", EncryptedPath: "/e1/e2/e3", UnecryptedPath: "/a/b/c", EncryptionKey: "key1"},
    {Bucket: "x", EncryptedPath: "/e1",       UnecryptedPath: "/a",     EncryptionKey: "key2"},
    {Bucket: "x", EncryptedPath: "/e4",       UnecryptedPath: "/f",     EncryptionKey: "key3"},
}
```

Then if one asked for `sj://x/a/d`, the 2nd path would be the longest match and `"key2"` would be used to encrypt `d` and it would be appened to `"/e1"`. Similarly, if one asked for `sj://x/a/b/c/f`, then the 1st path would be the longest match and `"key1"` would be used to encrypt `f` and it would be appened to `"/e1/e2/e3"`. If one asked for `sj://x/f/g`, then `"key3"` would be used. Asking for `sj://x/g` would return not found.

Note that the encrypted path does not need to match the encrypted unencrypted path, and this allows for having local views of bucket paths in the actual bucket. For example, one could store a file a `sj://x/a/b/c` which has encrypted path `sj://x/e1/e2/e3`, but share that with someone else and tell them that the encrypted path is actually the unencrypted path `sj://x/d/e/f`.

## Rationale

This proposal is not a radical departure of the current method, just an extension to allowing a collection of paths to compose a bucket at any unencrypted spot. Thus, it shoudn't require any changes on the server side, reducing risk and allowing for earlier shipping. For example, any current configuration can be represented as a scope with a `ProjectEncryptionKey` specified and no bucket shares.

Longest matching begs a question about consistency. In principle, it's possible to derive `"key1"` from the information provided by `"key2"`. If they disagree, an error could be thrown. Consistency does mean that you can't have differently keyed paths as part of your bucket, which may be useful. Inconsistency means that a bucket or project could contain objects that are invisible or undecryptable by clients. This design makes no requirement for consistency, and chooses to hide and warn about paths that cannot decrypt.

The design does mean that if you create a new bucket on the web, you must also create the bucket in your clients, unless you have the project key, because users would not want their encryption key to leave their computer. Since users are expected to manage their own encryption keys, and since creating a bucket from the command line can add the associated bucket share, this extra effort is justified.

This design calls for explicitly choosing your scope (perhaps a `--scope prod` flag), but that doesn't require that it do that forever. For example, one could automatically choose the scope if the bucket name is unambiguous, and provide more information in the bucket url to allow for unambiguously picking everything (satellite and api key). For example, if you had two scopes where one contained a bucket share for bucket `"x"` and another contained a bucket share for bucket `"y"`, it could unambiguously and automatically choose the correct scope if the user requested `sj://y/foo/bar`, or if it was ambiguous, one could imagine using a url like `sj://scopeName@y/foo/bar` to choose the scope name.

The `BucketShare` struct sadly duplicates the bucket name and unencrypted path that should exist in a well restricted macaroon anyway. One could imagine doing something like having the slice of bucket shares **not** include the unencrypted path and bucket, and just use the order that they appear with the order the restrictions are presented in the macaroon to save space. This is just a question of how the shares should be serialized, though, and so that is out of scope of this design. During unserialization, the full `BucketShare` struct could be filled in.

## Implementation

In order to implement this, the above structs would be added to the code base and uplink would be changed to be configured with a scope. After that, the apis to construct paths and perform lists would have to be changed to consult the bucket shares. The list implementation must synthesize responses when listing a prefix of an encrypted path. For example, if one listed `sj://x`, both `"a"` and `"f"` would be returned without consulting the server. The code to decrypt path entries would have to be audited to ensure that decryption failures become at worst warnings. Concurrently with any work after uplink has been changed to be configured with a scope, the command can be iterated upon to design the scope import, export, management, and share workflows.

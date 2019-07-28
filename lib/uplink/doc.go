// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/*
Package uplink is the main entrypoint to interacting with Storj Labs' decentralized
storage network.

Projects

An (*Uplink) reference lets you open a *Project, which should have already been created via
the web interface of one of the Storj Labs or Tardigrade network Satellites. You may be able
to create or access your us-central-1 account here: https://us-central-1.tardigrade.io/

Opening a *Project requires a specific Satellite address (e.g. "us-central-1.tardigrade.io:7777")
and an API key. The API key will grant specific access to certain operations and resources within
a project. Projects allow you to manage and open Buckets.

Example:

    ul, err := uplink.NewUplink(ctx, nil)
    if err != nil {
        return err
    }
    defer ul.Close()

    p, err := ul.OpenProject(ctx, "us-central-1.tardigrade.io:7777", apiKey)
    if err != nil {
        return err
    }
    defer p.Close()

API Keys

An API key is a "macaroon" (see https://ai.google/research/pubs/pub41892). As such, API keys
can be restricted such that users of the restricted API key only have access to a subset of
what the parent API key allowed. It is possible to restrict a macarron to specific operations,
buckets, paths, path prefixes, or time windows.

If you need a valid API key, please visit your chosen Satellite's web interface.

Example:

    adminKey, err := uplink.ParseAPIKey("13YqeJ3Xk4KHocypZMdQZZqfC1goMvxbYSCWWEjSmew6rVvJp3GCK")
    if err != nil {
        return "", err
    }

    readOnlyKey, err := adminKey.Restrict(macaroon.Caveat{
            DisallowWrites: true,
            DisallowLists: true,
            DisallowDeletes: true,
    })
    if err != nil {
        return "", err
    }

    // return a new restricted key that is read only
    return readOnlyKey.Serialize()

Restricting an API key to a path prefix is most easily accomplished using an
EncryptionAccess, so see EncryptionAccess for more.

Buckets

A bucket represents a collection of objects. You can upload, download, list, and delete objects of
any size or shape. Objects within buckets are represented by keys, where keys can optionally be
listed using the "/" delimiter. Objects are always end-to-end encrypted.

    b, err := p.OpenBucket(ctx, "staging", access)
    if err != nil {
        return err
    }
    defer b.Close()

EncryptionAccess

Where an APIKey controls what resources and operations a Satellite will allow a user to access
and perform, an EncryptionAccess controls what buckets, path prefixes, and objects a user has the
ability to decrypt. An EncryptionAccess is a serializable collection of hierarchically-determined
encryption keys, where by default the key starts at the root.

As an example, the following code creates an encryption access context (and API key) that is
restricted to objects with the prefix "/logs/" inside the staging bucket.

    access := uplink.NewEncryptionAccessWithDefaultKey(defaultKey)
    logServerKey, logServerAccess, err := access.Restrict(
        readOnlyKey, uplink.EncryptionRestriction{
            Bucket: "staging",
            Path: "/logs/",
        })
    if err != nil {
        return "", err
    }
    return logServerAccess.Serialize()

The keys to decrypt data in other buckets or in other path prefixes are not contained in this
new serialized encryption access context. This new encryption access context only provides the
information for just what is necessary.

Objects

Objects support a couple kilobytes of arbitrary key/value metadata, an arbitrary-size primary
data streams, with seeking. If you want to access only a small subrange of the data you
uploaded, you can download only the range of the data you need in a fast and performant way.
This allows you to stream video straight out of the network with little overhead.

    obj, err := b.OpenObject(ctx, "/logs/webserver.log")
    if err != nil {
        return err
    }
    defer obj.Close()

    reader, err := obj.DownloadRange(ctx, 0, -1)
    if err != nil {
        return err
    }
    defer reader.Close()

*/
package uplink

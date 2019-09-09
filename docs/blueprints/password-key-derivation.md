# Title: Password Key Derivation

## Abstract

This design doc describes the way cryptographic encryption keys will be derived from user provided passwords in a way that resists offline attacks and low entropy passwords.

## Background

A unique encryption key exists for every path inside of a bucket, and they are designed in a way that allows for deriving a key from a parent and some path component. For example, if you had the key for `sj://bucket/path1/path2` then you could derive the key for `sj://bucket/path1/path2/path3`.

That leaves open the question for how a root key is created. Some requirements on that root key creation include

1. It's based on the user's choice (for example, a password).
2. If the user inputs the same password on different machines, the same root key should be created.

These requirements allow users to be in full control of their encryption, and don't require users safely transporting high entropy (hard to remember) secrets to bootstrap new uplinks.

This design accomodates more requirements that allow for additional features:

3. A root key can be created for any encrypted path in a bucket, not just the bucket.
4. A table of root keys for low entropy passwords should not be possible. In other words, an attacker with knowledge of the algorithm should not be able to use a dictionary of common passwords and pre-compute what keys to check in the event of a data breach.
5. Users do not have to enter a password for every bucket they create: they can have a default password that is used for every bucket unless otherwise specified.

The third requirement allows having multiple encrypted domains to exist within a single bucket, allowing delegation of encryption. The fourth requirement enhances security if a satellite breach happens. This is usually accomplished with [salting](https://en.wikipedia.org/wiki/Salt_(cryptography)). The fifth requirement allows providing a finite amount of information to a third party that allows them access to the keys for any bucket created with the default password in the future.

## Design

First, a **root key** is defined to be, in the terminology of section 4.11 in the [whitepaper](https://storj.io/storjv3.pdf), the same as s<sub>0</sub>. Any number of root keys can be created for any bucket and (possibly empty) encrypted path. Subsequent secrets are derived using the unencrypted path components and will end up encrypted as described in the whitepaper and appended to the encrypted path used when the root key was created. This means that the root key for a bucket and encrypted path may not match the derived key from a root key for a bucket and empty encrypted path, and then deriving a key using path segments to match that encrypted path.

In other words, `rootKey(bucket, encryptedPath) != deriveKey(rootKey(bucket, ""), encryptedPath)`.

Satellites will allow clients to query for a salt for a project. The result of this query must be stable.

The following algorithm is used to create a root key from an api key, password, and optional encrypted path:

```
projectSalt = getProjectSalt(apiKey)
mixedSalt   = hmac(hash=sha256, secret=userPassword, data=projectSalt)
pathSalt    = hmac(hash=sha256, secret=mixedSalt, data=encryptedPath or "")
rootKey     = argon2id(salt=pathSalt, password=password)
```

Uplink always stores and operates on outputs of the previous algorithms (the root keys), never the input material (passwords, salts, etc).

During the folding of path components into the root key as defined by the whitepaper, if the root key was created with an empty encrypted path, the bucket name is used as the first path component in the fold. This ensures that different keys are output for the same input paths in two different buckets. A default root key is created by using an empty encrypted path.

## Rationale

This design accomplishes all of the requirements listed above.

- Each root key requires some secret that necessarily comes from the user in some way.
- The algorithm is deterministic, so the user gets the same keys on any machine where she inputs the same passwords.
- Root keys for a sub-path in a bucket are accomplished by providing a non-empty encrypted path when creating the root key.
- Because entropy is added as the salt in the `argon2id` step, dictionary attacks on low-entropy passwords cannot be precomputed.
- One can create a default root key by using an empty encrypted path and using that if no other root key applies.

Some other points of consideration include

- Clients can ignore the salt when they know they have a strong password if they wish. Indeed, they can ignore all of this and do their own key management. At a minimum, we allow the user to specify an already-formed root key in their configuration if they don't want one created from a password.
- The salt is not blindly trusted from the satellite, and is mixed with the password in an HMAC step that ensures a hostile satellite cannot predict what the actual salt will be.
- HMAC provides security against forgeries without knowing the secret, which implies that sharing the output of a HMAC does not leak information about the secret, even under attacker provided data, so I believe the usage of HMAC is safe.
- Users will already lose data if they do not back up their data pointers in the satellite (because the encryption key for the data is only stored there), so adding the salts does not add a requirement that users need to back up data in a satellite, it just adds a small amount of additional information they need to back up.

## Implementation

First, the satellite must allow salt information to be queried for projects, and it may use the project UUID to back it.

Next, the clients must be changed to run the above algorithms to create root keys.

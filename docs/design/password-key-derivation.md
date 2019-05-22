# Title: Password Key Derivation

## Abstract

This design doc describes the way cryptographic encryption keys will be derived from user provided passwords in a way that resists offline attacks and low entropy passwords.

## Background

A unique encryption key exists for every path inside of a bucket, and they are designed in a way that allows deriving a key from a parent and some path component. For example, if you had the key for `sj://bucket/path1/path2` then you could derive the key for `sj://bucket/path1/path2/path3`.

That leaves open the question for how a root key is derived. Some requirements on that root key derivation include

1. It's based on the user's choice (for example, a password).
2. If the user inputs the same password on different machines, the same root key should be derived.

These requirements allow users to be in full control of their encryption, and don't require users safely transporting high entropy (hard to remember) secrets to bootstrap new uplinks.

This design accomodates more requirements that allow for additional features:

3. A root key can be derived for any path in a bucket.
4. Buckets root keys should not depend on the name of the bucket.
5. A table of keys for low entropy passwords should not be possible. In other words, an attacker with knowledge of the algorithm should not be able to use a dictionary of common passwords and pre-compute what keys to check in the event of a data breach.
6. There is a default that allows deriving keys for buckets when no bucket specific secret is provided.

The third requirement allows having multiple encrypted domains to exist within a single bucket, allowing delegation of encryption. The fourth requirement allows renaming buckets without having to re-encrypt all of the paths inside of a bucket. The fifth requirement enhances security if a satellite breach happens. This is usually accomplished with [salting](https://en.wikipedia.org/wiki/Salt_(cryptography)). The sixth requirement allows providing a finite amount of information to a third party that allows them access to the keys for any bucket created in the future.

## Design

Uplink always stores and operates on outputs of the following algorithms, never the input material.

Satellites will add a salt field to bucket metadata that lives with the path encryption information. Satellites will also store a salt that lives with the project metadata. The salts will be 32 random, stable bytes.

A default password can be chosen for an api key. The following algorithm is used to derive the default password:

```
saltEntropy     = getProjectSalt(apiKey)
mixedSalt       = hmac(hash=sha256, secret=userPassword, data=saltEntropy)
defaultPassword = argon2id(salt=mixedSalt, password=userPassword)
```

The following algorithm is used to deriving a root key from an api key, password, optional bucket name and optional encrypted path:

```
password    = userPassword or defaultPassword
saltEntropy = getBucketSalt(apiKey, bucketName)
mixedSalt   = hmac(hash=sha256, secret=password, data=saltEntropy)
pathSalt    = hmac(hash=sha256, secret=mixedSalt, data=encryptedPath or "")
rootKey     = argon2id(salt=pathSalt, password=password)
```

If no password is specified for the bucket, the default password is used.

In the terminology of section 4.11 in the [whitepaper](https://storj.io/storjv3.pdf), the algorithm above describes how to derive s<sub>0</sub>. Subsequent secrets are derived using the unencrypted path components and will end up encrypted as described in the whitepaper and appended to the base encrypted path used when deriving s<sub>0</sub>.

## Rationale

This design accomplishes all of the requirements listed above.

- Each root key requires some secret that necessarily comes from the user in some way.
- The algorithm is deterministic, so the user gets the same keys on any machine where she inputs the same passwords.
- Root keys for a sub-path in a bucket are accomplished by providing a non-empty encrypted path when deriving the root key.
- Root keys do not depend on the name of the bucket they are in: just the salt associated with that bucket, allowing renames.
- Because entropy is added as the salt in the `argon2id` step, dictionary attacks on low-entropy passwords cannot be precomputed.
- The default password can be used for any bucket and will return a distinct key as long as the salt for each bucket differs.

Some other points of consideration include

- Clients can ignore the salt if they wish and know they have a strong password. Indeed, they can ignore all of this and do their own key derivation. At a minimum, we allow the user to specify an already-formed encryption key in their configuration if they don't want one derived from a passphrase.
- The salt is not blindly trusted from the satellite, and is mixed with the password in an HMAC step that ensures a hostile satellite cannot predict what the actual salt will be.
- HMAC provides security against forgeries without knowing the secret, which implies that sharing the output of a HMAC does not leak information about the secret, even under attacker provided data, so I believe the usage of HMAC is safe.
- Users will already lose data if they do not back up their data pointers in the satellite (because the encryption key for the data is only stored there), so adding the salts does not add a requirement that users need to back up data in a satellite, it just adds a small amount of additional information they need to back up.

## Implementation

First, satellites must add a field on buckets and projects to store the salts. It can either run a migration and fill them all at once, or fill it on demand. It should use random bytes from the Go `crypto/rand` package, and be large enough to handle all forseeable usage (32 bytes should be sufficient).

Next, the clients must be changed to run the above algorithm when deriving a key from a password.

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
5. A table of keys for low entropy passwords should not exist.
6. There is a default that allows deriving keys for buckets when no bucket specific secret is provided.

The third requirement allows having multiple encrypted domains to exist within a single bucket, allowing delegation of encryption. The fourth requirement allows renaming buckets without having to re-encrypt all of the paths inside of a bucket. The fifth requirement enhances security if a satellite breach happens. This is usually accomplished with [salting](https://en.wikipedia.org/wiki/Salt_(cryptography)). The sixth requirement allows providing a finite amount of information to a third party that allows them access to the keys for any bucket created in the future.

## Design

A default password can be chosen. There is no extra entropy or information available, so it will be run through [Argon2](https://en.wikipedia.org/wiki/Argon2) with a very high work load and a fixed salt. The output of that will be stored in a secure location. The higher the workload, the better, as it increases the cost for an offline attack, and only the output is used in subsequent steps, so it only needs to happen once.

Satellites will add a new field to bucket metadata called `entropy` that just contains 32 random, stable bytes.

The algorithm used to derive a root key for some path in some bucket for some secret is

```
bucketEntropy = getBucketMetadata(bucketName).entropy
bucketRootKey = argon2id(salt:hmac(hash:sha256, secret:secret, data:bucketEntropy), secret:secret)
pathRootKey   = hmac(hash:sha256, secret:bucketRootKey, data:encryptedPath)
```

In the terminology of section 4.11 in the [whitepaper](https://storj.io/storjv3.pdf), the algorithm above describes how to derive s<sub>0</sub>. Subsequent secrets are derived using the unencrypted path components and will end up encrypted as described in the whitepaper and appended to the base encrypted path used when deriving s<sub>0</sub>.

## Rationale

This design accomplishes all of the requirements listed above.

- Each bucket root key requires some secret that necessarily comes from the user in some way.
- The algorithm is deterministic, so the user gets the same keys on any machine where she inputs the same secrets.
- Root keys for a sub-path in a bucket are accomplished by providing a non-empty encrypted path when deriving the `pathRootKey`.
- Root keys do not depend on the name of the bucket they are in: just the entropy associated with that bucket, allowing renames.
- Because the entropy is added as the salt in the `argon2id` step, dictionary attacks on low-entropy passwords cannot be precomputed.
- The default password can be used for any bucket and will return a distinct key, and the output of the default password derivation can be shared with others to allow them to do the same.

Some other points of consideration include

- Clients can ignore the bucket entropy if they wish and know they have a strong password. Indeed, they can ignore all of this and do their own key derivation.
- The entropy is not blindly trusted from the satellite, and is mixed with the secret in an HMAC step that ensures a hostile satellite cannot predict what the salt will be.
- HMAC provides security against forgeries without knowing the secret, which implies that sharing the output of a HMAC does not leak information about the secret, even under attacker provided data, so I believe the usage of HMAC is safe.
- The entropy can be used for other purposes in the future if necessary.
- Users will already lose data if they do not back up their data pointers in the satellite (because the encryption key for the data is only stored there), so adding the bucket entropy does not add a requirement that users need to back up data in a satellite, it just adds a small amount of additional information.

## Implementation

First, satellites must add an entropy field to bucket metadata. It can either run a migration and fill them all at once, or fill it on demand. It should use random bytes from the Go `crypto/rand` package, and be large enough to handle all forseeable usage (32 bytes should be sufficient).

Next, the clients must be changed to run the above algorithm. One possible difficulty is if the APIs require knowing the encryption information before the bucket metadata is able to be requested, but that can be worked around in a number of ways.

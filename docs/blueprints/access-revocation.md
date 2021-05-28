# Access revocation

## Abstract

This blueprint describes a design to implement access revocation on the
satellite.

## Terminology

An `access` is a serialized data structure that contains the information
necessary to authorize actions on the Storj network. An access contains a
macaroon.

A `macaroon` is a structured token included in each call to the satellite. The
macaroon contains a key identifier, any "caveats" that may restrict the
macaroon's privileges, and a signature. (For more information about macaroons,
please see https://research.google/pubs/pub41892).

A `tail` is a cryptographic hash of a portion of the macaroon, generated to
validate the macaroon. In the process of macaroon validation, a chain of tails
is generated. First the macaroon's root identifier is signed with a secret to
generate the first tail. That tail is used as the "secret" to sign the next
portion of the macaroon (a caveat) to produce a second tail. This process
continues until the entire macaroon has been signed, which results in a `final
tail`.

## Background

Currently there is no way to revoke a macaroon once it is created. Because a
macaroon is validated by a cryptographic signature, once signed it is always
cryptographically valid. Thus, if the macaroon does not contain an expiration
caveat its privileges will never be revoked or expire.

The holder of a macaroon can create new, further-caveated macaroons without
coordinating with a centralized service or with the entity that provided them
with the macaroon. This makes macaroons ideal for distributed authorization, but
presents challenges when revoking a macaroon.

For example, if I hold the API key for Project A, I can create Macaroon A with a
caveat that it can only read and write files within Bucket A. I can
then share this macaroon with my own customer, Customer A. Customer A may then,
if they wish, create Macaroon B which is further caveated -- for example,
restricted to read-only access in Bucket A -- and share Macaroon B with someone
else. This can occur without my knowledge.

I may then want to revoke Macaroon A. By so doing, I would want Macaroon B to
also be revoked, and any other macaroons created from Macaroon A.

## Design

### Revoking a macaroon

We will create a satellite endpoint that receives revocation requests. A request
to this endpoint must include the _macaroon to revoke_. The request must also
include a macaroon authorizing this request. To be authorized, the _authorizing
macaroon_ must be a _parent_ of the macaroon to revoke. This means the final
tail of the authorizing macaroon must match a tail in the macaroon to revoke. In
this way we allow a holder of a macaroon to revoke any further-attenuated
macaroons that are based on the one they hold.

The satellite will respond with a success or failure, whether the macaroon was
successfully revoked.

### Handling revocation requests

The satellite will maintain a database of revoked final tails. When it receives
a macaroon to revoke, it will first validate the request, then calculate the
final tail for the macaroon, and store that value in a revocation database.

### Checking for revocation

When a request comes in to the satellite, in addition to checking the macaroon
for cryptographic validity, the satellite will check if it contains any tails
that have been revoked. By checking each tail of the macaroon it ensures that
any "sub-macaroons" created from a revoked macaroon will also be revoked.

The satellite will calculate the _final tail_ for the macaroon and check it
against an expiring bounded LRU cache to see if the tail has been seen recently.
If the tail is in the cache, this cache will give the result for whether it is
"valid" or "revoked". If the _final tail_ is not in the cache, it will query the
database.

The database query will include _all tails_ in the macaroon to see if any of
them are revoked. The result of this query will be stored in the cache, with the
_final tail_ being noted as "valid" or "revoked," so that the database does not
need to be consulted again for this macaroon (within some configurable expiring
window).

## Rationale

This approach was deemed best because it:

- Adds very little latency to each request.
- Creates very little load on the database.
- Is backwards compatible, and allows us to revoke existing macaroons.
- Allows us to revoke an entire "macaroon tree" while maintaining the
  distributive properties of macaroons.

Disadvantages to this approach:

- A satellite adds an additional piece of critical information (revoked tails)
  that it must never lose.
- Revocation is not immediate. If a revoked key has been used recently, it will
  still be valid until it expires from the whitelist cache. With proper tuning
  this shouldn't be a big issue.

Other approaches considered:

- Revocation as a third-party caveat. In this approach, a "revocable" macaroon
  contains a third-party caveat that must be discharged when submitting the
  macaroon to the satellite. The client must include a "discharge macaroon" with
  the regular macaroon that proves the macaroon is not revoked.
    - Advantages: when receiving a request, a satellite only needs to
      cryptographically verify the macaroon without having to check caches or a
      db.
    - Disadvantages: it's more complicated, existing macaroons cannot be
      revoked, extra latency required to get discharge macaroon.

## Implementation

### Database

I created a proof-of-concept database with a single indexed column storing the
tails. I placed 1,000,000 fake revoked keys in the database, and benchmarked
various scenarios (small macaroons, large macaroons, valid macaroons, revoked
macaroons), and the database performed well.

We used the following query:

`select exists(select 1 from revoked where tail in ([tails]))`

When testing a macaroon with 500 caveats (much larger than a real-life
scenario), with a valid macaroon, it resulted in an index-only scan and took
less than 6 ms.

### Cache

The whitelist/blacklist cache will be implemented in a way similar to our
current api keys cache. It will be bounded LRU cache and will store the known
_final tail_ and a boolean for whether it is valid. In this way the same cache
can be used for both whitelist and blacklist.

## Open issues

- None

# Title: Path Component Encoding

## Abstract

This document describes a way to handle path component encoding such that we can

- Always find a key that would be immediately before or after any other key
- Have minimal space used to store them
- Be consistent with empty path components for encrypted and unencrypted paths
- Works well with C style strings (avoids the null byte)

## Background

Paths are made up of components. For example, the path `foo/bar/baz` contains as components `"foo"`, `"bar"`, and `"baz"`. Similarly, `foo//bar` contains the components `"foo"`, `""`, and `"bar"`, and `/foo/bar` contains `""`, `"foo"` and `"bar"`. An empty path is invalid.

When encrypting a path, the components are encrypted individually, and the separator `/` joins them. Empty path components are, at the time of writing, encrypted to non-empty strings, but, every encryption algorithm does no padding and so leaks the length of the plaintext, meaning that encrypting empty path components provides no secrecy.

When iterating over paths in a database, it is helpful to be able to generate from a given path another path that immediately proceeds or follows in lexicographic order. This has challenges when a path components is empty, as there is no byte to increment or decrement. At the time of writing, this challenge does not happen for encrypted paths, because the encrypted output has some bytes of nonce and authentication tag.

At the time of this writing, all encrypted path components are base64 encoded, ensuring that no `/` characters are present changing the meaning of which components exist.

## Design

Because encrypting empty path components does not provide secrecy, we stop encrypting them.

Define an encoding method for path components where

- The empty path component is encoded as `\x01`
- Any other path component is encoded as `\x02 + escape(component)`

Escape serves three purposes,

1. Ensure that the byte literal `/` is not present in the component
2. Have the property that `A < B` implies `escape(A) < escape(B)` so that it preserves lexicographic ordering
3. Allow a string to be constructed that is the greatest string less than `escape(A)` for any `A`.

For this third point, for some constructed string, there may not exist any string `A` such that `escape(A)` is equal to it. Thus it cannot be unescaped. Unescaping is not necessary to do efficient database iteration, though, and allows us to squeeze strings in between the escaped representation, making increment and decrement much easier.

Escaping is defined as follows. Note that `/` has ASCII value 47 or `\x2f`. Since `\x2f` is not allowed to appear in the output of an escaped path component,

- `\x2e` escapes to `\x2e\x01`
- `\x2f` escapes to `\x2e\x02`

This ensures no `\x2f` is present in the string, and maintains lexicographic ordering. Finally, we disallow the bytes `\xff` and `\x00` to exist in the string. This is so that we never underflow when subtracting one from the string, and we can append a single `\xff` to a subtracted string to create a greatest string smaller than it. Additionally, because `\x00` is not present, there is no issue with C style null terminated strings, and finding the smallest string larger than a given string is as easy as appending `\x00`. Thus,

- `\xfe` escapes to `\xfe\x01`
- `\xff` escapes to `\xfe\x02`
- `\x00` escapes to `\x01\x01`
- `\x01` escapes to `\x01\x02`

This encoding happens for all path components, encrypted and unencrypted alike.

For example,

| Path       | Encoded                | Proceeding                 | Following                  |
|------------|------------------------|----------------------------|----------------------------|
| `foo/`     | `\x02foo/\x01`         | `\x02foo/\x00\xff`         | `\x02foo/\x01\x00`         |
| `foo/\x00` | `\x02foo/\x02\x01\x01` | `\x02foo/\x02\x01\x00\xff` | `\x02foo/\x02\x01\x01\x00` |
| `foo/\x01` | `\x02foo/\x02\x01\x02` | `\x02foo/\x02\x01\x01\xff` | `\x02foo/\x02\x01\x02\x00` |
| `foo/\x2e` | `\x02foo/\x02\x2e\x01` | `\x02foo/\x02\x2e\x00\xff` | `\x02foo/\x02\x2e\x01\x00` |
| `foo/\x2f` | `\x02foo/\x02\x2e\x02` | `\x02foo/\x02\x2e\x01\xff` | `\x02foo/\x02\x2e\x02\x00` |
| `foo/\xfe` | `\x02foo/\x02\xfe\x01` | `\x02foo/\x02\xfe\x00\xff` | `\x02foo/\x02\xfe\x01\x00` |
| `foo/\xff` | `\x02foo/\x02\xfe\x02` | `\x02foo/\x02\xfe\x01\xff` | `\x02foo/\x02\xfe\x02\x00` |

Note that `foo/\x2f` is actually `foo//`, but it's written that way to show what would happen if an encrypted component ended up with a `\x2f` character. In other words, it's discussing the path with components `"foo"` and `"/"`, and not the path with components `"foo"`, `""`, and `""`.

## Rationale

This design reduces metadata overhead by allowing almost any byte to be in the component. Assuming that encrypted data is uniformly random, we only expect a 6/256 expansion factor plus 1 byte per path component. This works out to approximately 3% versus the hefty 33% that base64 imposes.

The design escapes characters that could cause underflows or overflows, and ensures that no path components are empty. This makes increment and decrement always defined and they only have to consider the last byte in the string.

It is the case that `escape(increment(A))` is not equal to `increment(escape(A))` (as well as with `decrement`). This does not cause any issues, as they are only used as input to seeks in databases, but it is important to be aware of it.

## Implementation

The encryption package must implement the escaping and encoding algorithms as described, and they must be used in place of base64.

This is not backwards compatible.

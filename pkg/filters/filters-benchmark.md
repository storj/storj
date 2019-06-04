
# Filters benchmark

## Bloom filters
Three Bloom filter implementations are considered:
- **BF1**: Zeebo's bloom filters (github.com/zeebo/sbloom)
- **BF2**: Willf's Bloom filters (github.com/zeebo/sbloom)
- **BF3**: Steakknife's Bloom filters (github.com/golang/leveldb/bloom)

A bloom filter is a probabilistic data structure used to test if an element belongs to a set. It can raise false positives, but no false negatives. 
A Bloom filter is an array of *m* bits, and a set of *k* hash functions that return an integer between 0 and *m-1* . To add an element, it has to be fed to the different hash functions and the bits at the resulting positions are set to 1. 

The probability of having a false positive depends on the size of the Bloom filter, the hash functions used and the number of elements in the set.

- **n**: number of elements in the set
- **m**: size of the Bloom filter array
- **k**: number of hash functions used
- **Probability of false positives**: (1-(1-1/m)^kn)^k which can be approximate by (1-e^(kn/m))^k.

### Zeebo's bloom filters
Parameters:
- **k**: The Bloom filter will be built such that the probability of a false positive is less than (1/2)**k
- **h**: hash functions

### Willf's bloom filters
Parameters:
- **m**: max size in bits
- **k**: number of hash functions

### Steakknife's bloom filters
Parameters:
- **maxElements**: max number of elements in the set
- **p**: probability of false positive
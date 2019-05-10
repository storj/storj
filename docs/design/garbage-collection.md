
# Garbage Collection

## Abstract
 
## Background
When clients move, replace, or delete data, Satellites, or clients on behalf of Satellites, will notify storage nodes that they are no longer required to store that data. 
 In configurations where delete messages are issued by the client, the metadata system (and thus a Satellite, with Satellite reputation on the line) will require proof that deletes were issued to a configurable minimum number of storage nodes. 
 This means that every time data is deleted, storage nodes that are online and reachable will receive notifications right away.
Storage nodes will sometimes be temporarily unavailable and will miss delete messages. 
In these cases, unneeded data is considered garbage.

## Ways to create garbage data:
- Failed upload
- Upon deletion
- Upon replacement
- When a satellite makes a repair and drops the node


### Today's implementation
**Repair**
- When a repair is issued and a storage node is removed from the pointer, a delete is issued to let it know it does not have to store the piece anymore.

**Delete**
1. When a delete object command is issued, the uplink retrieves the list of segments for this object, and Delete is called on each segment. 		
2. A delete segment request is sent to the satellite. 		
3. The satellite deletes the segment and sends the order limits for deleting the segment on known online storage nodes.
4. A delete piece request is sent by the uplink to the corresponding storage node for each addressed order limit.


## Design
### Deletion Process
- The uplink should send a “DataDeletionReport” to the satellite with the list of pieces and corresponding storage nodes it has been unable to delete
- The satellite responds with a message indicating if there are still k nodes detaining pieces of the segment. If there are, the delete process has failed.
- The satellite keeps track of undeleted pieces and corresponding storage nodes. 		
- When a storage node comes back online, or just want to perform a clean-up, it sends a request to the satellite. The satellite replies with the list of pieces id it may delete (possibly using a bloom filter).
- The satellite removes the piece id and storage node id from its “not deleted pieces” table. 	 

### Garbage Collection
**Approach from the whitepaper:**
- The uplink makes a request to the satellite 
- The satellite replies with a hash of the pieces the storage node should be holding
- If the storage node detects a difference, it makes a second request to the satellite
- The satellite replies with the bloom filter of the pieces the storage node should keep
- Upon receiving the bloom filter, the storage node checks, for each piece, if it is in the set. If it is not, it deletes it. The storage node may still hold deleted pieces, as bloom filter can trigger a false positive.

**Bloom filter:** 
A bloom filter is a probabilistic data structure used to test if an element belongs to a set. It can raise false positives, but no false negatives. 
A Bloom filter is an array of *m* bits, and a set of *k* hash functions that return an integer between 0 and *m-1* . To add an element, it has to be fed to the different hash functions and the bits at the resulting positions are set to 1. 

The probability of having a false positive depends on the size of the Bloom filter, the hash functions used and the number of elements in the set. 


## Rationale

[A discussion of alternate approaches and the trade offs, advantages, and disadvantages of the specified approach.]

## Implementation

[A description of the steps in the implementation.]

## Open issues (if applicable)
Q: What is a deleted segment?
- A segment that has no recoverable metadata
- A segment that has no recoverable metadata and is, even with metadata, not repairable
- Something else?

The whitepaper states that a proof of deletion should be sent from the uplink to the satellite. 
- What is the expected behavior in this case? Is the deletion canceled if no proof is received? 
- Do we introduce a “prepare-to-delete” state, and what should happen if the proof of deletion never comes?



# Storage Node Garbage Collection
 
## Abstract
This design doc describes how the satellite should notify storage nodes about garbage pieces that they may be holding, and how storage nodes should go about deleting that data.

## Background
When clients move, replace, or delete data, Satellites, or clients on behalf of Satellites, will notify storage nodes that they are no longer required to store that data. 
 In configurations where delete messages are issued by the client, the metadata system (and thus a Satellite, with Satellite reputation on the line) will require proof that deletes were issued to a configurable minimum number of storage nodes. 
 This means that every time data is deleted, storage nodes that are online and reachable will receive notifications right away.
Storage nodes will sometimes be temporarily unavailable and will miss delete messages. 
In these cases, unneeded data is considered garbage.

## Ways to create garbage data:
- Failed or interrupted upload (we used to delete the previously uploaded segments but since the introduction of order limits, we no longer do this.)
- Regular upload where the longtail is canceled (e.g. uploading to 90 pieces, but the slowest 10 are cut, so we end up with 80 relevant)
- Upon deletion
- Upon replacement
- When a satellite makes a repair and drops the node
- When the client stops paying their bills
- Uplink uploads data without committing it

### Today's implementation
**Repair**
- When a repair is issued and a storage node is removed from the pointer

**Delete**
1. When a delete object command is issued, the uplink retrieves the list of segments for this object, and Delete is called on each segment.
2. A delete segment request is sent to the satellite. 
3. The satellite deletes the segment and sends the order limits for deleting the segment on known online storage nodes.
4. A delete piece request is sent by the uplink to the corresponding storage node for each addressed order limit.

## Design
### Deletion Process
- The satellite keeps track of pieces and corresponding storage nodes by creating a new bloom filter for every storage node.
    - The satellite creates the in-memory bloom filters using storage node IDs and piece IDs gotten from the pointerdb.
    - As an early implementation, this bloom filter creation process can be integrated with the data repair checker loop that periodically accesses the pointerdb. This will lessen pointerdb overhead vs. creating a new process.
- The satellite periodically pushes a bloom filter (or cuckoo filter) containing the list of piece IDs for the storage node to delete.
    - If the storage node misses the push because it's offline, it will just miss that GC cycle and catch the next one.
-  Each bloom filter will have a certain creation datetime. The storage node walks all pieces older than the bloom filter datetime and checks whether the piece exists in the bloom filter, if not then deletes it.
    - There could also be some additional short time period (e.g. one hour) to be sure that it covered possible differences in the clocks. Storage nodes need to be accurate within an hour or they will suffer reputation failure.

### Garbage Collection
#### Approach from the whitepaper
- The uplink makes a request to the satellite 
- The satellite replies with a hash of the pieces the storage node should be holding
- If the storage node detects a difference, it makes a second request to the satellite
- The satellite replies with the bloom filter of the pieces the storage node should keep
- Upon receiving the bloom filter, the storage node checks, for each piece, if it is in the set. If it is not, it deletes it. The storage node may still hold deleted pieces, as bloom filter can trigger a false positive.

### Service
```protobuf
service GarbageCollection {
    rpc Delete(DeleteRequest) returns (DeleteResponse);
}

message Filter {
    ...
}

message DeleteRequest {
    Filter filter = 1;
}
```

#### Bloom filter
A bloom filter is a probabilistic data structure used to test if an element belongs to a set. It can raise false positives, but no false negatives. 
A Bloom filter is an array of *m* bits, and a set of *k* hash functions that return an integer between 0 and *m-1* . To add an element, it has to be fed to the different hash functions and the bits at the resulting positions are set to 1. 

The probability of having a false positive depends on the size of the Bloom filter, the hash functions used and the number of elements in the set.

----

In our implementation, the satellite should create a new Bloom filter (or cuckoo filter) for every storage node that includes all piece IDs that the storage node should have.

We also can't remove entries from a Bloom filter, only add, meaning that the Satellite will need to frequently regenerate the Bloom filters.

Since currently the repair checker considers every piece id and node id anyway, we will integrate storage node garbage collection in the checker loop for the short term, but more long-term, we should have the Bloom filter generation run off of a snapshot of the database in a separate server. It doesn't need to necessarily run every day, but perhaps once a week.

An advantage of using the Bloom filter is that it knows which pieces a storage node should hold. We don't have to care about how the garbage was created. Otherwise, if we do garbage collecting in a different way for specific scenarios (such as those listed under "Ways to create garbage data"), we would need to make sure we cover each case.

Previously we'd planned on building reverse index functionality for pointerdb, but doing so would require storing tons of data. This would cause RAM issues eventually. In the case of the Bloom filter, RAM becomes less of an issue, but compute time becomes more of one.

Whether we use Bloom filters, cuckoo filters, or another data structure for adding up data at rest, we need to make sure that it's something we can do concurrently, and then merge later. At some point we'll need to partition the garbage collection service.

## Rationale

### Interesting figures
- piece id size: currently 32 bytes
- size of piece: s_max, s_min
- minimum number of pieces id for a storage node storing x bytes of data: d/s_max
- packet size?

### Thoughts
What could we send the node:
- list of useless pieces for a storage node
    - would mean the satellite has to keep track of these useless pieces. 
    - This list of pieces id would probably be smaller than the list of useful pieces if the storage node and the uplink are trustworthy.
- list of useful pieces for a storage node
    - no need for the satellite to track deleted pieces for each storage node (except for audit purposes) 
    - More robust against nodes and uplinks that are not trustworthy
    - possibility to use a Bloom filter
- We also had the idea of using two bloom filters (one containing pieces that should be deleted, one for pieces that should not be deleted), but that could potentially give us a false positive for deleting a piece. We definitely shouldn't delete useful pieces, so this would be too risky.

### Bloom filter
- **n**: number of elements in the set
- **m**: size of the Bloom filter array
- **k**: number of hash functions used
- **Probability of false positives**: (1-(1-1/m)^kn)^k which can be approximate by (1-e^(kn/m))^k.

| m/n|k|k=1	|k=2	|k=3	|k=4	|k=5	|k=6	|k=7	|k=8
|---|---|---|---|---|---|---|---|---|---|
|2	|1.39	|0.393	|0.400	|	 	 |	 |	| |	| 
|3	|2.08	|0.283	|0.237	|0.253	 |	 |	| |	| 	 
|4	|2.77	|0.221	|0.155	|0.147	|0.160|	| |	|| 	 	 
|5	|3.46	|0.181	|0.109	|0.092	|0.092	|0.101|	||| 	 	 
|6	|4.16	|0.154	|0.0804	|0.0609	|0.0561	|0.0578	|0.0638	||| 	 
|7	|4.85	|0.133	|0.0618	|0.0423	|0.0359	|0.0347	|0.0364|||	 	 
|8	|5.55	|0.118	|0.0489	|0.0306	|0.024	|0.0217	|0.0216	|0.0229||	 
|9	|6.24	|0.105	|0.0397	|0.0228	|0.0166	|0.0141|	0.0133|	0.0135|	0.0145|
|10	|6.93	|0.0952	|0.0329	|0.0174	|0.0118	|0.00943|	0.00844|	0.00819	|0.00846|
|11	|7.62	|0.0869	|0.0276	|0.0136	|0.00864|	0.0065|	0.00552|	0.00513	|0.00509|
|12	|8.32	|0.08	|0.0236	|0.0108	|0.00646|	0.00459|	0.00371|	0.00329	|0.00314|
|13	|9.01	|0.074	|0.0203	|0.00875	|0.00492|	0.00332	|0.00255|	0.00217|	0.00199|
|14	|9.7	|0.0689	|0.0177	|0.00718	|0.00381|	0.00244	|0.00179|	0.00146|	0.00129|
|15	|10.4	|0.0645	|0.0156	|0.00596	|0.003	|0.00183	|0.00128|	0.001|	0.000852|
|16	|11.1	|0.0606	|0.0138	|0.005	|0.00239	|0.00139	|0.000935|	0.000702|	0.000574|
|17	|11.8	|0.0571	|0.0123	|0.00423	|0.00193	|0.00107|	0.000692|	0.000499|	0.000394|
|18	|12.5	|0.054	|0.0111	|0.00362	|0.00158	|0.000839|	0.000519|	0.00036|	0.000275|
|19	|13.2	|0.0513	|0.00998|	0.00312	|0.0013	|0.000663|	0.000394|	0.000264|	0.000194|
|20	|13.9	|0.0488	|0.00906|	0.0027	|0.00108|	0.00053|	0.000303|	0.000196|	0.00014|
|21	|14.6	|0.0465	|0.00825|	0.00236	|0.000905|	0.000427|	0.000236|	0.000147|	0.000101|
|22	|15.2	|0.0444	|0.00755|	0.00207	|0.000764|	0.000347|	0.000185|	0.000112|	7.46e-05|
|23	|15.9	|0.0425	|0.00694|	0.00183	|0.000649|	0.000285|	0.000147|	8.56e-05|	5.55e-05|
|24	|16.6	|0.0408	|0.00639|	0.00162	|0.000555|	0.000235|	0.000117|	6.63e-05|	4.17e-05|
|25	|17.3	|0.0392	|0.00591|	0.00145	|0.000478|	0.000196|	9.44e-05|	5.18e-05|	3.16e-05|
|26	|18	0.|0377	|0.00548	|0.00129	|0.000413|	0.000164|	7.66e-05|	4.08e-05|	2.42e-05|
|27	|18.7	|0.0364	|0.0051	|0.00116	|0.000359|	0.000138|	6.26e-05|	3.24e-05|	1.87e-05|
|28	|19.4	|0.0351	|0.00475|	0.00105	|0.000314|	0.000117|	5.15e-05|	2.59e-05|	1.46e-05|
|29	|20.1	|0.0339	|0.00444|	0.000949	|0.000276|	9.96e-05|	4.26e-05|	2.09e-05|	1.14e-05|
|30	|20.8	|0.0328	|0.00416|	0.000862	|0.000243|	8.53e-05|	3.55e-05|	1.69e-05|	9.01e-06|
|31	|21.5	|0.0317	|0.0039	|0.000785	|0.000215|	7.33e-05|	2.97e-05|	1.38e-05|	7.16e-06|
|32	|22.2	|0.0308	|0.00367|	0.000717	|0.000191|	6.33e-05|	2.5e-05|	1.13e-05|	5.73e-06|

see: [Bloom filter math](http://pages.cs.wisc.edu/~cao/papers/summary-cache/node8.html)
#### Hash functions choice

### Integration of deleted segments in the audit system
*Should we and how we integrate deletion in the audit system?*

## Implementation
- Determine if a Bloom filter or cuckoo filter would make the most sense for associating nodes with pieces that need to be deleted
- The data repair checker should create a filter for each storage node that it checks, and its piece IDs
- Implement the garbage collection service defined by the interface
    - Satellite should be able to send a Delete request to a storage node
    - Storage node should be able to receive a Delete request from a Satellite
- Storage node should use the filter from the Delete request to decide which pieces to delete, then delete them
- Eventually, this service should iterate over a db snapshot instead of being integrated with the data repair checker

## Open issues (if applicable)
Q: What is a deleted segment?
- A segment that has no recoverable metadata
- A segment that has no recoverable metadata and is, even with metadata, not repairable
- Something else?

The whitepaper states that a proof of deletion should be sent from the uplink to the satellite. 
- What is the expected behavior in this case? Is the deletion canceled if no proof is received? 
- Do we introduce a “prepare-to-delete” state, and what should happen if the proof of deletion never comes?
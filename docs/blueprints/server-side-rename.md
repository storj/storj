# Server-side move

## Abstract

This blueprint describes a design to implement server-side move feature.

## Background

Server-side move (rename) is a very useful feature as since people quite often move things around. Currently, there is no easy and fast way to move an object without re-uploading object content. After Metainfo refactor this feature can be implemented without any significant obstacle.

## Design

Move operation needs to be as fast as possible and should not involve re-uploading the existing object. Uploaded content is encrypted using a random key. Later this random key is encrypted with a key derived from the object key and nonce. To encrypt each segment we are using different keys. All keys are stored on the satellite side, one for each segment. Also, the object key is encrypted before sending to the satellite. To move object without re-uploading whole content we need to do several things:
- read encrypted keys for all object segments (uplink)
- encrypt *new* object key (uplink)
- decrypt all received encrypted keys and decrypt it with *new* key derived from *new* object key (uplink)
- send *new* object key and *new* encrypted keys (uplink/satellite)
- replace *old* object key and *old* encrypted keys with *new* received from uplink in atomic operation (satellite)

The above list cannot be executed in a single uplink->satellite request so we will need at least two calls to satellite to
perform move. The first call will be used to get all necessary data needed for move from the satellite and the second will be to send updated data back to the satellite.

### Satellite

Satellite needs to define two new Metainfo endpoint methods: `BeginMoveObject` and `FinishMoveObject`. The first method will provide encrypted keys for all object segments. The second method will accept new encryption keys for moved object and will update the corresponding object `ObjectKey` and segments `EncryptedKey` in an atomic database operation. Because an object can have a large number of segments both methods needs to handle receiving and sending requests in chunks.

Object to move should be searched with `project_id, bucket_name, object_key, version, stream_id`. This will avoid situation where we will try to move different object e.g. uploaded in the middle of move. In such case `stream_id` will be different for new object. While doing move of an object key and replacement of encrypted keys we should not touch object `CreatedAt` field.

One of the difficulties related to renaming the object key is a possibility that we will miss a moved object in metainfo loop. Such case can happen when an object will be moved in a way where it will be placed in the area which was already processed by metainfo loop but object before renaming was not processed yet (e.g. from `object10000` to `object1` and loop was currently processing objects between new and old, as the loop is going in ascending direction). 

Another case is when 'old' and 'new' objects are processed two times during metainfo loop execution. Such situation can happen when object will be moved after object was processed by metainfo loop and it was moved to an area that will be still processed by the loop.

Services that can be affected by a missing object or counting it two times:
- **Garbage Collection**: missing object creates a dangerous situation because if an object will be missed in the loop then it won't be added to the GC bloom filter and the object can be cleanup by mistake. To avoid that we need to refactor GC service and move collecting pieces outside metainfo loop. Garbage collection service should not care about objects and should collect pieces by looping only segments table. If object is processed two times then pieces will be added two times to bloom filter which should be safe.
- **Audit**: not matter if we will miss an object or count it twice it should only increase/decrease change for auditing this object during single metainfo loop execution. At some point most probably Audit should iterate only over segments only (like GC).
- **Data Repair**: with a missed object we are risking that repair will start a few hours later with next metainfo loop execution, with object processed two times we will have two objects in the repair queue. In such case repairing old object will fail as the segments repairer will not find the old object key.
- **Graceful Exit**: with missed object there is a risk that segments from such object won't be transferred, with object processed twice GE will try to add the segment’s pieces of exiting node to GE transfer queue but there is a check if inserted pieces are already in the queue. In such a case, they are not added again. At some point most probably GE should iterate only over segments only (like GC). That will resolve issue with risk of missing object.
- **Metainfo > Expired Deletion**: it's moved to a separate chore (outside metainfo loop) but can have the same issues as metainfo loop. If we will miss deleting object it will be deleted during the next chore execution. If we will try to delete original object and renamed object no harm should be done as SQL query will find only segments from one object (original or renamed).
- **Accounting / Tally**: with a missed object we may account less user and pay less SNO, with object processed two times we may account more user and pay more SNO. Either user or SNO can have inaccurate invoice/payment but we need to remember that this is only for single metainfo loop execution so inaccuracy most probably will be unnoticeable.
- **Metrics**: not matter if we will miss an object or count it twice, such temporary inaccuracy should be acceptable.

The first implementation can handle only objects with a number of segments that can be handled in a single request (e.g. 10000). Renaming objects with a larger number of segments should be rejected.

As a next step, we can handle renaming objects with a large number of segments by keeping partial results of move in a separate database table. The final move operation will be applied when all new encrypted keys will be collected. For such operation `BeginMoveObject` method should return a unique operation identifier which should be used with `FinishMoveObject`. This will give us the ability to safely link together begin and finish requests. Keeping partial results should be limited by time as the user can start move operation but not necessarily finish it (e.g. cancellation, crash during operation).

### Uplink

Uplink needs to expose a new method in public API `MoveObject(bucketName, oldName, newName)`. Internally this method will need to read all segments from `oldName` object. Then, by using access that can decrypt this uploaded object metadata, uplink will decrypt all encrypted keys and will encrypt them with a key derived from `newName`. The last step will be to send updated data to the satellite.

## Open issues

- How deal with object with large number of segments (e.g. 1M)

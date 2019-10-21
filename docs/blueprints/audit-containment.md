# Auditing and Node Containment

## Abstract

This design doc outlines how we will implement "containment mode," in which the auditing service monitors a node that has received an audit request, but refuses to send the requested data.
Currently in the codebase, nodes that do this will be marked as having audit failures.
However, we don't want to immediately mark an audit failure if this specific case arises, because factors such as unintentional misconfiguration by the SNO, or a busy node fulfilling requests should not necessarily warrant the penalty of an audit failure mark.
Instead, we want to "contain" these nodes by retrying the original audit until they either eventually respond and pass the audit, or refuse to respond a certain number of times and ultimately receive an audit failure mark.

## Main Objectives

1. Nodes won’t be able to “escape” audits for specific data by refusing to send requested data or going offline right after receiving a request from a Satellite.
See _Identifying Nodes That Need to Be Contained_ subsection below for how we consider "refusing."
2. The audit service should attempt to verify the nodes' originally requested data in addition to continuing normal audits for that node.
3. ContainmentDB holds metadata required to verify contained nodes' originally requested data.

## Background

The whitepaper section 4.13 talks about containment mode as follows:

> Given a specific storage node, an audit might reveal that it is offline or incorrect. In the case of a node being offline, the audit failure may be due to the address in the node discovery cache being stale, so another, fresh Kademlia lookup will be attempted. If the node still appears to be offline, the Satellite places the node in containment mode. In containment mode, the Satellite will calculate and save the expected response, then continue to try the same audit with that node until the node either responds successfully, actively fails the audit, or is disqualified from being offline too long. Once the node responds successfully, it leaves containment mode. All audit failures will be stored and saved in the reputation system. 

However, we're departing from the whitepaper because offline nodes will not be moved to containment mode.
They will just be marked as offline.
Instead, containment mode is specifically for nodes that initially respond to the audit service's dial but then don't send the requested erasure share.

The node will be given a `contained` flag, and if the audit service attempts to audit the node again, it will see this flag and first request the same share it had originally requested, then continue the new audit right after.
If the storage node fails to send this share again, it will have its `reverifyCount` incremented.

If the `reverifyCount` exceeds a `reverifyLimit`, then the node will be marked as failing the audit and removed from containment mode.
Otherwise, the node will pass the audit and will also be removed from containment mode.

Additionally, other services such as the overlay, repair checker, and repairer should not "care" about the contained flag and functionality should remain the same.
Contained nodes should be handled the same as other nodes in the node selection process.
The contained flag should only be relevant to the audit service.

## Design

### Identifying Nodes That Need to Be Contained
In the audit verifier, we need a better system for handling the different cases in which a storage node may not respond to an audit request.
Here are a few possible cases:
1. The node is busy fulfilling other requests and can't respond to the audit request within the audit timeout defined on the Satellite.
2. The node does not have the audit piece anymore because the SNO deleted it.
3. The node can't read the piece due to a file permission issue (a SNO config mistake).
4. The node can't read the piece due to bad sectors on the HDD.

For 1, the node should be contained. For 2-4, the node should definitely fail the audit.
Basically, we want to able to get a straight answer back when a node returns the wrong data, says "I don't have the data," or "I can't read the data." Because this will mean that they failed the audit. If the storage node completes an initial TLS handshake but then doesn't respond within some timeframe, _then_ we should mark it as contained.

We need to find out what the error messages are for these different cases so that we don't lump the first case with the rest.

### Handling Contained Nodes
New functionality needs to be added to the audit verifier, where upon receiving a pointer, the verifier should first iterate through the nodes listed in the pointer and check if they have a `contained` field set to true on their NodeDossier or not.

For nodes that are not contained, the audit will continue as normal.

For nodes that are contained, the verifier will ask the ContainmentDB for the hash of the erasure share expected from the node when it was originally audited.
The verifier will also use info from the ContainmentDB to make the request to the node for the original erasure share.

The verifier will then attempt to download that erasure share from the contained node.
From there, the following cases can occur:
- Passes audit ->  `pendingAudit` is deleted from the ContainmentDB  -> stats updated in overlaycache/satelliteDB and `contained` field set to false
- Fails audit -> `pendingAudit` is deleted from the ContainmentDB -> stats updated in overlaycache/satelliteDB and `contained` field set to false
- Refuses to respond again -> the `pendingAudit`'s `reverifyCount` is incremented in the ContainmentDB ->
    - -> If that count exceeds the reverify limit, then the node’s stats will be updated to reflect an audit failure and the node will be removed from the ContainmentDB and `contained` field will be set to false.
    - -> If the count does not exceed the reverify limit, the `pendingAudit` will remain in the ContainmentDB.

```go
type pendingAudit struct {
    nodeID            storj.NodeID
    pieceID           storj.PieceID
    pieceNum          int
    stripeIndex       int
    shareSize         int64
    expectedShareHash []byte
    reverifyCount     int // number of times reverify has been attempted
}
```
`pendingAudit` entries are created and added to the ContainmentDB when a node seems to be online and opens a connection with the satellite to be audited (this happens within the existing Verify function in the audit package), but then refuses to send the requested erasure share.
There should not be more than one pendingAudit per node.

Additionally, the satellite should make sure to empty the ContainmentDB of certain pending audits when segments are deleted.

```go
type ContainmentDB interface {
  Get(ctx context.Context, nodeID pb.NodeID) (pendingAudit, error)
  IncrementPending(ctx context.Context, pendingAudit pendingAudit) error
  Delete(ctx context.Context, nodeID pb.NodeID) error
}
```

`Get` will be used in the Reverify function to get the `pendingAudit` information from the ContainmentDB needed to compare the expected audit result with the actual.

`IncrementPending` is an upsert that is used when an "uncooperative" node is identified during the normal process, and when a node doesn't pass a reverification.

`Delete` will be used to delete the pendingAudit from the ContainmentDB after the node either passes or fails the audit.
It will also update the node's NodeDossier to set `contained` to false.

#### Pending Audits SQL Table
```sql
pending_audits (
  node_id bytea NOT NULL,
  piece_id bytea NOT NULL,
  piece_num bigint NOT NULL,
  stripe_index bigint NOT NULL,
  share_size bigint NOT NULL,
  expected_share_hash bytea NOT NULL,
  reverify_count integer NOT NULL,
  PRIMARY KEY ( node_id )
)
```

## Rationale

Originally I had the idea to have a separate service iterate through and check the ContainmentDB.
However, after talking with JT it seemed unnecessary to have another service for this functionality that could be added to the existing verifier code.
I think we also don't want to pound the contained nodes with reverification audits.
They should just be reverified whenever they pass through the audit service normally.
With one service, it would also be very difficult to make sure that the pending audit happened before any other normal audits happened to a contained node.

## Walkthrough

#### 1. Uncooperative nodes are documented in the ContainmentDB:

By "uncooperative," I mean that the verifier will likely be able to successfully DialNode (within pkg/audit/verifier.go), but errors may occur at
```go
downloader, err := ps.Download(timedCtx, limit.GetLimit(), offset, int64(shareSize))
```
or
```go
_, err = io.ReadFull(downloader, buf)
```
Once these errors have occurred and we know that a `pendingAudit` entry should be made in the ContainmentDB, we should use infectious’s `f.Decode` to generate the original stripe.
Then we should use infectious’s `f.EncodeSingle` where we input the stripe and output the missing share. We then make a hash of that missing share to be saved to the ContainmentDB as `ExpectedShareHash`.

`containment.IncrementPending` should be called when a node is found to be “uncooperative."

```go pkg/audit/verifier.go
err := verifier.containment.IncrementPending(ctx, &pendingAudit{
    nodeID:            nodeID,
    pieceID:           pieceID,
    pieceNum:          pieceNum,
    stripeIndex:       stripeIndex,
    shareSize:         shareSize,
    expectedShareHash: shareHash,
}
```

#### 2. Audits continue normally. But now the nodes listed in a pointer are checked for `contained` status.
```go pkg/audit/containment/checker.go
func (verifier *Verifier) Verify(ctx context.Context, stripe *Stripe) (verifiedNodes *RecordAuditsInfo, err error) {
  defer mon.Task()(&ctx)(&err)

  pointer := stripe.Segment

  pieces := pointer.GetRemote().GetRemotePieces()

  // Get the NodeDossiers from the overlay using the pieces’ nodeIDs.
  // Determine if they have the contained flag set to true.
  // If so, call Reverify on the contained nodes in parallel.
  // Wait for the Reverify results. If those nodes passed or failed
  // their reverification, then they should continue to be verified here.
  // Otherwise, if they're still contained, we don't try to verify new shares
  // for them.
  ...
}
```

#### 3. Contained nodes are checked for the same data that they were originally requested to respond with:

```go pkg/audit/verifier.go
func (verifier *Verifier) Reverify(ctx context.Context, node storj.NodeID) (err error) {

  pendingAudit, err := checker.containment.Get(id)

  // Dial the node, then use info from the pendingAudit to download the target share from the node.

  offset := pendingAudit.stripeIndex * pendingAudit.shareSize
  downloader, err := ps.Download(timedCtx, limit.GetLimit(), offset, int64(pendingAudit.shareSize))

  // If the error is a timeout (or any other error?), call IncrementPending.
  // if the download occurred successfully, then get the hash of the original erasure share from the ContainmentDB.

  shareData := make([]byte, shareSize)
  _, err = io.ReadFull(downloader, shareData)

  // Create a hash of shareData.

  var successNodes storj.NodeIDList
  var failNodes storj.NodeIDList

  if !bytes.Equal(shareHash, pendingAudit.expectedShareHash) {
    failNodes = append(failNodes, node)
    auditRecords.FailNodeIDs = failNodes
  } else {
    successNodes = append(successNodes, node)
    auditRecords.SuccessNodeIDs = successNodes
  }

  // Remove the set the contained flag to false on the NodeDossier and update the overlay (satellitedb).
  // Delete the pending audit from the ContainmentDB.
  ...
```

#### 4. Reverify should directly report any successful or failed audits to the StatDB.
This will require a refactor because the audit system's existing `reporter` is currently in charge of recording all audits, and it's only accessible at the level of `pkg/audit/service.go`.
I think we'll want to call `RecordAudits` from inside the Reverify function, so that the Verify function (which calls Reverify) won't have to wait on Reverify's results to package with other audit results.

## Implementation (Stories)
- Create ContainmentDB table
- Identify errors from connecting with nodes, determine who needs to be contained
- Implement ContainmentDB interface
- Make sure `pendingAudit` entries associated with deleted segments are also deleted from ContainmentDB
    - (or that their `pendingAudit` info gets cleared from their NodeDossier)
- Make the audit system Reverify contained nodes and save audit results
- Prevent nodes from backing up all their data to Glacier and only getting that data and responding to the satellite
    - Keep track of an overall speed for nodes using repairs
    - Establish a minimum speed for node selection, since we won't get a high ratio of node speed if they're doing the Glacier thinng

## Closed Issues

Q: How often can a contained node expect to be reverified in a real-life system?

A: It should be checked a few times per day.

Q: Why create yet another database table when we never iterate through all records in ContainmentDB?
Couldn't the information on the `pendingAudit` struct just be added to the NodeDossier?

A: The advantage of the containmentDB table is that it's separate from the existing nodes table. If the nodes table got messed up somehow, the containmentDB wouldn't be affected.
Additionally, adding several more columns to the nodes table could make performance worse.
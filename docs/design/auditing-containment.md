# Auditing and Node Containment

## Abstract

This design doc outlines how we will implement "containment mode," in which the auditing service monitors a node that has accepted an audit request, but refuses to send the requested data.
Currently in the codebase, nodes that do this will be marked as having audit failures.
However, we don't want to mark nodes as offline or immediately mark an audit failure if this specific case arises.
Instead, we want to "contain" these nodes by retrying the original audit until they either eventually respond and pass the audit, or refuse to respond a certain number of times and ultimately receive an audit failure mark.

## Main Objectives
1. Nodes won’t be able to “escape” audits for specific data by refusing to send requested data.
2. The audit service should not audit contained nodes for other data, and should verify their originally requested data.
3. The overlay service should not select contained nodes.
4. A ContainmentDB holds metadata required to verify contained nodes' originally requested data.

## Background

The whitepaper section 4.13 talks about containment mode as follows:
```
Given a specific storage node, an audit might reveal that it is offline or incorrect. In the case of a node being offline, the audit failure may be due to the address in the node discovery cache being stale, so another, fresh Kademlia lookup will be attempted. If the node still appears to be offline, the Satellite places the node in containment mode. In containment mode, the Satellite will calculate and save the expected response, then continue to try the same audit with that node until the node either responds successfully, actively fails the audit, or is disqualified from being offline too long. Once the node responds successfully, it leaves containment mode. All audit failures will be stored and saved in the reputation system. 
```
However, we're departing from the whitepaper because offline nodes will not be moved to containment mode.
They will just be marked as offline.
It's only when nodes initially respond to the audit service's dial but then refuse to send the requested erasure share that they are moved to containment mode.

The node will be given a `contained` flag, and if the audit service attempts to audit the node again, it will see this flag and request the same share it had originally requested.
If the storage node fails to send this share again, it will have its `reverifyCount` incremented.

If the `failedReverifyCount` exceeds a `reverifyLimit`, then the node will be marked as failing the audit and removed from containment mode.
Otherwise, the node will pass the audit and will also be removed from containment mode.

Additionally, contained nodes will be excluded from node selection.

## Design
New functionality needs to be added to the audit verifier, where upon receiving a pointer, the verifier should first iterate through the nodes listed in the pointer and check if they have a `contained` field set to true on their NodeDossier or not.

For nodes that are not contained, the audit will continue as normal.

For nodes that are contained, the verifier will ask the ContainmentDB for the hash of the erasure share expected from the node when it was originally audited.

The verifier will then attempt to download that erasure share from the contained node.
From there, the following cases can occur:
- Passes audit ->  `pendingAudit` removed from the ContainmentDB  -> stats updated in overlaycache/satelliteDB
- Fails audit -> `pendingAudit` is removed from the ContainmentDB -> stats updated in overlaycache/satelliteDB
- Refuses to respond again -> the `pendingAudit`'s `reverifyCount` is incremented in the ContainmentDB ->
    - -> If that count exceeds the reverify limit, then the node’s stats will be updated to reflect an audit failure and the node will be removed from the ContainmentDB
    - -> If the count does not exceed the reverify limit, the `pendingAudit` will remain in the ContainmentDB

```
type pendingAudit struct {
    nodeID       storj.NodeID
    pieceID      storj.PieceID
    pieceNum     int
    stripeIndex  int
    shareSize    int64
    expectedShareHash []byte
    reverifyCount   int // number of times reverify has been attempted
}

type ContainmentDB interface {
  Contain(ctx context.Context, pendingAudit pendingAudit) error
  Get(ctx context.Context, nodeID pb.NodeID) error
  ReverifyFail(ctx context.Context, nodeID pb.NodeID) error
  ReverifySuccess(ctx context.Context, nodeID pb.NodeID) error
}
```

`pendingAudit` entries are created and added to the ContainmentDB when a node seems to be online and opens a connection with the satellite to be audited (this happens within the existing Verify function in the audit package), but then refuses to send the requested erasure share.

Additionally, the satellite should make sure to empty the ContainmentDB of certain pending audits when segments are deleted.

## Rationale

Originally I had the idea to have a separate service iterate through and check the ContainmentDB.
However, after talking with JT it seemed unnecessary to have another service for this functionality that could be added to the existing verifier code.
I think we also don't want to pound the contained nodes with reverification audits.
They should just be reverified whenever they pass through the audit service normally.

## Implementation

#### 1. Uncooperative nodes are documented in the ContainmentDB:

By "uncooperative," I mean that the auditor will be able to successfully DialNode (within pkg/audit/verifier.go), but errors will occur at
```
downloader, err := ps.Download(timedCtx, limit.GetLimit(), offset, int64(shareSize))
```
or
```
_, err = io.ReadFull(downloader, buf)
```
Once these errors have occurred and we know that a `pendingAudit` entry should be made in the ContainmentDB, we should use infectious’s `f.Decode` to generate the original stripe.
Then we should use infectious’s `f.EncodeSingle` where we input the stripe and output the missing share. We then make a hash of that missing share to be saved to the ContainmentDB as `ExpectedShareHash`.

`containment.Contain` should be called when a node is found to be “uncooperative."

``` pkg/audit/verifier.go
verifier.containment.Contain(ctx, &pendingAudit{
    nodeID:            nodeID,
    pieceID:           pieceID,
    pieceNum:          pieceNum,
    stripeIndex:       stripeIndex,
    shareSize:         shareSize,
    expectedShareHash: shareHash,
}
```

#### 2. Audits continue normally. But now the nodes listed in a pointer are checked for `contained` status.
``` pkg/audit/containment/checker.go
func (verifier *Verifier) Verify(ctx context.Context, stripe *Stripe) (verifiedNodes *RecordAuditsInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	pointer := stripe.Segment

	pieces := pointer.GetRemote().GetRemotePieces()
	
	// get the NodeDossiers from the overlay using the pieces’ nodeIDs
  // determine if they have the contained flag set to true
  ...
```

#### 3. Contained nodes are checked for the same data that they were originally requested to respond with:

```
func (verifier *Verifier) Reverify(ctx context.Context, node storj.NodeID) (auditRecords *RecordAuditsInfo, err error) {

  pendingAudit, err := checker.containment.Get(id)

  // dial the node, then use info from the pendingAudit to download the target share from the node

  offset := node.stripeIndex * node.shareSize
  downloader, err := ps.Download(timedCtx, limit.GetLimit(), offset, int64(shareSize))

  // if the error is a timeout (or any other error?), call ReverifyFail
  // if the download occurred successfully, then get the hash of the original erasure share from the ContainmentDB

  shareData := make([]byte, shareSize)
  _, err = io.ReadFull(downloader, shareData)

  // create a hash of shareData

  var successNodes storj.NodeIDList
  var failNodes storj.NodeIDList

  if !bytes.Equal(shareHash, pendingAudit.expectedShareHash) {
    failNodes = append(failNodes, node)
    auditRecords.FailNodeIDs = failNodes
  } else {
    successNodes = append(successNodes, node)
    auditRecords.SuccessNodeIDs = successNodes
  }

  // remove the set the contained flag to false on the NodeDossier and update the overlay (satellitedb)
  // remove the pending audit from the ContainmentDB

  return auditRecords
```

## Open Issues

#### Why not audit a contained node for other data?
If a node is storing lots of pieces and goes offline or refuse to respond to audits, it should be audited for all the segments that were selected by audit service. What would be the reason for not doing this?

If the fear is that the node would pass audits for other data and have a higher audit success ratio, the node could also gain a higher success ratio when set free from other audits and continuing to refuse to respond to just one audit.

#### How do we specifically determine if a node should be "contained" or marked as offline?
We can't verify that just because DialNode failed that a node is offline. There are many other reasons for DialNode to fail, not only that the target is not reachable (offline). For example, the node can check that the incoming connection is from a satellite and just refuse the connection. This is not the same as being offline.

What is a better way to check and distinguish between an "uncooperative node" and an "offline node"?
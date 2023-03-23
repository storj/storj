# Node and operator certification

## Abstract

This is a proposal for a small feature and service that allows for nodes and
operators to have signed tags of certain kinds for use in project-specific or
Satellite-specific node selection.

## Background/context

We have a couple of ongoing needs:

 * 1099 KYC
 * Private storage node networks
 * SOC2/HIPAA/etc node certification
 * Voting and operator signaling

### 1099 KYC

The United States has a rule that if node operators earn more than $600/year,
we need to file a 1099 for each of them. Our current way of dealing with this
is manual and time consuming, and so it would be nice to automate it.

Ultimately, we should be able to automatically:

1) keep track of which nodes are run by operators under or over the $600
   threshold.
2) keep track of if an automated KYC service has signed off that we have the
   necessary information to file a 1099.
3) automatically suspend nodes that have earned more than $600 but have not
   provided legally required information.

### Private storage node networks

We have seen growing interest from customers that want to bring their own
hard drives, or be extremely choosy about the nodes they are willing to work
with. The current way we are solving this is spinning up private Satellites
that are configured to only work with the nodes those customers provide, but
it would be better if we didn't have to start custom Satellites for this.

Instead, it would be nice to have a per-project configuration on an existing
Satellite that allowed that project to specify a specific subset of verified
or validated nodes, e.g., Project A should be able to say only nodes from
node providers B and C should be selected. Symmetrically, Nodes from providers
B and C may only want to accept data from certain projects, like Project A.

When nodes from providers B and C are added to the Satellite, they should be
able to provide a provider-specific signature, and requirements about
customer-specific requirements, if any.

### SOC2/HIPAA/etc node certification

This is actually just a slightly different shape of the private storage node
network problem, but instead of being provider-specific, it is property
specific.

Perhaps Project D has a compliance requirement. They can only store data
on nodes that meet specific requirements.

Node operators E and F are willing to conform and attest to these compliance
requirements, but don't know about project D. It would be nice if Node
operators E and F could navigate to a compliance portal and see a list of
potential compliance attestations available. For possible compliance
attestations, node operators could sign agreements for these, and then receive
a verified signature that shows their selected compliance options.

Then, Project D's node selection process would filter by nodes that had been
approved for the necessary compliance requirements.

### Voting and operator signaling

As Satellite operators ourselves, we are currently engaged in a discussion about
pricing changes with storage node operators. Future Satellite operators may find
themselves in similar situations. It would be nice if storage node operators
could indicate votes for values. This would potentially be more representative
of network sentiment than posts on a forum.

Note that this isn't a transparent voting scheme, where other voters can see
the votes made, so this may not be a great voting solution in general.

## Design and implementation

I believe there are two basic building blocks that solves all of the above
issues:

 * Signed node tags (with potential values)
 * A document signing service

### Signed node tags

The network representation:

```
message Tag {
    // Note that there is a signal flat namespace of all names per
    // signer node id. Signers should be careful to make sure that
    // there are no name collisions. For self-signed content-hash
    // based values, the name should have the prefix of the content
    // hash.
    string name = 1;
    bytes value = 2; // optional, representation dependent on name.
}

message TagSet {
    // must always be set. this is the node the signer is signing for.
    bytes node_id = 1;

    repeated Tag tags = 2;

    // must always be set. this makes sure the signature is signing the
    // timestamp inside.
    int64 timestamp = 3;
}

message SignedTagSet {
    // this is the seralized form of TagSet, serialized so that
    // the signature process has something stable to work with.
    bytes serialized_tag = 1;

    // this is who signed (could be self signed, could be well known).
    bytes signer_node_id = 3;
    bytes signature = 4;
}

message SignedTagSets {
    repeated SignedTagSet tags = 1;
}
```

Note that every tag is signing a name/value pair (value optional) against
a specific node id.

Note also that names are only unique within the namespace of a given signer.

The database representation on the Satellite. N.B.: nothing should be entered
into this database without validation:

```
model signed_tags (
    field node_id            blob
    field name               text
    field value              blob
    field timestamp          int64
    field signer_node_id     blob
)
```

The "signer_node_id" is worth more explanation. Every signer should have a
stable node id. Satellites and storage nodes already have one, but any other
service that validates node tags would also need one.
In particular, the document signing service (below) would have its own unique
node id for signing tags, whereas for voting-style tags or tags based on a
content-addressed identifier (e.g. a hash of a document), the nodes would
self-sign.

### Document signing service

We would start a small web service, where users can log in and sign and fill
out documents. This web service would then create a unique activation code
that storage node operators could run on their storage nodes for activation and
signing. They could run `storagenode activate <code>` and then the node would
reach out to the signing service and get a `SignedTag` related to that node
given the information the user provided. The node could then present these
to the satellite.

Ultimately, the document signing service will require a separate design doc,
but here are some considerations for it:

Activation codes must expire shortly. Even Netflix has two hours of validity
for their service code - for a significantly less critical use case. What would
be a usable validity time for our use case? 15 minutes? 1 hour? Should we make
it configurable?

We want to still keep usability in mind for a SNO who needs to activate 500
nodes.

It would be even better if the SNO could force invalidating the activation code
when they are done with it.

As activation codes expire, the SNO should be able to generate a new activation
code if they want to associate a new node to an already signed document.

It should be hard to brute-force activation codes. They shouldn't be simple
numbers (4-digit or 6-digit) but something as complex as UUID.

It's also possible that SNO uses some signature mechanism during signing service
authentication, and the same signature is used for activation. If the same
signature mechanism is used during activation then no token is necessary.

### Update node selection

Once the above two building blocks exist, many problems become much more easily
solvable.

We would want to extend node selection to be able to do queries,
given project-specific configuration, based on these signed_tag values.

Because node selection mostly happens in memory from cached node table data,
it should be easy to add some denormalized data for certain selected cases,
such as:

 * Document hashes nodes have self signed.
 * Approval states based on well known third party signer nodes (a KYC service).

Once these fields exist, then node selection can happen as before, filtering
for the appropriate value given project settings.

## How these building blocks work for the example use cases

### 1099 KYC

The document signing service would have a KYC (Know Your Customer) form. Once
filled out, the document signing service would make a `TagSet` that includes all
of the answers to the KYC questions, for the given node id, signed by the
document signing service's node id.

The node would hang on to this `SignedTagSet` and submit it along with others
in a `SignedTagSets` to Satellites occasionally (maybe once a month during
node CheckIn).

### Private storage node networks

Storage node provisioning would provide nodes with a signed `SignedTagSet`
from a provisioning service that had its own node id. Then a private Satellite
could be configured to require that all nodes present a `SignedTagSet` signed
by the configured provisioning service that has that node's id in it.

Notably - this functionality could also be solved by the older waitlist node
identity signing certificate process, but we are slowly removing what remains
of that feature over time.

This functionality could also be solved by setting the Satellite's minimum
allowable node id difficulty to the maximum possible difficulty, thus preventing
any automatic node registration, and manually inserting node ids into the
database. This is what we are currently doing for private network trials, but
if `SignedTagSet`s existed, that would be easier.

### SOC2/HIPAA/etc node certification

For any type of document that doesn't require any third party service
(such as government id validation, etc), the document and its fields can be
filled out and self signed by the node, along with a content hash of the
document in question.

The node would create a `TagSet`, where one field is the hash of the legal
document that was agreed upon, and the remaining fields (with names prefixed
by the document's content hash) would be form fields
that the node operator filled in and ascribed to the document. Then, the
`TagSet` would be signed by the node itself. The cryptographic nature of the
content hash inside the `TagSet` would validate what the node operator had
agreed to.

### Voting and operator signaling

Node operators could self sign additional `Tag`s inside of a miscellaneous
`TagSet`, including `Tag`s such as

```
"storage-node-vote-20230611-network-change": "yes"
```

Or similar.

## Open problems

* Revocation? - `TagSets` have a timestamp inside that must be filled out. In
  The future, certain tags could have an expiry or updated values or similar.

## Other options

## Wrapup

## Related work

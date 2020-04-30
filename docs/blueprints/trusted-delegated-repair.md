# Delegating Repair Work Outside The Satellite

## Abstract

At the time of this writing, repair work is all performed by the satellite directly. This has some drawbacks. Most notably, (1) repair throughput is limited by the network resources of the satellite, and (2) satellites with high-uptime network connections are using those same connections for repair traffic, which leads to untenably high egress traffic costs. We want to be able to delegate repair work to workers outside the satellite's immediate trust boundary, either to a set of servers managed by the same satellite operations staff in a different datacenter, or to storage nodes who have computing power to spare and want to be paid for the work.

## Background

Performing repair work on the satellite is the simplest configuration in terms of security and code complexity. Workers are simply goroutines in one of the satellite processes, and have access to the satellite's own identity keys. Repair results can be immediately trusted, destination storage nodes can be easily selected, and PUT orders can be trivially signed.

However, this is leading to excessively high costs per byte repaired. With the current configuration on a Tardigrade satellite being run in GCP, we pay between $0.08 and $0.23 in egress costs per gigabyte of repaired pieces (depending on GCP region, monthly usage totals, and geographic destination). A theoretical satellite being run outside of GCP may have a lower bandwidth cost, but might not be able to complete repair work fast enough (even with a large number of repair workers) due to having all repair traffic flow through the bottleneck of the satellite's own limited network drops.

Delegating repair work to servers hosted elsewhere will save on bandwidth costs. These servers do not need a network connection with high uptime SLAs, like the satellite API servers do; repair servers might be able to do their job successfully even if they experience a brief network outage every day. It is a fundamentally different class of network traffic, so satellite operators should not need to pay as much for it as they would for the highly-available API traffic.

Delegating repair work to storage nodes will have similar cost benefits, but will also come with the extra benefit of not requiring the maintenance and administration of the additional servers. Also, this would decentralize repair further, allow storage node operators to earn more, and let storage node operators participate more in the health of the network.

This blueprint contemplates the former approach: delegating repair work to servers hosted elsewhere. We will refer to this as "_trusted delegated repair_" or _TDR_, to distinguish it from delegating repair work to storage nodes ("_foreign repair_" or _FR_). We want to allow for foreign repair at some point, and this blueprint should keep that direction open, but it may be some time before we can implement that.

## Design

### Requirements

Repair workers must be able to download existing pieces directly, perform reed-solomon repair while verifying correctness (checking signed hashes or doing error-correction), and upload repaired pieces to appropriate new storage nodes directly, without any of the download or upload traffic passing through the satellite. The satellite can still coordinate repair work.

If repair workers take too long to complete a job, it should expire and the satellite may re-issue it to another worker. If the original job completes after the expiration time, the satellite will reject it.

Ideally, we want to limit the damage that could result from a compromised repair worker, but that is not a primary consideration of this blueprint.

Repair workers must not contact the satellite databases directly. They should communicate with the satellite using the specified repair worker API only.

### Outline

Satellite will:

* identify repair work needed
* authenticate external repair workers
* issue jobs to authorized workers
* select destination storage nodes for repaired pieces
* sign the appropriate GET and PUT orders
* reissue jobs that were not completed before expiration time
* reject submitted repair results that took too long
* keep track of pointer contents at time of repair job start, and discard repair results if pointer has changed

Repair workers will:

* request jobs from the satellite
* download existing pieces for the segment to be repaired
* perform RS+EC repair
* upload the new pieces to the designated receiving storage nodes
* report results to satellite

### Details

#### Messages

Repair job objects sent to workers will include:

* some unique identifier for the job, possibly uuid
* a set of signed GET orders for all believed-healthy pieces to be downloaded (the repair worker is expected to use only as many as necessary)
* signed PUT orders for all new pieces to be written, along with the PieceNum to use with each
  * one order for all possible pieces other than healthy ones in the segment's pointer
* the expiration time of the job

Repair job result objects sent back by workers include:

* the job identifier
* a list of ``RemotePiece`` messages, containing PieceHashes signed by storage nodes, as with normal PUTs done by uplinks
* a list comprised of all PUT orders that were used in storing new pieces (these can simply be copied from the repair job input message)
* a list of ``(PieceNum, NodeID)`` pairs, indicating pieces which should be _removed_ from the pointer. This will include pieces for which the expected owning storage node returned a "not found" error, as well as pieces which were downloaded but failed their validation check.

#### Satellite-side state

The satellite will need to maintain some information about pending repair jobs. This should include, at least:

* the job identifier
* the nodeID of the repair worker to which the job was assigned
* the expiration time of the job
* the path to the segment being repaired
* a copy of the serialized pointer for the segment being repaired, at the time the repair job was issued

#### Authorization

Authorization is pretty simplistic here. The satellite can simply check whether the repair worker's nodeID is on a configured whitelist.

#### Handling concurrent changes to segments under repair

The repair coordinator will keep track of the pointer that was current when a repair job was issued. If a repair job completes and the pointer for that segment has changed in the meantime, the repair results will be discarded.

#### Dealing with repair workers which fail to complete jobs

A time limit will be imposed on all repair jobs, as adjudicated by the repair coordinator's clock. If a repair job takes longer than the specified amount of time, it will be rejected. After a job is expired, the repair coordinator may hand the same job out to a different worker. (This is probably best implemented by simply placing the job back in the repair queue.)

From the repair worker's point of view, if the expiration time is reached, it should abandon work on the current job. If any new pieces are already uploaded, they will be left in place until garbage collection cleans them up. After this blueprint is implemented, we may want to add code that takes care of deletion of the spurious pieces.

Because it is better to have _some_ new pieces uploaded than to have _no_ new pieces, repair workers should have their own time limit, occurring before the job expiration time by a configurable interval (e.g. 5 minutes). If that time limit is reached, the worker will abandon progress on pending uploads and submit its job with however many new pieces have been uploaded already.

#### Privacy concerns

There should be no privacy concerns introduced by these changes. Foreign repair workers will only be able to see piece hashes, piece nums, and IDs and IPs of other storage nodes, but none of that is considered sensitive or secret.

#### Metrics

Among the values that should be monitored in the new code are the following:

* How long repair jobs take from the worker point of view
* How long repair jobs take from the coordinator point of view
* How frequently repair jobs reach the cutoff timeout before the desired number of new pieces are uploaded
* How frequently repair jobs reach the expiration time
* How many new pieces are uploaded by repair workers per job
* How many piece removals are reported by repair workers per job
* How frequently repair jobs are discarded because the pointer changed
* How many newly written pieces are abandoned because a job reached the expiration time (best effort; a worker may not be able to determine whether its job submission was received before the expiration time or not)

## Rationale

The chief alternative approaches, as far as I can tell, are:

1. Continue to perform repair on the satellite, as we have been doing. This has the problem of network bottlenecking and high costs, as discussed above.

2. Don't do repair at all. Our modeling suggests this would lead to data loss with near-100% certainty, and is not a real choice if we want a sustainable, reliable business.

3. Extend the trust boundary of the satellite beyond its host datacenter to include a host or hosts in another datacenter (by VPN, for example) and run repair workers there exactly as we do now. These repair workers would contact the satellite database directly and otherwise act as full-fledged satellite processes. This would have the benefit of requiring no code changes and could possibly be deployed sooner, saving some money. It is not clear to me what the operational impact of this would be; probably we would treat the external host(s) as a fully separate unit for deployment and monitoring purposes, despite being part of a satellite living elsewhere. Setting up secure peering and database authentication between the networks need not be too terribly complicated. However, I think that the code changes required to achieve the approach discussed in this blueprint will be pretty minimal. Repair workers already act as separate processes and get their work from a queue; we would mostly just need to change the communication mechanism by which they do so. If the expectation of fairly minimal code changes holds true, then that plus the security benefit and operational simplicity of _not_ extending the satellite trust boundary past the datacenter boundary seem to me to justify the approach in this blueprint. That is only speculation, though.

4. Delegate repair to entities outside the control of the satellite admins: e.g., storage nodes. Storage nodes cannot be trusted implicitly to perform repairs correctly, as are the servers proposed in this blueprint, but verification of repair work could be made possible through duplication of effort. If our chances of catching a flawed repair from a worker are high enough, and the penalty to a failed repair is high enough, this strategy should lead to sufficient accuracy in repair. However, this strategy is far more complicated, and we need a solution sooner than we would be able to make this happen. Therefore, Trusted Delegated Repair is preferable at this point. Still, Foreign Repair is being considered in a different blueprint.

## Implementation

1. Implement a standalone repair worker process which can run outside of a satellite environment.

2. (May be done in parallel with step 1.) Create a new satellite process similar to the API process. Add a DRPC service exposing the GetRepairWork and SubmitRepairWork interfaces. Add a config item controlling whether satellites still perform their own repair. (Satellites with these interfaces will continue to perform their own repair until external repair workers are deployed and tested and ready to take over.)

3. (May be done in parallel with steps 1 and 2.) Research to determine where would be a good place to host the external repair workers. Set up a managed environment so that repair workers can be easily deployed when ready.

4. Once satellites are deployed with the new interfaces, and the repair worker code is ready, and the environment for the repair workers is ready, enable the repair workers. Create node identities for the workers and add their IDs to the whitelist for trusted delegated repair. Start up the workers and let them begin to take repair jobs. Check their results, their performance, and their bandwidth usage to be sure we're getting the expected benefits before switching to them entirely.

5. Disable satellite-side repair, allowing the delegated repair workers to perform all repairs as needed.

## Wrapup

The durability team (specifically, Paul Cannon if possible) will archive the blueprint when completed. We don't see any current documentation at Storj Labs which would be especially invalidated by this change. New documentation should be generated, explaining how the repair workers are set up and managed, and how they communicate.

# Partial rollout automation

## Abstract

This is a proposal to partially automate some of the version rollout process.
The version rollout process right now, for better or worse, is taking manual
action every 5%. This is bad so we want to fix it.

## Background/context

We have a bunch of different problems around safely keeping storage nodes
up to date in a safe way. We want to:

 * Make sure that storage nodes are not running stale or old code.
 * Make sure that new code doesn't break many nodes at once.

These two goals are at a bit of a tension - if we automatically update all nodes
immediately, then we risk an update breaking a significant portion of the
network.

We landed on this design:
https://github.com/storj/storj/blob/main/docs/blueprints/storage-node-automatic-updater.md
This design allows us to control what percent of the network is eligible for an
update. The design of this system intended to have an exponentially increasing
rollout scheme, where storage node updates would upgrade a small percentage
of the network, maybe 5%, and then see what happens. If that 5% looked good,
then maybe we'd double it, and so on, until the network was upgraded.

The upside to this exponential scheme is that humans are only involved at growth
points just to double check that the network hasn't fallen over. This is an
important feature and something we want to preserve.

The downside of this scheme is that the later stages of the rollout process
potentially involve lots of nodes upgrading at the same moment. Because storage
node operators are often eager to get the latest update when it is available
to them, we potentially risk having half or more of the network down for an
otherwise safe upgrade at a time.

The data science team suggested we don't do an upgrade increment larger than 5%
every 6 hours.

In practice this means that we are now pushing the rollout along at 5%
increments every rollout, and it's tiring.

## Design and implementation

The intention of the below design is to resolve the data science team's
concern, while still allowing us to do exponentially increasing rollouts.
This design still intentionally requires multiple PRs per rollout, but
instead of one every 5% (20 PRs), we would have just 5 (or maybe
slightly less, but not 1):

 * a PR to start a 6.25% rollout,
 * then a PR for a 12.5% rollout,
 * then a PR for a 25% rollout,
 * then a PR for a 50% rollout,
 * then a PR for a 100% rollout.

The reason for continuing to have more than 1 PR is because we get
valuable feedback both from dashboards about the network and from
the community about how the rollout is doing. We have often stopped
rollouts due to issues discovered by the community or by degraded network
behavior. We would like the default to be that we check first before
we continue the rollout, and not blindly roll it through.

Here is how we will make this work:

Currently, version.storj.io's service takes a configuration for each process
type under management. For each process, the configuration needed is:

 * The minimum required version
 * The suggested version (for the rollout)
 * A rollout seed (see [this design doc](https://github.com/storj/storj/blob/main/docs/blueprints/storage-node-automatic-updater.md) for details)
 * and a target percentage for the rollout

We will be adding two new fields:

 * a global "safe rate" value, perhaps the 5% every 6 hours thing.
 * the prior percentage for the rollout

When the process serving version.storj.io starts, it will look at its configuration
and keep track of the time since the process started. Whenever a request comes in,
it will calculate the current percent using linear interpolation on the prior
percentage, the target percentage, the time since process start, and the rate.
It will then use that to calculate the rollout cursor.

With the above change, the first rollout would be 6.25%, but
then we would only need to push updates every doubling, while not running afoul
of bumping the cursor too much every 6 hours.

## Other options

One downside with the above approach is it is fairly dependent on the process
runtime. Process restarts will restart that phase of the rollout. However,
the benefit we get with the above design is that the version servers remain
stateless. When a process restarts, what will happen is that that phase of the
configured rollout will start over, on account of the time since process start
clock effectively starting over. If the process restarted and the rollout was
halfway through the 25% to the 50% phase, it will start back at 25% and continue
to 50%. While this isn't a "feature" we would choose, this allows us to remain
purely stateless, and only require the configuration file at the time of 
redeployment. We do not need databases of any kind for this design to work. 
This is a massive benefit in terms of operational simplicity and potential
failure modes.

We could add a start timestamp to the above config, but then merging
configuration updates require getting reviews and merges before time deadlines
which sounds pretty annoying.

If we are okay adding state to the version server (such as a small database)
then many other options are on the table.

We could have the version server keep track of the current rollout so that
process restarts don't defeat it, but even better, we could get rid of needing
Git commits entirely. If the version server kept state or had a database it
could write to, then we could have an admin interface and manage rollout
status entirely through the version server itself and skip pull requests
entirely.

## Wrapup

## Related work

https://github.com/storj/storj/blob/main/docs/blueprints/storage-node-automatic-updater.md

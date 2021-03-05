# Layer 2 support for SNO payouts

## Context

Recently, the real USD value of per-ERC20 transactions has skyrocketed, both
in terms of GWEI per transaction, but also in terms of the price of ETH.

Like every cryptocurrency, Ethereum's community has been investigating approaches
to scaling transactions, both in reducing cost and increasing throughput.

In the last year, zkRollup-based layer 2 scaling approaches have shown a great
deal of promise. By using SNARKs (or PLONKs), zkRollup-based approaches are able
to support >8000 TPS while still maintaining all of the security guarantees of
the underlying layer 1. [zkSync](https://zksync.io/faq/tech.html) in particular is
a great, usable example. It supports ERC20 tokens, user-driven layer 2 deposits
and withdrawals via an API or a friendly web interface, and can even support
initiating withdrawal to a contract address or other address not explicitly
registered or usable via zkSync.

We're excited about zkRollup (and zkSync in particular) and want to start using
it to dramatically lower transaction costs and improve user experience. With
zkRollup, we may even eventually be able to consider more frequent payouts than
once a month.

Because zkSync is early technology and still has a few rough edges, we don't
want to force uncomfortable users to use it at this time, so we want to give
SNOs the ability to opt in to new layer 2 options.

### Current Payouts system

Our current payouts system has two major components, the satellite and the
CSV-driven payouts pipeline.

The satellite is responsible for generating monthly reports we call
compensation invoices, which are CSVs with the following fields:

```
period,node-id,node-created-at,node-disqualified,node-gracefulexit,node-wallet,
node-address,node-last-ip,codes,usage-at-rest,usage-get,usage-put,
usage-get-repair,usage-put-repair,usage-get-audit,comp-at-rest,comp-get,comp-put,
comp-get-repair,comp-put-repair,comp-get-audit,surge-percent,owed,held,disposed,
total-held,total-disposed,paid-ytd
```

We then pass this CSV to an internal set of tools called the `accountant` which
are responsible for checking these nodes' IP and wallet addresses against export
restrictions and a few other things and ultimately transforming the above data
into a small, two column spreadsheet with just addresses and USD amounts we
should transfer.

```
addr,amnt
```

Once these payouts have been processed, we generate a CSV of receipts (links
to settled transactions hashes) and reimport this data back into the satellite
for that period.

For us to use a different solution than layer 1 transfers, we need to extend
the above pipeline to:

 * indicate which transactions should happen on layer 2
 * indicate what type of receipt a receipt is

## Design goals

* Whenever a SNO configures a wallet address, we want that SNO to be able to
  additionally flag what features that wallet address supports (initially,
  opt-in zkSync support, but potentially more in the future).
* The satellite should keep track of per-node wallet addresses along with what
  features the wallet supports.
* Two storage nodes that share the same wallet address will not necessarily
  have the same feature flags (we want to support a SNO choosing to experiment
  with zkSync on one node but not on another).
* Transaction references to completed payouts should indicate which technology
  was used with that transaction, so zkSync transactions can be displayed in the
  SNO dashboard differently than layer 1 transactions.

## Implementation notes

 * Matter Labs has already provided us with a tool that processes CSVs of the
   form `addr,amnt` and will generate receipts in our format, so the actual
   integration with zkSync is already done for our pipeline. We only need to
   know when to use their tool vs ours.
 * Docker storage nodes currently configure wallet addresses with an environment
   variable. We should configure supported wallet technologies alongside this
   environment variable. For example:
   WALLET="0x..." WALLET_FEATURES="zksync,raiden"
   The storage node should confirm that the list of wallet features is a
   comma-separated list of strings.
 * Windows configuration is obviously different. Each platform needs a way to
   have a user add support for some wallet features. We can start off with
   config file only and add UI features soon thereafter.
 * This list of wallet features should be sent through the contact.NodeInfo
   struct to the Satellite, and should be stored on the Satellite in the nodes
   table.
 * We want this column to be outputted per node during the generate-invoices
   subcommand of the compensation subcommand of the satellite, so
   "wallet-features" will need to be added to the invoices CSV.
 * Our accountant pipeline currently aggregates all payments to a single
   wallet address. We'll need to change our accountant pipeline to output
   a different CSV per transaction method (zkSync vs normal layer 1). This
   means that if a user has the same wallet on two nodes, but only one node
   opts-in to zkSync, then that wallet will get two payouts, one with zkSync
   and one without. We will only aggregate within a specific method.
   In a scenario where an operator has three nodes, one with no wallet features,
   one with the `zksync` wallet feature, and one with the `zksync,raiden`
   wallet features, it will be up to the `accountant` tool to choose whether
   or not the third node gets a payout via Raiden or zkSync based on what
   we prefer. If the `accountant` tool prefers `zksync` over `raiden`, then
   the operator will get two transactions: one layer 1, and one combined `zksync`
   payout. If the `accountant` tool prefers `raiden` over `zksync`, then that
   operator would get three transactions.

## Future compatibility and plans

 * We want zkSync to be opt-in for now, but we expect at some future point to
   be opt-out when zkSync and our community are ready.
 * Even though zkSync is opt-in for now, we want it to be prominent, in that
   we want to encourage people to use it if they are willing.
 * If at some point we decide we want to add a new wallet feature (e.g.
   "plasma"), we should not require storage node or satellite code changes to
   get that wallet feature indication out of the compensation CSV.

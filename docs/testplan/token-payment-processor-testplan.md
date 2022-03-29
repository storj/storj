# Token Payment Processor Testplan

&nbsp;

## Background
This testplan is going to cover the token payment processor, it will go over must haves and additional features that we can add on later. 

UI relating to token payment processing can be seen on [Billing Page](https://www.figma.com/file/HlmasFJNHxs2lzGerq3WYH/Satellite-GUI-Public?node-id=11080%3A68109) figma design.

&nbsp;

&nbsp;

<google-sheets-html-origin><!--td {border: 1px solid #ccc;}br {mso-data-placement:same-cell;}-->

Test Scenario | Test Case | Description | Comments
-- | -- | -- | --
Transaction | Block Chain Reorg | If there is a naturally occurring chain reorg then past transactions in the old chain are deactivated due to a chain reorganization (this should be visible in UI, pending to cancelled) | Must have early on
  | Transaction Override | If a user overrides the first transaction with a higher transaction fee, then the token payment processor has to replace the transaction hash it is watching (this should be visible in UI) | Must have early on
  | Minimum Confirmation | A transaction should not be called paid unless it meets the minimum number of confirmations reached  (this should be visible in UI, pending until it meets minimum number of confirmations) | Must have early on
  | Cancel Transaction- 0 Confirmations | If a user has a pending transaction and it has 0 confirmations, then user should be able to cancel transaction from their wallet (change should be visible on UI, Pending to cancelled) | Must have early on
  | Cancel Transaction- Double Spending | If a user has a pending transaction and it has more than or equal to 1 confirmation, then user could override first transaction with a higher transaction fee and send to themself (this should be visible in UI, Pending to cancelled) | Must have early on
  | Transaction Fee Warning | A general warning message should be shown in the UI regarding low transaction fees and explain to users that if the fee is too low then there is a possibility that the transaction will be reverted (their gas fee won't be given back) or delayed | Must have early on
  | Multiple Transactions | If a user has a pending transaction and then performs another transaction using the same address, the new transaction should be placed on hold until the previous transaction is confirmed (this hold or pending status should show on the UI for any subsequent transactions using the same address) | Must have early on
Transaction (Not required- More so for User Experience) | Pending Transaction- Mempool (User Experience) | If a user has a pending transaction that has yet to be accepted in a block, it should then be placed in the mempool | Can be implemented later
  | Confirmed Transaction- Block (User Experience) | If a user has a confirmed transaction, it should be removed from the mempool and included in a block, the user should be able to view said transaction on the UI | Must have early on
  | Incoming Transaction (User Experience) | Incoming transactions should have increased mempool priority so geth nodes wont drop it even if the transaction fee is low | Can be implemented later
  | Stuck Transaction (User Experience) | If a user submits a transaction with a very low gas fee then they will be stuck with a pending transaction, so the user should be able to clear the nonce by customizing another transaction to the same address with the said same nonce but this time with a higher transaction fee to unstuck the transaction (change should reflect in UI eventually, pending to confirmed) | Can be implemented later- User should be given warning for low transaction fee see transaction fee warning
  | Customize Nonce (User Experience) | Users should be given the option to customize nonce(for stuck transactions), but also be given a warning to use cautiously | Can be implemented later
  | Change Priority | Priority for transactions should drop to default if transactions still remain unconfirmed, due to reasons such as low transaction fee, after a set amount of time (for example - 1 week) | Can be implemented later
Alerts | Stuck Geth Node | Geth nodes can get stuck and stop processing new blocks and in this case, we wont be able to register new transactions, so for this case the token payment processor should detect the issue and alert us to fix it | Must have early on
  | Corrupted Geth Node | Geth nodes can get corrupted and get stuck in a crash loop, so for this case the token payment processor should detect the issue and alert us to fix it | Must have early on
  | Update Geth Node | Geth nodes can get outdated and this may lead to further complications, so for this case, if there are new patches or updates of geth then we should be alerted to update | Must have early on
  | Resynced Geth Node | Geth nodes should be able to sync with the network, if a geth node resyncs it can take several days and so the token payment processor should be alerted and pick up transactions during this resync | Must have early on
UI | Confirmed/Paid Transaction | Users should be able to see if a transaction is confirmed and paid for on UI | Must have early on
  | Pending Transaction | Users should be able to see if a transaction is pending on UI | Must have early on
  | Failed Transaction | Users should be able to see if a transaction failed on UI | Must have early on
  | Transaction Override UI | Users should be able to see if a transaction is succesfully overrided on UI (so user should see status go from pending-->confirmed, during the process of transaction override) | Must have early on
  | Transaction Ordered By Nonce/ View Nonce | When a user performs multiple transactions using the same address, those with a lower nonce (older transactions) are included in the blockchain first for security purposes regardless of any issue or User should be able to view Nonce | Can be implemented later
  | Balance Increase | Balance increases should be based on the first confirmation and not based on the time we process it | Must have early on (viewable from transaction details link from wallet))
  | Sender Fee | Sender fee should be obtainable from UI for said transaction | Must have early on (viewable from transaction details link from wallet))
  | Reciever Fee | Reciever fee should be obtainable from UI for said transaction | Must have early on (viewable from transaction details link from wallet))
  | Sender/Reciever Address | Sender and receiver address should be obtainable from UI for said transaction | Must have early on (viewable from transaction details link from wallet))
  | Transaction Fee | Price from gas fee, gas used from transaction etc should be obtainable from UI | Must have early on (viewable from transaction details link from wallet)
  | Conversion Rate | When user adds storj token as payment method, token should convert to USD in total balance with 10 percent bonus added on top | Token should convert to USD otherwise it can cause some problems f.e person wants to charge back when token buying power increases
  | Transaction Detail | If user makes a transaction user should be able to see ethereum scaling solution used for said transaction (view details) | Must have early on
Testing | Recreate Transactions | Using sender, receiver, nonce, and transaction fees we should be able to recreate transactions to see if there are any mempool bugs and this should also be doable using storj-up | Can be implemented later
  | Integrating Storj-scan into Storj-up | For ease of testing it would be valuable to have storj-scan built into storj-up (may need integration with satellite for us to test) | Can be implemented later
  | Storj-scan with testnet | As stated above, token payment processor has to work with testnet to be able to run in storj-up and use existing testnet geth node | Can be implemented later

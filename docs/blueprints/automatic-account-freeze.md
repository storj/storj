# Automatic Account Freezing/Unfreezing

## Abstract

This blueprint describes a way to allow user accounts to be automatically restricted after failing to pay and unrestricted after payment has been collected.

## Background

Currently, there is no incentive for a user to ensure that the payment method attached to their account has a sufficient available balance to cover data usage charges. This results in the accrual of unpaid usage charges due to insufficient funds. The solution outlined in this document is as follows: when a user has not paid usage charges for a set period of time, they should not be able to upload or download data from the Storj network. This process is known as freezing. The user is then given the opportunity to unfreeze their account by triggering collection of their outstanding invoice. This can be done manually by clicking a button in the satellite UI or automatically when a payment method is added.

When the user attempts to use their account after it has been frozen, they will receive an error message notifying them of the account freeze and informing them of the steps they must take to restore their access.

The behavior described in this document only applies to paid-tier users despite the possibility of free-tier users' segment fees exceeding the amount discounted by a free-tier coupon. What actions we must take to address that issue is a different scope of work.

## Design

A new database table will be responsible for storing account freeze events. This table should contain event ID, user ID, and event type columns. There are 2 types of events, and there should be at most 1 event of each type for each user.

* Warning: An event of this type is inserted when the user has been warned that their account is at risk of being frozen because of a low balance. If such an event exists for a user, they will not be sent any more warning emails. All warning events should be cleared at the beginning of the billing cycle so that new warning emails may be sent.
* Freeze: An event of this type is inserted to signify that the user is frozen. If there is no such event for a user, they should be considered unfrozen.

A new chore will be responsible for freezing the accounts of all unfrozen paid-tier users who have not paid within a certain time period. The chore will also send 2 types of emails: an email to warn each user whose balance is nearly depleted about impending account restriction and an email to notify a user once their account has been restricted.

Frozen users will be displayed a satellite UI banner notifying them of the restrictions on their account and a button allowing them to unfreeze their account through invoice collection. Due to our caching of usage limits, recently unfrozen users will be displayed a message in the satellite UI stating that it will take up to 5 minutes for their restored usage limits to propagate to all servers in all regions.

## Rationale

It is possible to manually decrease a user’s usage limits after unsuccessful debt collection and increase them once payment has been resolved. However, doing so places an undue burden on satellite admins, especially as the number of accounts requiring such actions increases. It is more efficient to automatically freeze and unfreeze accounts, freeing satellite admins to focus on other tasks.

## Implementation

### Phase 1 - Automatic Account Unfreezing

In this phase, account freezes are manually performed by an administrator through the satellite admin UI. Because the existing usage limit verification functionality will be used, there will be no impact on existing metainfo validation.

1. Create a migration that adds a table containing account freeze events. In addition to the columns listed in the Design section of this document, the table should contain a temporary column for holding a user's usage limits at the time they were frozen. Once the user is unfrozen, their usage limits will be restored by referencing the information in this column.
2. Create a button on the billing page that, when clicked, attempts to collect payment on a frozen user’s outstanding invoice. If this attempt is successful, the user’s account is unfrozen by restoring their limits and deleting their freeze and warning events in the account freeze events table. Otherwise, the user’s account should remain frozen and an error message should be displayed notifying them of insufficient funding.
3. Update the payment method addition procedure such that when it is invoked by a frozen user, it attempts outstanding invoice collection using the newly-added payment method and unfreezes the user’s account if this process is successful.
4. Create a banner or modal notifying a user that their account is frozen.
5. Allow satellite admins to freeze accounts and retrieve accounts in need of freezing from within the satellite admin UI. Freezing involves creating a freeze event in the account freeze event table and zeroing a user’s usage limits.

### Phase 2 - Automatic Account Freezing

In this phase, a user’s usage limits are not touched when freezing or unfreezing. Instead, the account freeze table is used to determine the frozen state of a user’s account.

1. Update satellite accounting methods to utilize an account freeze cache in a manner similar to the existing usage limit cache. File operations should be rejected regardless of a user’s usage limits if the cached data states that the user is frozen.
2. Update the invoicing procedure such that it clears all warning events from the account freeze event table. This will allow warning emails to be sent in the new billing cycle.
3. Create a chore that performs the following operations:
    1. It iterates over each open invoice whose usage period ended before the grace period, freezes the corresponding customer’s account, and sends them an email informing them of this.
    2. It iterates over each unfrozen user whose balance does not cover their impending usage charges, and if there does not exist a warning event for them, it informs them that their account is at risk of being frozen if they cannot pay their dues in time. A warning event is then inserted into the account freeze events table.
4. Restore the usage limits zeroed from the first implementation phase.
5. Update the account freezing procedure such that it does not zero usage limits. This behavior is no longer required because usage limits are not read directly when rejecting file operations.
6. Drop the column containing saved usage limits from the account freeze event table.
7. Ensure that Uplink users with frozen accounts are informed of such when attempting to interact with the Storj network in a prohibited manner.

## Wrap Up

The Integrations team is responsible for archiving this blueprint upon completion.

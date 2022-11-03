# Automatic Account Freezing/Unfreezing

## Abstract

This blueprint describes a way to allow user accounts to be automatically restricted after failing to pay and unrestricted after payment has been collected.

## Background

Currently, there is no incentive for a user to ensure that the payment method attached to their account has a sufficient available balance to cover data usage charges. This results in the accrual of unpaid usage charges due to insufficient funds. The solution outlined in this document is as follows: When a user has not paid usage charges for a set period of time, their account’s usage limits will be set to zero, revoking their access to the Storj network. This process is known as freezing. The user is then given the opportunity to unfreeze their account by triggering collection of their outstanding invoice through the satellite UI.

When the user attempts to use their account after it has been frozen, they will receive an error message notifying them of the account freeze and informing them of the steps they must take to restore their access.

## Design

A new chore should be responsible for freezing the accounts of all paid-tier users who have not paid within a certain time period.

A new database column should track whether a user’s account is frozen. Usage limits are always treated as zero as long as the value of the column is true. This value should also be used to display a banner in the satellite UI notifying the user of the restrictions on their account if present.

Once a user opts to unfreeze their account, payment of their outstanding invoice should be attempted. If it is successful, the user’s account should be automatically unfrozen.

## Rationale

It is possible to manually decrease a user’s usage limits after unsuccessful payment. However, doing so places an undue burden on service operators, especially as the number of accounts requiring such action increases. It is more efficient to automatically freeze accounts having unpaid usage charges, freeing operators to focus on other tasks.

## Implementation

1. Create a new migration that adds a column to the users table specifying whether a user’s account is frozen.
2. Update the method(s) responsible for processing usage requests such that they reject any request originating from a frozen user regardless of what the user’s usage limits are.
3. Create a button on the billing page that, when clicked, attempts to collect payment on a frozen user’s outstanding invoice. If this attempt is successful, the user’s account is unfrozen; otherwise, the user’s account should remain frozen and an error message should be displayed notifying them of insufficient funding.
4. Create a banner or modal notifying a user that their account is frozen.
5. Create a chore that iterates over all paid-tier users. Upon encountering one that has not paid within a specified grace period, that customer’s account is frozen and an e-mail is sent informing them of this.

## Wrap Up

The Integrations team is responsible for archiving this blueprint upon completion.

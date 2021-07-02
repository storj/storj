# Coupon Codes 

## Abstract

Enable users to apply a promotional code to their account so that they can automatically receive the free credit associated with that code.

Summary: When Sales/Marketing whats to run a promotion or hand out a specific amount of free credit to a group of people, we want to be able to generate a coupon code that a user could enter into their account, so that specific free credits and expiration of the credits would be automatically applied to that user’s account.

## Design

There are 2 high level aspects of this design:

### Generating Coupon Codes

This does not actually require any code changes - we can use Stripe's existing [promo codes functionality](https://stripe.com/docs/billing/subscriptions/discounts/codes). New promotional coupons can be created from the Stripe UI, then the codes can be delivered to customers through marketing.

It is the responsibility of the individual creating a new coupon to ensure that its value is prefereable to the free tier coupon - we don't want the free tier coupon to be replaced with an inferior coupon.

### Applying a Coupon Code to a User

On account creation or from the payments panel, a user should be able to insert a coupon code and immediately have the corresponding coupon applied to their account. Because Stripe only allows for one coupon per customer, the default free tier coupon will be replaced with this new coupon.

When a promotional coupon expires, we must apply the default free tier coupon to the customer.

## Rationale

The design below basically makes a call to the Stripe API when a user attempts to apply a coupon code. Because Stripe handles coupons on its own, there is no need for us to make any additional changes to the database or invoice generation logic.

## Implementation

### Service

The service layer will communicate with the Stripe API to apply a coupon code to a user. Since there is very little functionality added, this can probably just be added to the stripecoinpayments service.

It will require the following interface:

```
ApplyCouponCodeToUser(userID uuid.UUID, couponCode string) error
```

* `ApplyCouponCodeToUser` - Look up the Stripe customer corresponding to the provided user ID, then attempt to apply the coupon code to the Stripe customer. This code may be a helpful reference:
```
customerParams := &stripe.CustomerParams{}
customerParams.Coupon = new(string)
*customerParams.Coupon = couponCode 
_, err := customer.Update(c.ID, customerParams)
if err != nil {
    ...
}

```

### UI

There are a couple options we have from a UI perspective here. Either a user can create a coupon code on account creation, or add it from the payments panel in the webapp. We may even end up having both enabled or AB test one vs. the other. Either way, all that needs to be done is make a call to `service.ApplyCouponCodeToUser` when a user attempts to apply a coupon code.

### Free Tier Coupon Reapply

This is beneficial beyond coupon codes, but we need to ensure that all Stripe customers have a coupon applied to their account before invoices are generated. We may be able to verify this in one of the early invoice-generation steps. If a customer does not have a coupon (or has an expired coupon), the free tier coupon should be applied. There should already be a config value for the free tier coupon ID in the satellite config.

## Wrapup

Ensure that there is documentation on Confluence or elsewhere that describes how new coupons are created and how they should be used both by marketing/sales and users.

The Satellite Web Team will take ownership of this project and is responsible for archiving this blueprint upon completion.

## Open issues

* Can a user delete a coupon code from their account?
* Can a user replace a coupon code with another? What if the new one is inferior?

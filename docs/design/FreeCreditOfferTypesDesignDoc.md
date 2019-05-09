# Title: [Free Credit Offer Types Design Document]

## Abstract

Build trust with new users by allowing them to try out the network.
Increase viral growth of our platform through word-of-mouth marketing. 
Discover the best client referral offer type during our beta release by testing the performance of different client offering types.

**Free Tier Credit**: A free trial for all users who create an account on the Tardigrade network without using a Referral Offer link.  

**Referral Offer Credit**: An incentive program for motivating existing clients to refer their peers to set up paid accounts on the Tardigrade network. 
Existing clients give their UUID Referral Link to their friends for their friends to create an account and receive an Invite Offer Credit that is greater than the default Free Tier Credit.

## Background

### REFERRAL LINK

- When a user wants to issue a referral link:
  An existing client who has an account on one of the Tardigrade Satellites is able to retrieve their unique referral link from their dashboard. They can issue their referral link to their friends in the following ways:
  - Copy the static link associated to their account with the default invite message from their dashboard 
  Twitter Share

- When a referral link is accepted:
  The invitee can accept their invite to redeem their Referral Offer by going through the Referral Link to create an account on the Satellite they were invited to/through.
  Upon creating their account they must enter their payment information into the Stripe integration and accept/acknowledge that:
    If they use up their referral credit before the expiration date they should expect a bill for any amount of activity they acquire beyond what was credited to them
    When their offer period expires they should expect a bill for any activity on the Tardigrade network going forward billed monthly at unit per hour

### AWARDED CREDIT

- When free credit is awarded:
It is displayed as the USD value that is equivalent to the storage & bandwidth allotted amount for the credit
The credit is automatically applied to the account and will have a max limit that can be reached within a set timeframe, as well as an expiration date for it to be used by.
  The user will be charged for activity that is beyond the Free Credit limit within the set timeframe
  The user will be charged for activity that is after the expiration date.
  Example statement to the user: 
  “You will not be charged until all of your $XX credit is used or it expires. Your $XX credit will be applied immediately”
  A user's free credit can only be applied to projects that they create

  A client can see their awarded Free Credits in their dashboard with associated expiration date
  A client who has been awarded storage credit for their referrals is able to retain the awarded credits when the referral offer they received the credit under has been updated/changed

### CATEGORIES OF FREE CREDIT & OFFER TYPES

**Default Free Tier Offer:**
  - This is the credit that all new users who join the network receive if they create an account without a Referral Link. Once a new user successfully creates their account they will have 14 days to use their Free Credit allowance.  
  The Default Offer will remain constant, with no expiration date to the offer and does not have a redemption cap
  The Offer Program Admin is able to adjust the default offer
  One Default Offer per Tardigrade Satellite.
  When there is a lapse of the Referral Testing Offer/the Referral Testing Offer has expired or reached a cap, the Default Offer will be the default to the Referral Testing Offer that new users will redeem until a new Referral Testing Offer is generated.

**Referral Testing Offer**:
  - This is the credit awarded to the Invitee upon creating their account using a Referral Link. This credit can have a different expiration date than the awarded Referrer Credit and the Default Free Credit.
  An admin of the Referral Program is able to easily adjust the current offer through the  Admin Satellite GUI

**Referrer Award Credit**:
  - This is the credit awarded to the Referrer once the Invitee has created a *paid* account and has successfully paid their second invoice. This credit can have a different expiration date than the awarded Invitee Credit and the Default Free Credit.

[Offer Types Table](https://docs.google.com/spreadsheets/d/1I3Do-HMNkpUpJAsebtl1NXw6-PlkoVaFKCP9e-TTgHA/edit?zx=aqgnyh2ltfad#gid=0&range=A3:E13)

## Design

### Database

**offer  table**
```sql
    Id - bytea
    Name -  text
    Description - text  
    Credits - integer
    redeemable_cap - integer
    Num_redeemed - integer
    Created_at - timestamp
    offer_expiration - int
    award_credit_expiration - int
    Invitee_credit_expiration - int
    type - enum[FREE_TIER, REFERRAL]
    status - enum[ON_GOING, DEFAULT, EXPIRED, NO_STATUS]
    PRIMARY KEY (id)
```

**user_credit_stats table**
```sql
    User_id - bytea
    offer_id - bytea
    credits_earned - float
    Credit_type - enum[AWARD, INVITEE, NO_TYPE]
    Is_expired - bool
    Created_at - timestamp
    PRIMARY KEY (user_id)
```

**user table**
```sql
    total_Referred - int
```

### Offer Program Service

**satellite/offer/offer.go**
- Create offers interface to interact with offer table

```golang
type Offers interface {
  GetAllOffers()
  GetOfferById(offerId)
  Update(offerId)
  Delete(offerId)
  Create()
}
```

**satellite/offer/credit.go**
- Create a user_credit_stats interface to interact with the user_credit_stats  table

```golang
type user_credit_stats interface {
  GetAvailableCreditsByUserId(userId)
  GetUserCreditsByCreditType(userId, creditType)
  Update(userId, offerId, creditType, isExpired)
  Create(credit *UserCredit)
}
```

**satellite/console/userCreditStats.go**
- New service methods for retrieving user credit data from credit.go

**satellite/offer/offerweb/server.go**
- Open a new private port on the satellite for admin users to manage referral offer configuration

```golang
// NewServer creates a new instance of offerweb server
Func NewServer(logger *zap.Logger, config Config, service *offer.Service, listener net.Listener) *Server {}

// Register offerweb server onto satellite
peer.Offer.Endpoint = offerweb.NewServer(logger, config, service, listener)
```

### Referral Links

The referral link url will be a static url that contains userid as the unique identifier
Exp: `https://mars.tardigrade.io/ref/?uuid=<userid>`

### Segment.io service

**pkg/analytics/analytics.go**

- For email service, we will design several event triggers in customer.io using their event triggered campaign. We will be using analytics-go package for back-end server and analytics.js for Satellite GUI from segment.io to send our trigger event to customer.io We will create a new package in storj/pkg for analytics that will check DNT first before sending data to customer.io

```golang
client := analytics.New()
```

How to send an event from satellite to customer.io?

```golang
client.Enqueue(analytics.Track{
  UserId: "f4ca124298",
  Event:  "sign-ups",
  Properties: analytics.NewProperties().Set("referred", "true"),
})
```

How to send an event from Satellite GUI to customer.io?

```javascript
analytics.track('Signed Up', {
  referred_by: ‘brandonisawesome’
});
```

**List of trigger events**:
- Send_referral event - triggered from the Satellite GUI
- Referral_redeemed event - triggered from the consoleweb server.
Reason: activation success page is served as a static page, therefore we need to send this event from the back-end after user activated their account

How to send notification for a specific use based on their state?

By utilizing customer.io’s attribute feature, we can update user’s state through each step in our referral program and deliver appropriate content to them.

**How to update user’s state in customer.io?**

```javascript
analytics.identify("97980cfea0067", {
  name: "Peter Gibbons",
  email: "peter@initech.com",
  low_credits: true,
});
```

Then,we will be using Customer.io’s segment triggered campaign to send out notifications.

How does email get sent out when admin starts a new offer?
Due to the limitation of the customer.io API, we can’t create a new campaign through an API call. An Admin needs to go to customer.io web interface to create a new campaign manually.

### Front End

### Satellite GUI

**src/store/modules/userReferralStats.ts**
- A store for managing user’s referral stats from the backend

**src/components/referral/***.ts**
- Based on mockups, we will create components to display user’s referral stats

**src/components/account/AccountArea.vue**
- Add display for user’s referral link and copy to clipboard button using vue-clipboard2

**src/plugins/analytics**
- Create a customized plugin for analytics.js and check user’s tracking preference setting

### Admin GUI

we will be using go template for the UI
**/web/admin/offer/home.html**

## Rationale

As if the current user_credit_stats table design, we will have a new entry each time when a user earns credits.
**Disadvantage**:

- The table will grow very quickly as the user base grows. The reason why we designed this way is due to the dynamic nature of the expiration date of credits. Each credit will be expired at a different time based on the duration we set for a particular offer and the date the credit is awarded to a user.
- Each time when we want to retrieve a user's available credits, we will need to access two tables, offer and user_credit_stats, to update the is_expired value for all available credits to make sure it's up-to-date.
- Foreign key relationship between user_credits_stats table, offer table, and user table

## Open issues (if applicable)

1. Is there a better way to design the tables so that we don't have to have the foreign key relationship for user_credit_stats table?
2. We will have other marketing programs, for example, open source partner program. Should we create a top level system for all the marketing related services since we will probably be using the same private port and admin GUI for the marketing team to manage configurations for all the programs?

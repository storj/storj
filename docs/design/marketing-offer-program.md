# Marketing Offer Program Design Document

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
  - Twitter Share

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

[Offer Types Table](https://docs.google.com/spreadsheets/d/1I3Do-HMNkpUpJAsebtl1NXw6-PlkoVaFKCP9e-TTgHA/edit?usp=sharing)

## Design

### Database

**offer  table**
```sql
    id - int
    name -  text
    description - text  
    award_credits_in_cents - integer
    invitee_credits_in_cents - integer
    redeemable_cap - integer
    num_redeemed - integer
    created_at - timestamp
    expires_at - timestamp
    award_credit_duration_days - int
    invitee_credit_duration_days - int
    // ACTIVE=1, DEFAULT=2, DONE=0
    status - int
    // FreeCredit=0, Referral=1
    type - int
    PRIMARY KEY (id)
```

**user_credit table**
```sql
    id - int
    user_id - bytea
    offer_id - int
    credits_earned_in_cents - int
    credits_used_in_cents - int
    // AWARD=1, INVITEE=2, NO_TYPE=0
    credit_type - int
    expires_at - timestamp
    created_at - timestamp
    referred_by - bytea (nullable)
    FOREIGN KEY (offer_id)
    FOREIGN KEY (referred_by)
    FOREIGN KEY (user_id)
    PRIMARY KEY (id)
```

**user table**
```sql
    total_Referred - int
```

### Marketing Service

**satellite/marketing/service.go**
```golang
func (m *marketing) GetCurrentOffer(ctx context.Context, offerType OfferType) (*Offer, error) {
  offer, err := m.db.Marketing().Offers().GetOfferByType(ctx, offerType)
  if err != nil {
    return nil, Error.Wrap(err)
  }

  return offer, nil
}

func (m *marketing) StopOffer(ctx context.Context, offerId Offer.ID) error {
  o := UpdateOffer{
    Status: Done,
    ExpiresAt: time.Now(),
  }
  err := m.db.Marketing().Offers().UpdateOfferByID(ctx, offerId, o)
  if err != nil {
    return Error.Wrap(err)
  }

  return nil
}

func (m *marketing) Create(ctx context.Context, offer Offer) error {
  if offer.Status == Default {
    offer.ExpiresAt = time.Now().AddDate(100, 0, 0)
  }

  err := m.db.Marketing().Offers().CreateOffer(ctx, offer)
  if err != nil {
    return Error.Wrap(err)
  }

  return nil
}

func (m *marketing) ListAllOffers(ctx context.Context) ([]Offers, error)
```

**satellite/marketing/offers.go**
```golang
type Offers interface {
  ListAllOffers(ctx context.Context) ([]Offer, error)
  GetCurrentOffer(ctx context.Context, offerId Offer.ID) (Offer, error)
  Create(ctx context.Context, offer *Offer)
}

type OfferStatus int
const (
  Done OfferStatus = 0
  Active OfferStatus = 1
  Default OfferStatus = 2
)

type OfferType int
const (
    FreeCredit = 0
    Referral = 1
)

type UpdateOffer struct {
  NumRedeemed int 
  Status OfferStatus
  ExpiresAt time.time
}
```

**satellite/satellitedb/offers.go**
```golang
func (o *offersDB) Create(ctx context.Context, offer *marketing.Offer) error {
  tx, err := o.db.Open(ctx)
  if err != nil {
    return marketing.OfferError.Wrap(err)
  }

  _, err := tx.Get_Offer_By_Status(ctx, dbx.Offer_Status(offer.Status))
  if err == sql.ErrNoRows {
    _, err := o.db.Create_Offer(ctx, offerDbx)
    if err != nil {
      return Error.Wrap(errs.Combine(err, tx.Rollback()))
    }

    return nil
  }

  if err != nil {
    return Error.Wrap(errs.Combine(err, tx.Rollback()))
  }

  updateOffer := marketing.UpdateOffer{
    Status: Done,
    ExpiresAt: time.Now(),
  }
  _, err := tx.Update_Offer_By_Status(ctx, dbx.Offer_Status(offer.Status), updateOffer)
  if err != nil {
    return Error.Wrap(err)
  }

  _, err := o.db.Create_Offer(ctx, offerDbx)
  if err != nil {
    return Error.Wrap(errs.Combine(err, tx.Rollback()))
  }

  return nil
}
```

**satellite/console/credit.go**
- Create a user_credit interface to interact with the user_credit table
- Credits will be stored in cents as its unit.

```golang
type Credit interface {
  AvailableCredits(ctx context.Context, userId uuid.UUID) (int, error)
  ListByCreditType(ctx context.Context, userId uuid.UUID, creditType Credit.Type) ([]Credit, error)
  Update(ctx context.Context, credit *Credit) (*Credit, error)
  Create(ctx context.Context, credit *Credit) (*Credit, error)
}
```

**satellite/console/database.go**
- New method for retrieving user credit data from user_credit table
  
```golang
type DB interface {
	// Users is a getter for Users repository
  Users() Users
  // Credits is a getter for Credits repository
  Credits() Credits
	// Projects is a getter for Projects repository
	Projects() Projects
	// ProjectMembers is a getter for ProjectMembers repository
	ProjectMembers() ProjectMembers
	// APIKeys is a getter for APIKeys repository
	APIKeys() APIKeys
	// BucketUsage is a getter for accounting.BucketUsage repository
	BucketUsage() accounting.BucketUsage
	// RegistrationTokens is a getter for RegistrationTokens repository
	RegistrationTokens() RegistrationTokens
	// UsageRollups is a getter for UsageRollups repository
	UsageRollups() UsageRollups

	// BeginTransaction is a method for opening transaction
  BeginTx(ctx context.Context) (DBTx, error)
}
```

**satellite/marketing/marketingweb/server.go**
- Open a new private port on the satellite for admin users to manage referral offer configuration and other marketing configuration for various programs on our satellites
- For right now, we will rely on our VPN to restrict access to the admin GUI. Only people who are on our VPN will have access to this page.

```golang
// NewServer creates a new instance of marketingweb server
Func NewServer(logger *zap.Logger, config Config, service *marketing.Service, listener net.Listener) *Server {}

// Register marketingweb server onto satellite
peer.Offer.Endpoint = marketingweb.NewServer(logger, config, service, listener)
```

### Referral Links

The referral link url will be a static url that contains userid as the unique identifier
Exp: `https://mars.tardigrade.io/register?uuid=<userid>`

### Segment.io service

**pkg/analytics/analytics.go**

- For email service, we will design several event triggers in customer.io using their event triggered campaign. We will be using analytics-go package for back-end server and analytics.js for Satellite GUI from segment.io to send our trigger event to customer.io We will create a new package in storj/pkg for analytics that will check DNT first before sending data to customer.io
- We will add a new configuration for storing segment tracking Id specifically for tardigrade branded satellite

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
  email: "peter@mail.test",
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

**github.com/storj/tardigrade-satellite-theme**
- Add `.env` file that stores segment tracking id for each satellite 

### Admin GUI

we will be using go template for the UI
**/web/admin/offer/home.html**

## Rationale

As if the current user_credit table design, we will have a new entry each time when a user earns a credit. The reason why we designed this way is due to the starting date for the expiration date of credits. Each credit will be expired at a different time based on the duration we set for a particular offer and the date the credit is awarded to a user.

We will check against the credit duration interval when inserting a new row into the user_credit table and update the expires_at for each entry accordingly.
## Open issues (if applicable)

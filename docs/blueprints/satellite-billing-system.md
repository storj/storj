# Satellite billing system
Satellite billing system combines stripe and coinpayments API for credit card and cryptocurrency processing. It uses `satellite/accounting` pkg for project accounting. Billing is set on account but that is the subject for future changes as we want billing to be on a project level. That requires decoupling stripe dependency to the level where we utilize only credit card processing and maintain all other stuff such as customer balances and invoicing internally. Every satellite should have separate stripe and coinpayments account to prevent collision of customer related data such as uuid and email.

# Stripe customer
Stripe operates on a basis of customers. Where customer is, from stripe doc: Customer objects allow you to perform recurring charges, and to track multiple charges, that are associated with the same customer. The API allows you to create, delete, and update your customers. You can retrieve individual customers as well as a list of all your customers. Satellite billing doesn't uses `customer` concern with public API, so it is treated as implementation detail. Stripe customer balance is automatically applied to invoice total before charging a credit card. 

Stripe billing system implementation stores a customer reference for every user:
```
model stripe_customer (
    key user_id
    unique customer_id

    field user_id     blob
    field customer_id text
    field created_at  timestamp ( autoinsert )
)
```

# Public interface
Satellite payments exposes public API for console to use. It includes account related behavior that customers can use to get his/her billing information and to interact with billing system, adding credit cards and making token deposits. The top level interface `payments.Accounts` exposes account level interaction with the system. Having an interface allows to disable billing by using `satellite/payments/mockpayments` implementation which basically is just a stub that does nothing. All methods requires satellite user id, then service maps it to related stripe customer id.
```go
// Accounts exposes all needed functionality to manage payment accounts.
//
// architecture: Service
type Accounts interface {
	// Setup creates a payment account for the user.
	// If account is already set up it will return nil.
	Setup(ctx context.Context, userID uuid.UUID, email string) error

	// Balance returns an integer amount in cents that represents the current balance of payment account.
	Balance(ctx context.Context, userID uuid.UUID) (int64, error)

	// ProjectCharges returns how much money current user will be charged for each project.
	ProjectCharges(ctx context.Context, userID uuid.UUID) ([]ProjectCharge, error)

	// Charges returns list of all credit card charges related to account.
	Charges(ctx context.Context, userID uuid.UUID) ([]Charge, error)

	// CreditCards exposes all needed functionality to manage account credit cards.
	CreditCards() CreditCards

	// StorjTokens exposes all storj token related functionality.
	StorjTokens() StorjTokens

	// Invoices exposes all needed functionality to manage account invoices.
	Invoices() Invoices

	// Coupons exposes all needed functionality to manage coupons.
	Coupons() Coupons
}
```

# Customer setup
Every satellite user has a corresponding customer entity on stripe which holds credit cards, balance which reflects the ammount of STORJ tokens, and is used for invoicing. Every time a user visits billing page on the satellite UI we try to create a customer for him if one doesn't exists.
```go
// Setup creates a payment account for the user.
// If account is already set up it will return nil.
func (accounts *accounts) Setup(ctx context.Context, userID uuid.UUID, email string) (err error) {
	defer mon.Task()(&ctx, userID, email)(&err)

	_, err = accounts.service.db.Customers().GetCustomerID(ctx, userID)
	if err == nil {
		return nil
	}

	params := &stripe.CustomerParams{
		Email: stripe.String(email),
	}

	customer, err := accounts.service.stripeClient.Customers.New(params)
	if err != nil {
		return Error.Wrap(err)
	}

	// TODO: delete customer from stripe, if db insertion fails
	return Error.Wrap(accounts.service.db.Customers().Insert(ctx, userID, customer.ID))
}
```

# Balance
Account balance for a satellite user consist of stripe customer balance and active coupons and credits.
```go
// Balance returns an integer amount in cents that represents the current balance of payment account.
func (accounts *accounts) Balance(ctx context.Context, userID uuid.UUID) (_ int64, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	customerID, err := accounts.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return 0, Error.Wrap(err)
	}

	c, err := accounts.service.stripeClient.Customers.Get(customerID, nil)
	if err != nil {
		return 0, Error.Wrap(err)
	}

	// add all active coupons amount to balance.
	coupons, err := accounts.service.db.Coupons().ListByUserIDAndStatus(ctx, userID, payments.CouponActive)
	if err != nil {
		return 0, Error.Wrap(err)
	}

	var couponsAmount int64 = 0
	for _, coupon := range coupons {
		alreadyUsed, err := accounts.service.db.Coupons().TotalUsage(ctx, coupon.ID)
		if err != nil {
			return 0, Error.Wrap(err)
		}

		couponsAmount += coupon.Amount - alreadyUsed
	}

	return -c.Balance + couponsAmount, nil
}
```

# Project Charges
Project charges is basically just current bandwidth and storage usage fetched from `satellite/accounting` multiplied by price set in config.
```go
// ProjectCharges returns how much money current user will be charged for each project.
func (accounts *accounts) ProjectCharges(ctx context.Context, userID uuid.UUID) (charges []payments.ProjectCharge, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	// to return empty slice instead of nil if there are no projects
	charges = make([]payments.ProjectCharge, 0)

	projects, err := accounts.service.projectsDB.GetOwn(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	start, end := date.MonthBoundary(time.Now().UTC())

	// TODO: we should improve performance of this block of code. It takes ~4-5 sec to get project charges.
	for _, project := range projects {
		usage, err := accounts.service.usageDB.GetProjectTotal(ctx, project.ID, start, end)
		if err != nil {
			return charges, Error.Wrap(err)
		}

		projectPrice := accounts.service.calculateProjectUsagePrice(usage.Egress, usage.Storage, usage.ObjectCount)

		charges = append(charges, payments.ProjectCharge{
			ProjectID:    project.ID,
			Egress:       projectPrice.Egress.IntPart(),
			ObjectCount:  projectPrice.Objects.IntPart(),
			StorageGbHrs: projectPrice.Storage.IntPart(),
		})
	}

	return charges, nil
}
```

# Charges
Charges fetches all credit card charges related to a particular customer from stripe.  
```go
// Charges returns list of all credit card charges related to account.
func (accounts *accounts) Charges(ctx context.Context, userID uuid.UUID) (_ []payments.Charge, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	customerID, err := accounts.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	params := &stripe.ChargeListParams{
		Customer: stripe.String(customerID),
	}
	params.Filters.AddFilter("limit", "", "100")

	iter := accounts.service.stripeClient.Charges.List(params)

	var charges []payments.Charge
	for iter.Next() {
		charge := iter.Charge()

		// ignore all non credit card charges
		if charge.PaymentMethodDetails.Type != stripe.ChargePaymentMethodDetailsTypeCard {
			continue
		}
		if charge.PaymentMethodDetails.Card == nil {
			continue
		}

		charges = append(charges, payments.Charge{
			ID:     charge.ID,
			Amount: charge.Amount,
			CardInfo: payments.CardInfo{
				ID:       charge.PaymentMethod,
				Brand:    string(charge.PaymentMethodDetails.Card.Brand),
				LastFour: charge.PaymentMethodDetails.Card.Last4,
			},
			CreatedAt: time.Unix(charge.Created, 0).UTC(),
		})
	}

	if err = iter.Err(); err != nil {
		return nil, Error.Wrap(err)
	}

	return charges, nil
}
```

# Credit Cards
Credit cards processing is done via stripe. Every stripe customer can be attached a credit card, which can be used to pay for the customer invoice. Upon adding first card is automatically marked as default. If a customer has more than one card, any of the cards can be made default. Default credit card is automatically applied to new invoice as default payment method if corresponding setting is not set explicitly during invoice creation. No data is stored for the credit cards in satellite db. All methods are just wrappers around stripe API.
```go
// CreditCards exposes all needed functionality to manage account credit cards.
//
// architecture: Service
type CreditCards interface {
	// List returns a list of credit cards for a given payment account.
	List(ctx context.Context, userID uuid.UUID) ([]CreditCard, error)

	// Add is used to save new credit card and attach it to payment account.
	Add(ctx context.Context, userID uuid.UUID, cardToken string) error

	// Remove is used to detach a credit card from payment account.
	Remove(ctx context.Context, userID uuid.UUID, cardID string) error

	// MakeDefault makes a credit card default payment method.
	// this credit card should be attached to account before make it default.
	MakeDefault(ctx context.Context, userID uuid.UUID, cardID string) error
}
```

# Clearing chore
Clearing chore runs cycles which reconcile transfers and balance. It consists of transaction update cycle that updates pending transactions states and account update cycle that updates stripe customer account balance, applying successfully completed transactions.
```go
// Chore runs clearing process of reconciling transactions deposits,
// customer balance, invoices and usages.
//
// architecture: Chore
type Chore struct {
	log                 *zap.Logger
	service             *Service
	TransactionCycle    *sync2.Cycle
	AccountBalanceCycle *sync2.Cycle
}
```

# STORJ tokens processsing
Unlike with credit cards billing system uses deposit model for STORJ tokens, user has to deposit some amount prior using satellite services. 

Public API of token related billing:
```go
// StorjTokens defines all payments STORJ token related functionality.
//
// architecture: Service
type StorjTokens interface {
	// Deposit creates deposit transaction for specified amount in cents.
	Deposit(ctx context.Context, userID uuid.UUID, amount int64) (*Transaction, error)
	// ListTransactionInfos returns all transaction associated with user.
	ListTransactionInfos(ctx context.Context, userID uuid.UUID) ([]TransactionInfo, error)
}
```

# Making a deposit
STORJ cryptocurrency processing is done via coinpayments API. Every time a user wants to deposit some amount of STORJ token to his account balacne, new coinpayments transaction is created. Transaction amount is set in USD, and conversion rates is beeing locked(saved) after transaction is created and stored in the db.
```go
// Deposit creates new deposit transaction with the given amount returning
// ETH wallet address where funds should be sent. There is one
// hour limit to complete the transaction. Transaction is saved to DB with
// reference to the user who made the deposit.
func (tokens *storjTokens) Deposit(ctx context.Context, userID uuid.UUID, amount int64) (_ *payments.Transaction, err error) {
	defer mon.Task()(&ctx, userID, amount)(&err)

	customerID, err := tokens.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	c, err := tokens.service.stripeClient.Customers.Get(customerID, nil)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	rate, err := tokens.service.GetRate(ctx, coinpayments.CurrencySTORJ, coinpayments.CurrencyUSD)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	tokenAmount := convertFromCents(rate, amount).SetPrec(payments.STORJTokenPrecision)

	tx, err := tokens.service.coinPayments.Transactions().Create(ctx,
		&coinpayments.CreateTX{
			Amount:      *tokenAmount,
			CurrencyIn:  coinpayments.CurrencySTORJ,
			CurrencyOut: coinpayments.CurrencySTORJ,
			BuyerEmail:  c.Email,
		},
	)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	key, err := coinpayments.GetTransacationKeyFromURL(tx.CheckoutURL)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if err = tokens.service.db.Transactions().LockRate(ctx, tx.ID, rate); err != nil {
		return nil, Error.Wrap(err)
	}

	cpTX, err := tokens.service.db.Transactions().Insert(ctx,
		Transaction{
			ID:        tx.ID,
			AccountID: userID,
			Address:   tx.Address,
			Amount:    tx.Amount,
			Status:    coinpayments.StatusPending,
			Key:       key,
			Timeout:   tx.Timeout,
		},
	)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &payments.Transaction{
		ID:        payments.TransactionID(tx.ID),
		Amount:    *payments.TokenAmountFromBigFloat(&tx.Amount),
		Rate:      *rate,
		Address:   tx.Address,
		Status:    payments.TransactionStatusPending,
		Timeout:   tx.Timeout,
		Link:      tx.CheckoutURL,
		CreatedAt: cpTX.CreatedAt,
	}, nil
}
```

# List information about account transactions
List infos of accounts token transactions including locked rates.
```go
// TransactionInfo holds transaction data with additional information
// such as links and expiration time.
type TransactionInfo struct {
	ID            TransactionID
	Amount        TokenAmount
	Received      TokenAmount
	AmountCents   int64
	ReceivedCents int64
	Address       string
	Status        TransactionStatus
	Link          string
	ExpiresAt     time.Time
	CreatedAt     time.Time
}
```
```go
// ListTransactionInfos fetches all transactions from the database for specified user, reconstructing checkout link.
func (tokens *storjTokens) ListTransactionInfos(ctx context.Context, userID uuid.UUID) (_ []payments.TransactionInfo, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	txs, err := tokens.service.db.Transactions().ListAccount(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	var infos []payments.TransactionInfo
	for _, tx := range txs {
		link := coinpayments.GetCheckoutURL(tx.Key, tx.ID)

		var status payments.TransactionStatus
		switch tx.Status {
		case coinpayments.StatusPending:
			status = payments.TransactionStatusPending
		case coinpayments.StatusReceived:
			status = payments.TransactionStatusPaid
		case coinpayments.StatusCancelled:
			status = payments.TransactionStatusCancelled
		default:
			// unknown
			status = payments.TransactionStatus(tx.Status.String())
		}

		rate, err := tokens.service.db.Transactions().GetLockedRate(ctx, tx.ID)
		if err != nil {
			return nil, err
		}

		infos = append(infos,
			payments.TransactionInfo{
				ID:            []byte(tx.ID),
				Amount:        *payments.TokenAmountFromBigFloat(&tx.Amount),
				Received:      *payments.TokenAmountFromBigFloat(&tx.Received),
				AmountCents:   convertToCents(rate, &tx.Amount),
				ReceivedCents: convertToCents(rate, &tx.Received),
				Address:       tx.Address,
				Status:        status,
				Link:          link,
				ExpiresAt:     tx.CreatedAt.Add(tx.Timeout),
				CreatedAt:     tx.CreatedAt,
			},
		)
	}

	return infos, nil
}
```

# Transaction update cycle
There is a cycle that iterates over all `pending`(`pending` and `paid` statuses of coinpayments transaction respectively) transactions, list it's infos and updates tx status and received amount. If updated is status is set to `cancelled` or `completed`, that transactions won't take part in the next update cycle. When there is a status transation to `completed` along with the update `apply_balance_transaction_intent` is created. Transaction with status `completed` and present `apply_balance_transaction_intent` with state `unapplied` defines as `UnappliedTransaction` which is later processed in update balance cycle. If the received amount is greater that 50$ a promotional coupon for 55$ is created.
```go
// updateTransactions updates statuses and received amount for given transactions.
func (service *Service) updateTransactions(ctx context.Context, ids TransactionAndUserList) (err error) {
	defer mon.Task()(&ctx, ids)(&err)

	if len(ids) == 0 {
		service.log.Debug("no transactions found, skipping update")
		return nil
	}

	infos, err := service.coinPayments.Transactions().ListInfos(ctx, ids.IDList())
	if err != nil {
		return err
	}

	var updates []TransactionUpdate
	var applies coinpayments.TransactionIDList

	for id, info := range infos {
		updates = append(updates,
			TransactionUpdate{
				TransactionID: id,
				Status:        info.Status,
				Received:      info.Received,
			},
		)

		// moment of transition to completed state, which indicates
		// that customer funds were accepted and transferred to our
		// account, so we can apply this amount to customer balance.
		// Therefore, create intent to update customer balance in the future.
		if info.Status == coinpayments.StatusCompleted {
			applies = append(applies, id)
		}

		userID := ids[id]

		rate, err := service.db.Transactions().GetLockedRate(ctx, id)
		if err != nil {
			service.log.Error(fmt.Sprintf("could not add promotional coupon for user %s", userID.String()), zap.Error(err))
			continue
		}

		cents := convertToCents(rate, &info.Received)

		if cents >= 5000 {
			err = service.Accounts().Coupons().AddPromotionalCoupon(ctx, userID, 2, 5500, memory.TB)
			if err != nil {
				service.log.Error(fmt.Sprintf("could not add promotional coupon for user %s", userID.String()), zap.Error(err))
				continue
			}
		}
	}

	return service.db.Transactions().Update(ctx, updates, applies)
}
```

# Balance update cycle
Cycle that iterates over all `unapplied` transactions, adjusting stripe customer balance for transaction received amount. Transaction is consumed prior updating customer balance on stripe to prevent double accounting. The idea is that we rather not credit customer than credit twice. Created customer balance transaction holds id of applied transaction in it's meta, therefore there is room for verification loop which iterates over customer balance transactions, reseting state for transaction which id hasn't been found.
```go
// applyTransactionBalance applies transaction received amount to stripe customer balance.
func (service *Service) applyTransactionBalance(ctx context.Context, tx Transaction) (err error) {
	defer mon.Task()(&ctx)(&err)

	cusID, err := service.db.Customers().GetCustomerID(ctx, tx.AccountID)
	if err != nil {
		return err
	}

	rate, err := service.db.Transactions().GetLockedRate(ctx, tx.ID)
	if err != nil {
		return err
	}

	if err = service.db.Transactions().Consume(ctx, tx.ID); err != nil {
		return err
	}

	cents := convertToCents(rate, &tx.Received)

	params := &stripe.CustomerBalanceTransactionParams{
		Amount:      stripe.Int64(-cents),
		Customer:    stripe.String(cusID),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Description: stripe.String("storj token deposit"),
	}

	params.AddMetadata("txID", tx.ID.String())

	// TODO: 0 amount will return an error, how to handle that?
	_, err = service.stripeClient.CustomerBalanceTransactions.New(params)
	return err
}
```

# Invoices
Invoices are statements of amounts owed by a customer, and are generated one-off. 
```go
// Invoice holds all public information about invoice.
type Invoice struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Amount      int64     `json:"amount"`
	Status      string    `json:"status"`
	Link        string    `json:"link"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
}
```
Satellite users can retrieve all their invoices via public `payments.Invoices` interface.
```go
// Invoices exposes all needed functionality to manage account invoices.
//
// architecture: Service
type Invoices interface {
	// List returns a list of invoices for a given payment account.
	List(ctx context.Context, userID uuid.UUID) ([]Invoice, error)
}
```

# Invoice creation
Invoice include project usage cost as well as any discounts applied. Coupons and credits applied as separate invoice line items, therefore it reduce total due amount. Next applied STORJ token amount which is repesented as credits on custmer balance if any. If invoice total amount is greater than zero after bonuses and STORJ tokens, default credit card at the moment of invoice creation will be charged. If total amount is less than 1$, then stripe won't try to charge credit card but increase debt on customer balance. 

Invoice creation consist of few steps. First invoice project records have to be created. Each record consist of project id, usage and timestamps of the start and end of billing period. This way we ensure that usage is the same during all invoice creation steps and there won't be two or more invoices created for the same period(actually only invoice line items for certain billing period and project are ensured not to be created more than once). Coupon usages are also created during this step, which are later used to create coupon invoice line items.

Invoice are created using cmd `inspector` tool.
```bash
inspector --identity-path "/Users/user/Library/Application Support/Storj/Identity/inspector" payments [command]
```
Available commands:
```text
create-invoice-coupons  Creates stripe invoice line items for not consumed coupons

create-invoice-items  Creates stripe invoice line items for not consumed project records

create-invoices Creates stripe invoices for all stripe customers known to satellite

prepare-invoice-records Prepares invoice project records that will be used during invoice line items creation
```
## Prepare invoice project records
```bash
inspector payments prepare-invoice-records [mm/yyyy]
```
Create project records for all projects for specified billing period. Billing period defined as `[0th nanosecond of the first day of the month; 0th nanosecond of the first day of the following month)`. 
Project record contains project usage for some billing period. Therefore, it is impossible to create project record for the same project and billing period.
```go
// ProjectRecord holds project usage particular for billing period.
type ProjectRecord struct {
	ID          uuid.UUID
	ProjectID   uuid.UUID
	Storage     float64
	Egress      int64
	Objects     float64
	PeriodStart time.Time
	PeriodEnd   time.Time
}
```
This command sets billing period and iterates over all projects on the satellite creating invoice project records.
```go
// PrepareInvoiceProjectRecords iterates through all projects and creates invoice records if
// none exists.
func (service *Service) PrepareInvoiceProjectRecords(ctx context.Context, period time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	const limit = 25

	now := time.Now().UTC()
	utc := period.UTC()

	start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(utc.Year(), utc.Month()+1, 1, 0, 0, 0, 0, time.UTC)

	if end.After(now) {
		return Error.New("prepare is for past periods only")
	}

	projsPage, err := service.projectsDB.List(ctx, 0, limit, end)
	if err != nil {
		return Error.Wrap(err)
	}

	if err = service.createProjectRecords(ctx, projsPage.Projects, start, end); err != nil {
		return Error.Wrap(err)
	}

	for projsPage.Next {
		if err = ctx.Err(); err != nil {
			return Error.Wrap(err)
		}

		projsPage, err = service.projectsDB.List(ctx, projsPage.NextOffset, limit, end)
		if err != nil {
			return Error.Wrap(err)
		}

		if err = service.createProjectRecords(ctx, projsPage.Projects, start, end); err != nil {
			return Error.Wrap(err)
		}
	}

	return nil
}
```
If a project record already exists, project is skipped. 
```go
// createProjectRecords creates invoice project record if none exists.
func (service *Service) createProjectRecords(ctx context.Context, projects []console.Project, start, end time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	var records []CreateProjectRecord
	var usages []CouponUsage
	for _, project := range projects {
		if err = ctx.Err(); err != nil {
			return err
		}

		if err = service.db.ProjectRecords().Check(ctx, project.ID, start, end); err != nil {
			if err == ErrProjectRecordExists {
				continue
			}

			return err
		}

		usage, err := service.usageDB.GetProjectTotal(ctx, project.ID, start, end)
		if err != nil {
			return err
		}

		// TODO: account for usage data.
		records = append(records,
			CreateProjectRecord{
				ProjectID: project.ID,
				Storage:   usage.Storage,
				Egress:    usage.Egress,
				Objects:   usage.ObjectCount,
			},
		)

		coupons, err := service.db.Coupons().ListByProjectID(ctx, project.ID)
		if err != nil {
			return err
		}

		currentUsagePrice := service.calculateProjectUsagePrice(usage.Egress, usage.Storage, usage.ObjectCount).TotalInt64()

		// TODO: only for 1 coupon per project
		for _, coupon := range coupons {
			if coupon.IsExpired() {
				if err = service.db.Coupons().Update(ctx, coupon.ID, payments.CouponExpired); err != nil {
					return err
				}

				continue
			}

			alreadyChargedAmount, err := service.db.Coupons().TotalUsage(ctx, coupon.ID)
			if err != nil {
				return err
			}
			remaining := coupon.Amount - alreadyChargedAmount

			if currentUsagePrice >= remaining {
				currentUsagePrice = remaining
			}

			usages = append(usages, CouponUsage{
				Period:   start,
				Amount:   currentUsagePrice,
				Status:   CouponUsageStatusUnapplied,
				CouponID: coupon.ID,
			})
		}
	}

	return service.db.ProjectRecords().Create(ctx, records, usages, start, end)
}
```

## Create invoice line items
Next step is to create invoice line items for project usage from invoice project records. We iterate through all unapplied invoice project records creating invoice line item for each of them. Stripe invoice line items are connected to stripe customer, each project line item that belongs to the same customer will be gathered under one invoice and billed at once. Before creating line item, project record is consumed so it won't participate in the next loop. If any error during creation of the invoice line item on stripe, invoice project record state can be reset manually to create line item for the project in the next loop. It also possible to include this item in the invoice for next billing period.

To create invoice project line items:
```bash
inspector payments create-invoice-items
```
Iterate over all project records, calculating price and creating invoice line item for each. Project record that has project owner without corresponding stripe customer is skipped, therefore, this record will participate in the next loop, until the record is consumed or deleted.
```go
// applyProjectRecords applies invoice intents as invoice line items to stripe customer.  
func (service *Service) applyProjectRecords(ctx context.Context, records []ProjectRecord) (err error) {  
   defer mon.Task()(&ctx)(&err)  
  
   for _, record := range records {  
      if err = ctx.Err(); err != nil {  
         return err  
  }  
  
      proj, err := service.projectsDB.Get(ctx, record.ProjectID)  
      if err != nil {  
         return err  
  }  
  
      cusID, err := service.db.Customers().GetCustomerID(ctx, proj.OwnerID)  
      if err != nil {  
         if err == ErrNoCustomer {  
            continue  
  }  
  
         return err  
  }  
  
      if err = service.createInvoiceItems(ctx, cusID, proj.Name, record); err != nil {  
         return err  
  }  
   }  
  
   return nil  
}
```
Project record is consumed prior calling stripe, so in case of failure it won't appear in next loop and has to be reset.
```go
// createInvoiceItems consumes invoice project record and creates invoice line items for stripe customer.
func (service *Service) createInvoiceItems(ctx context.Context, cusID, projName string, record ProjectRecord) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err = service.db.ProjectRecords().Consume(ctx, record.ID); err != nil {
		return err
	}

	projectPrice := service.calculateProjectUsagePrice(record.Egress, record.Storage, record.Objects)

	projectItem := &stripe.InvoiceItemParams{
		Amount:      stripe.Int64(projectPrice.TotalInt64()),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Customer:    stripe.String(cusID),
		Description: stripe.String(fmt.Sprintf("project %s", projName)),
		Period: &stripe.InvoiceItemPeriodParams{
			End:   stripe.Int64(record.PeriodEnd.Unix()),
			Start: stripe.Int64(record.PeriodStart.Unix()),
		},
	}

	projectItem.AddMetadata("projectID", record.ProjectID.String())

	_, err = service.stripeClient.InvoiceItems.New(projectItem)
	return err
}
```
## Create invoice on stripe
Stripe invoices are created via command:
```bash
inspector payments create-invoices
```
Iterate over all customers and create invoice for each.
```go
// CreateInvoices lists through all customers and creates invoices.  
func (service *Service) CreateInvoices(ctx context.Context) (err error) {  
   defer mon.Task()(&ctx)(&err)  
  
   const limit = 25  
  before := time.Now()  
  
   cusPage, err := service.db.Customers().List(ctx, 0, limit, before)  
   if err != nil {  
      return Error.Wrap(err)  
   }  
  
   for _, cus := range cusPage.Customers {  
      if err = ctx.Err(); err != nil {  
         return Error.Wrap(err)  
      }  
  
      if err = service.createInvoice(ctx, cus.ID); err != nil {  
         return Error.Wrap(err)  
      }  
   }  
  
   for cusPage.Next {  
      if err = ctx.Err(); err != nil {  
         return Error.Wrap(err)  
      }  
  
      cusPage, err = service.db.Customers().List(ctx, cusPage.NextOffset, limit, before)  
      if err != nil {  
         return Error.Wrap(err)  
      }  
  
      for _, cus := range cusPage.Customers {  
         if err = ctx.Err(); err != nil {  
            return Error.Wrap(err)  
         }  
  
         if err = service.createInvoice(ctx, cus.ID); err != nil {  
            return Error.Wrap(err)  
         }  
      }  
   }  
  
   return nil  
}
```
If stripe customer has no line items invoice creation will fail. This error is checked and skipped for loop not to break for customers with no projects or in case of retrial skipped customers with successfully created invoices when previous loop failed.
```go
// createInvoice creates invoice for stripe customer. Returns nil error if there are no
// pending invoice line items for customer.
func (service *Service) createInvoice(ctx context.Context, cusID string) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = service.stripeClient.Invoices.New(
		&stripe.InvoiceParams{
			Customer:    stripe.String(cusID),
			AutoAdvance: stripe.Bool(true),
		},
	)

	if err != nil {
		if stripeErr, ok := err.(*stripe.Error); ok {
			switch stripeErr.Code {
			case stripe.ErrorCodeInvoiceNoCustomerLineItems:
				return nil
			default:
				return err
			}
		}
	}

	return nil
}
```
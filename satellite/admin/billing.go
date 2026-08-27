// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/currency"
	"storj.io/common/uuid"
	"storj.io/storj/private/api"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/billing"
)

// UserTokenBalance is a user's STORJ token deposit wallet and balance.
type UserTokenBalance struct {
	// Wallet is the hex deposit wallet address. It's empty when the user has never claimed one.
	Wallet string `json:"wallet"`
	// Balance is the STORJ token balance in US dollars, as a decimal string.
	Balance string `json:"balance"`
}

// TokenTransaction is a single STORJ token transaction of a user.
type TokenTransaction struct {
	ID string `json:"id"`
	// Type is the transaction source: "storjscan", "storjscanbonus" or "coinpayments".
	Type   string `json:"type"`
	Wallet string `json:"wallet"`
	// Amount is the transaction amount in US dollars, as a decimal string.
	Amount string `json:"amount"`
	// Received is the amount actually received in US dollars, as a decimal string. It's only
	// set for coinpayments transactions, which may be partially paid.
	Received string `json:"received"`
	Status   string `json:"status"`
	// Link is the block explorer URL of the transaction.
	Link      string    `json:"link"`
	Timestamp time.Time `json:"timestamp"`
}

// UserTokenTransactions is a user's STORJ token transaction history.
type UserTokenTransactions struct {
	Transactions []TokenTransaction `json:"transactions"`
}

// GetUserTokenBalance returns the STORJ token deposit wallet address and token balance of a user.
func (s *Service) GetUserTokenBalance(ctx context.Context, userID uuid.UUID) (_ *UserTokenBalance, _ api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	if apiErr := s.verifyBillableUser(ctx, userID); apiErr.Err != nil {
		err = apiErr.Err
		return nil, apiErr
	}

	balance, err := s.billingDB.GetBalance(ctx, userID)
	if err != nil {
		return nil, api.HTTPError{Status: http.StatusInternalServerError, Err: Error.Wrap(err)}
	}

	response := &UserTokenBalance{Balance: balance.AsDecimal().String()}

	address, err := s.depositWallets.Get(ctx, userID)
	if err != nil {
		if !errors.Is(err, billing.ErrNoWallet) {
			return nil, api.HTTPError{Status: http.StatusInternalServerError, Err: Error.Wrap(err)}
		}
		// A user without a claimed wallet is not an error, they simply have no deposit address.
		err = nil
		return response, api.HTTPError{}
	}
	response.Wallet = address.Hex()

	return response, api.HTTPError{}
}

// GetUserTokenTransactions returns the STORJ token transaction history of a user, newest first.
func (s *Service) GetUserTokenTransactions(ctx context.Context, userID uuid.UUID) (_ *UserTokenTransactions, _ api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	if apiErr := s.verifyBillableUser(ctx, userID); apiErr.Err != nil {
		err = apiErr.Err
		return nil, apiErr
	}

	apiError := func(err error) api.HTTPError {
		return api.HTTPError{Status: http.StatusInternalServerError, Err: Error.Wrap(err)}
	}

	// Only the on-chain payments are keyed on the deposit wallet. Coinpayments deposits and
	// bonuses are keyed on the user, so a user who never claimed a wallet may still have them.
	var (
		walletHex      string
		walletPayments []payments.WalletPayment
	)
	address, err := s.depositWallets.Get(ctx, userID)
	switch {
	case err == nil:
		walletHex = address.Hex()

		// The limit matches the one the console uses, see console.Payments.WalletPayments.
		walletPayments, err = s.depositWallets.Payments(ctx, address, 3000, 0)
		if err != nil {
			return nil, apiError(err)
		}
	case errors.Is(err, billing.ErrNoWallet):
		err = nil
	default:
		return nil, apiError(err)
	}

	txInfos, err := s.payments.StorjTokens().ListTransactionInfos(ctx, userID)
	if err != nil {
		return nil, apiError(err)
	}
	bonuses, err := s.billingDB.ListSource(ctx, userID, billing.StorjScanBonusSource)
	if err != nil {
		return nil, apiError(err)
	}

	transactions := make([]TokenTransaction, 0, len(walletPayments)+len(txInfos)+len(bonuses))

	for _, payment := range walletPayments {
		source := s.paymentSourceChainIDs[payment.ChainID]
		transactions = append(transactions, TokenTransaction{
			ID:        fmt.Sprintf("%s#%d", payment.Transaction.Hex(), payment.LogIndex),
			Type:      "storjscan",
			Wallet:    payment.To.Hex(),
			Amount:    payment.USDValue.AsDecimal().String(),
			Status:    string(payment.Status),
			Link:      s.blockExplorerURL(payment.Transaction.Hex(), source),
			Timestamp: payment.Timestamp,
		})
	}

	for _, txInfo := range txInfos {
		transactions = append(transactions, TokenTransaction{
			ID:        txInfo.ID.String(),
			Type:      "coinpayments",
			Wallet:    txInfo.Address,
			Amount:    centsToDollarString(txInfo.AmountCents),
			Received:  centsToDollarString(txInfo.ReceivedCents),
			Status:    txInfo.Status.String(),
			Link:      txInfo.Link,
			Timestamp: txInfo.CreatedAt.UTC(),
		})
	}

	for _, bonus := range bonuses {
		// A bonus transaction carries a verbatim copy of the metadata of the storjscan payment
		// it was granted for, so the block explorer link is resolved from the chain ID of that
		// payment rather than from the bonus' own source.
		var meta struct {
			ReferenceID string
			ChainID     int64
		}
		if err = json.Unmarshal(bonus.Metadata, &meta); err != nil {
			return nil, apiError(err)
		}

		transactions = append(transactions, TokenTransaction{
			ID:        strconv.FormatInt(bonus.ID, 10),
			Type:      bonus.Source,
			Wallet:    walletHex,
			Amount:    bonus.Amount.AsDecimal().String(),
			Status:    string(bonus.Status),
			Link:      s.blockExplorerURL(meta.ReferenceID, s.paymentSourceChainIDs[meta.ChainID]),
			Timestamp: bonus.Timestamp,
		})
	}

	sort.SliceStable(transactions, func(i, j int) bool {
		return transactions[i].Timestamp.After(transactions[j].Timestamp)
	})

	return &UserTokenTransactions{Transactions: transactions}, api.HTTPError{}
}

// verifyBillableUser checks that the user with the given ID exists and that the caller is allowed
// to see its billing information.
func (s *Service) verifyBillableUser(ctx context.Context, userID uuid.UUID) api.HTTPError {
	if !s.consoleConfig.BillingFeaturesEnabled {
		return api.HTTPError{
			Status: http.StatusForbidden,
			Err:    Error.New("billing features are not enabled"),
		}
	}

	user, err := s.consoleDB.Users().Get(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return api.HTTPError{Status: http.StatusNotFound, Err: errs.New("user not found")}
		}
		return api.HTTPError{Status: http.StatusInternalServerError, Err: Error.Wrap(err)}
	}
	if !s.userMatchesTenant(user.TenantID) {
		return api.HTTPError{Status: http.StatusNotFound, Err: errs.New("user not found")}
	}

	return api.HTTPError{}
}

// blockExplorerURL builds the zkSync or etherscan URL of a transaction based on its billing
// payment source. It mirrors console.Payments.BlockExplorerURL.
func (s *Service) blockExplorerURL(tx string, source string) string {
	if tx == "" {
		return ""
	}

	beURL := s.consoleConfig.BlockExplorerURL
	if source == billing.StorjScanZkSyncSource {
		beURL = s.consoleConfig.ZkSyncBlockExplorerURL
	}
	if !strings.HasSuffix(beURL, "/") {
		beURL += "/"
	}

	return beURL + "tx/" + tx
}

// centsToDollarString formats an amount of cents as a decimal string of US dollars.
func centsToDollarString(cents int64) string {
	return currency.AmountFromBaseUnits(cents, currency.USDollars).AsDecimal().String()
}

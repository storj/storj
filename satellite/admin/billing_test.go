// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/currency"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/blockchain"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/admin"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/coinpayments"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/satellite/payments/storjscan/blockchaintest"
	"storj.io/storj/satellite/payments/stripe"
)

const (
	testBlockExplorerURL       = "https://etherscan.io/"
	testZkSyncBlockExplorerURL = "https://explorer.zksync.io/"
	// testBonusRate is the percentage of a deposit granted as a bonus, mirroring what the
	// billing chore would apply in production.
	testBonusRate = 10
)

// insertStorjscanPayment stores a confirmed storjscan payment for the wallet and returns it.
func insertStorjscanPayment(
	ctx *testcontext.Context, t *testing.T, db satellite.DB,
	wallet blockchain.Address, chainID int64, usdMicros int64, logIndex int, timestamp time.Time,
) storjscan.CachedPayment {
	t.Helper()

	payment := storjscan.CachedPayment{
		ChainID:     chainID,
		From:        blockchaintest.NewAddress(),
		To:          wallet,
		TokenValue:  currency.AmountFromBaseUnits(usdMicros, currency.StorjToken),
		USDValue:    currency.AmountFromBaseUnits(usdMicros, currency.USDollarsMicro),
		Status:      payments.PaymentStatusConfirmed,
		BlockHash:   blockchaintest.NewHash(),
		BlockNumber: int64(logIndex),
		Transaction: blockchaintest.NewHash(),
		LogIndex:    logIndex,
		Timestamp:   timestamp,
	}
	require.NoError(t, db.StorjscanPayments().InsertBatch(ctx, []storjscan.CachedPayment{payment}))

	return payment
}

// insertBillingDeposit records a storjscan deposit as a billing transaction along with its bonus,
// the way the billing chore does. Both credit the user's balance.
func insertBillingDeposit(
	ctx *testcontext.Context, t *testing.T, db satellite.DB,
	userID uuid.UUID, payment storjscan.CachedPayment, source string,
) {
	t.Helper()

	metadata, err := json.Marshal(struct {
		ReferenceID string
		Wallet      string
		ChainID     int64
		BlockNumber int64
		LogIndex    int
	}{
		ReferenceID: payment.Transaction.Hex(),
		Wallet:      payment.To.Hex(),
		ChainID:     payment.ChainID,
		BlockNumber: payment.BlockNumber,
		LogIndex:    payment.LogIndex,
	})
	require.NoError(t, err)

	deposit := billing.Transaction{
		UserID:      userID,
		Amount:      payment.USDValue,
		Description: "Storj token deposit",
		Source:      source,
		Status:      billing.TransactionStatusCompleted,
		Type:        billing.TransactionTypeCredit,
		Metadata:    metadata,
		Timestamp:   payment.Timestamp,
	}
	bonus := deposit
	bonus.Amount = billing.CalculateBonusAmount(payment.USDValue, testBonusRate)
	bonus.Description = fmt.Sprintf("STORJ Token Bonus (%d%%)", testBonusRate)
	bonus.Source = billing.StorjScanBonusSource

	_, err = db.Billing().Insert(ctx, deposit, bonus)
	require.NoError(t, err)
}

func TestTokens(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		UplinkCount:    1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
				config.Admin.BypassAuth = true
				config.Console.BlockExplorerURL = testBlockExplorerURL
				config.Console.ZkSyncBlockExplorerURL = testZkSyncBlockExplorerURL
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		userID := planet.Uplinks[0].Projects[0].Owner.ID

		baseURL := "http://" + sat.Admin.Admin.Listener.Addr().String()
		userURL := func(id uuid.UUID, resource string) string {
			return baseURL + "/api/v1/users/" + id.String() + "/" + resource
		}

		get := func(t *testing.T, url string, out any) int {
			t.Helper()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			if resp.StatusCode == http.StatusOK && out != nil {
				require.NoError(t, json.NewDecoder(resp.Body).Decode(out))
			}

			return resp.StatusCode
		}

		getBalance := func(t *testing.T) admin.UserTokenBalance {
			t.Helper()

			var balance admin.UserTokenBalance
			require.Equal(t, http.StatusOK, get(t, userURL(userID, "token-balance"), &balance))

			return balance
		}

		getTransactions := func(t *testing.T) []admin.TokenTransaction {
			t.Helper()

			var result admin.UserTokenTransactions
			require.Equal(t, http.StatusOK, get(t, userURL(userID, "token-transactions"), &result))

			return result.Transactions
		}

		t.Run("no wallet", func(t *testing.T) {
			balance := getBalance(t)
			require.Empty(t, balance.Wallet)
			require.Equal(t, "0", balance.Balance)

			require.Empty(t, getTransactions(t))
		})

		t.Run("coinpayments only, no wallet", func(t *testing.T) {
			legacyTxs := sat.DB.StripeCoinPayments().Transactions()
			legacyTxID := coinpayments.TransactionID("legacy-tx-1")

			_, err := legacyTxs.TestInsert(ctx, stripe.Transaction{
				ID:        legacyTxID,
				AccountID: userID,
				Address:   "0xcoinpayments",
				Amount:    currency.AmountFromBaseUnits(40*1e8, currency.StorjToken),
				Received:  currency.AmountFromBaseUnits(40*1e8, currency.StorjToken),
				Status:    coinpayments.StatusReceived,
				Key:       "key",
				Timeout:   time.Hour,
				CreatedAt: time.Now().Add(-96 * time.Hour),
			})
			require.NoError(t, err)

			// The locked rate is what converts the STORJ amount into the USD figure the API reports.
			require.NoError(t, legacyTxs.TestLockRate(ctx, legacyTxID, decimal.NewFromInt(1)))

			transactions := getTransactions(t)
			require.Len(t, transactions, 1)

			tx := transactions[0]
			require.Equal(t, "coinpayments", tx.Type)
			require.Equal(t, "40", tx.Received)
			require.NotEmpty(t, tx.Link)
			// Coinpayments rows carry the address coinpayments issued, not a storjscan wallet.
			require.Equal(t, "0xcoinpayments", tx.Wallet)

			// Coinpayments deposits are credited to the stripe balance, not the token balance.
			require.Empty(t, getBalance(t).Wallet)
		})

		wallet := blockchaintest.NewAddress()
		require.NoError(t, sat.DB.Wallets().Add(ctx, userID, wallet))

		t.Run("wallet without deposits", func(t *testing.T) {
			balance := getBalance(t)
			require.Equal(t, wallet.Hex(), balance.Wallet)
			require.Equal(t, "0", balance.Balance)
		})

		now := time.Now().Truncate(time.Second).UTC()
		ethPayment := insertStorjscanPayment(ctx, t, sat.DB, wallet, 1, 10_000_000, 0, now.Add(-2*time.Hour))
		insertBillingDeposit(ctx, t, sat.DB, userID, ethPayment, billing.StorjScanEthereumSource)

		zkPayment := insertStorjscanPayment(ctx, t, sat.DB, wallet, 324, 5_000_000, 1, now.Add(-time.Hour))
		insertBillingDeposit(ctx, t, sat.DB, userID, zkPayment, billing.StorjScanZkSyncSource)

		t.Run("wallet with deposits", func(t *testing.T) {
			balance := getBalance(t)
			require.Equal(t, wallet.Hex(), balance.Wallet)
			// $10 and $5 deposited, plus the 10% bonus inserted alongside each.
			require.Equal(t, "16.5", balance.Balance)
		})

		transactions := getTransactions(t)

		byType := map[string][]admin.TokenTransaction{}
		for _, tx := range transactions {
			byType[tx.Type] = append(byType[tx.Type], tx)
		}

		t.Run("every source is listed", func(t *testing.T) {
			// Two on-chain deposits, their two bonuses, and the coinpayments deposit above.
			require.Len(t, transactions, 5)
			require.Len(t, byType["storjscan"], 2)
			require.Len(t, byType[billing.StorjScanBonusSource], 2)
			require.Len(t, byType["coinpayments"], 1)
		})

		t.Run("sorted newest first", func(t *testing.T) {
			for i := 1; i < len(transactions); i++ {
				require.False(t, transactions[i-1].Timestamp.Before(transactions[i].Timestamp))
			}
		})

		t.Run("on-chain deposits", func(t *testing.T) {
			byID := map[string]admin.TokenTransaction{}
			for _, tx := range byType["storjscan"] {
				byID[tx.ID] = tx
			}

			eth := byID[fmt.Sprintf("%s#%d", ethPayment.Transaction.Hex(), ethPayment.LogIndex)]
			require.Equal(t, wallet.Hex(), eth.Wallet)
			require.Equal(t, "10", eth.Amount)
			require.Empty(t, eth.Received)
			require.Equal(t, string(payments.PaymentStatusConfirmed), eth.Status)
			require.Equal(t, testBlockExplorerURL+"tx/"+ethPayment.Transaction.Hex(), eth.Link)

			zk := byID[fmt.Sprintf("%s#%d", zkPayment.Transaction.Hex(), zkPayment.LogIndex)]
			require.Equal(t, "5", zk.Amount)
			require.Equal(t, testZkSyncBlockExplorerURL+"tx/"+zkPayment.Transaction.Hex(), zk.Link)
		})

		t.Run("bonuses link to the chain of their deposit", func(t *testing.T) {
			var links []string
			for _, tx := range byType[billing.StorjScanBonusSource] {
				require.Equal(t, wallet.Hex(), tx.Wallet)
				links = append(links, tx.Link)
			}
			require.Contains(t, links, testBlockExplorerURL+"tx/"+ethPayment.Transaction.Hex())
			require.Contains(t, links, testZkSyncBlockExplorerURL+"tx/"+zkPayment.Transaction.Hex())
		})

		t.Run("unknown user", func(t *testing.T) {
			unknown := testrand.UUID()
			require.Equal(t, http.StatusNotFound, get(t, userURL(unknown, "token-balance"), nil))
			require.Equal(t, http.StatusNotFound, get(t, userURL(unknown, "token-transactions"), nil))
		})

		t.Run("billing features disabled", func(t *testing.T) {
			sat.Admin.Admin.Service.TestSetBillingFeaturesEnabled(false)
			defer sat.Admin.Admin.Service.TestSetBillingFeaturesEnabled(true)

			require.Equal(t, http.StatusForbidden, get(t, userURL(userID, "token-balance"), nil))
			require.Equal(t, http.StatusForbidden, get(t, userURL(userID, "token-transactions"), nil))
		})
	})
}

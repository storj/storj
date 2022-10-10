// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package coinpayments

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeebo/errs"

	"storj.io/common/currency"
)

const (
	cmdCreateTransaction      = "create_transaction"
	cmdGetTransactionInfo     = "get_tx_info"
	cmdGetTransactionInfoList = "get_tx_info_multi"
)

// Status is a type wrapper for transaction statuses.
type Status int

const (
	// StatusCancelled defines cancelled or timeout transaction.
	StatusCancelled Status = -1
	// StatusPending defines pending transaction which is waiting for buyer funds.
	StatusPending Status = 0
	// StatusReceived defines transaction which successfully received required amount of funds.
	StatusReceived Status = 1
	// StatusCompleted defines transaction which is fully completed.
	StatusCompleted Status = 100
)

// Int returns int representation of status.
func (s Status) Int() int {
	return int(s)
}

// String returns string representation of status.
func (s Status) String() string {
	switch s {
	case StatusCancelled:
		return "cancelled/timeout"
	case StatusPending:
		return "pending"
	case StatusReceived:
		return "received"
	case StatusCompleted:
		return "completed"
	default:
		return "unknown"
	}
}

// TransactionID is type wrapper for transaction id.
type TransactionID string

// String returns string representation of transaction id.
func (id TransactionID) String() string {
	return string(id)
}

// TransactionIDList is a type wrapper for list of transactions.
type TransactionIDList []TransactionID

// Encode returns encoded string representation of transaction id list.
func (list TransactionIDList) Encode() string {
	if len(list) == 0 {
		return ""
	}
	if len(list) == 1 {
		return string(list[0])
	}

	var builder strings.Builder
	for _, id := range list[:len(list)-1] {
		builder.WriteString(string(id))
		builder.WriteString("|")
	}

	builder.WriteString(string(list[len(list)-1]))
	return builder.String()
}

// Transaction contains data returned on transaction creation.
type Transaction struct {
	ID             TransactionID
	Address        string
	Amount         currency.Amount
	DestTag        string
	ConfirmsNeeded int
	Timeout        time.Duration
	CheckoutURL    string
	StatusURL      string
	QRCodeURL      string
}

// UnmarshalJSON handles json unmarshaling for transaction.
func (tx *Transaction) UnmarshalJSON(b []byte) error {
	var txRaw struct {
		Amount         string `json:"amount"`
		Address        string `json:"address"`
		DestTag        string `json:"dest_tag"`
		TxID           string `json:"txn_id"`
		ConfirmsNeeded string `json:"confirms_needed"`
		Timeout        int    `json:"timeout"`
		CheckoutURL    string `json:"checkout_url"`
		StatusURL      string `json:"status_url"`
		QRCodeURL      string `json:"qrcode_url"`
	}

	if err := json.Unmarshal(b, &txRaw); err != nil {
		return err
	}

	amount, err := currency.AmountFromString(txRaw.Amount, currency.StorjToken)
	if err != nil {
		return err
	}

	confirms, err := strconv.ParseInt(txRaw.ConfirmsNeeded, 10, 64)
	if err != nil {
		return err
	}

	*tx = Transaction{
		ID:             TransactionID(txRaw.TxID),
		Address:        txRaw.Address,
		Amount:         amount,
		DestTag:        txRaw.DestTag,
		ConfirmsNeeded: int(confirms),
		Timeout:        time.Second * time.Duration(txRaw.Timeout),
		CheckoutURL:    txRaw.CheckoutURL,
		StatusURL:      txRaw.StatusURL,
		QRCodeURL:      txRaw.QRCodeURL,
	}

	return nil
}

// TransactionInfo holds transaction information.
type TransactionInfo struct {
	Address          string
	Coin             CurrencySymbol
	Amount           decimal.Decimal
	Received         decimal.Decimal
	ConfirmsReceived int
	Status           Status
	ExpiresAt        time.Time
	CreatedAt        time.Time
}

// UnmarshalJSON handles json unmarshaling for transaction info.
func (info *TransactionInfo) UnmarshalJSON(b []byte) error {
	var txInfoRaw struct {
		Address      string          `json:"payment_address"`
		Coin         string          `json:"coin"`
		Status       int             `json:"status"`
		Amount       decimal.Decimal `json:"amountf"`
		Received     decimal.Decimal `json:"receivedf"`
		ConfirmsRecv int             `json:"recv_confirms"`
		ExpiresAt    int64           `json:"time_expires"`
		CreatedAt    int64           `json:"time_created"`
	}

	if err := json.Unmarshal(b, &txInfoRaw); err != nil {
		return err
	}

	*info = TransactionInfo{
		Address:          txInfoRaw.Address,
		Coin:             CurrencySymbol(txInfoRaw.Coin),
		Amount:           txInfoRaw.Amount,
		Received:         txInfoRaw.Received,
		ConfirmsReceived: txInfoRaw.ConfirmsRecv,
		Status:           Status(txInfoRaw.Status),
		ExpiresAt:        time.Unix(txInfoRaw.ExpiresAt, 0),
		CreatedAt:        time.Unix(txInfoRaw.CreatedAt, 0),
	}

	return nil
}

// TransactionInfos is map of transaction infos by transaction id.
type TransactionInfos map[TransactionID]TransactionInfo

// UnmarshalJSON handles json unmarshaling for TransactionInfos.
func (infos *TransactionInfos) UnmarshalJSON(b []byte) error {
	var _infos map[TransactionID]TransactionInfo

	var errors map[TransactionID]struct {
		Error string `json:"error"`
	}

	if err := json.Unmarshal(b, &errors); err != nil {
		return err
	}

	var errg errs.Group
	for _, info := range errors {
		if info.Error != "ok" {
			errg.Add(errs.New(info.Error))
		}
	}

	if err := errg.Err(); err != nil {
		return err
	}

	if err := json.Unmarshal(b, &_infos); err != nil {
		return err
	}

	for id, info := range _infos {
		(*infos)[id] = info
	}

	return nil
}

// CreateTX defines parameters for transaction creating.
type CreateTX struct {
	Amount      decimal.Decimal
	CurrencyIn  *currency.Currency
	CurrencyOut *currency.Currency
	BuyerEmail  string
}

// Transactions defines transaction related API methods.
type Transactions struct {
	client *Client
}

// Create creates new transaction.
func (t Transactions) Create(ctx context.Context, params *CreateTX) (*Transaction, error) {
	cpSymbolIn, ok := currencySymbols[params.CurrencyIn]
	if !ok {
		return nil, Error.New("can't identify coinpayments currency symbol for %q", params.CurrencyIn.Name())
	}
	cpSymbolOut, ok := currencySymbols[params.CurrencyOut]
	if !ok {
		return nil, Error.New("can't identify coinpayments currency symbol for %q", params.CurrencyOut.Name())
	}
	values := make(url.Values)
	values.Set("amount", params.Amount.String())
	values.Set("currency1", string(cpSymbolIn))
	values.Set("currency2", string(cpSymbolOut))
	values.Set("buyer_email", params.BuyerEmail)

	tx := new(Transaction)

	res, err := t.client.do(ctx, cmdCreateTransaction, values)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if err = json.Unmarshal(res, tx); err != nil {
		return nil, Error.Wrap(err)
	}

	return tx, nil
}

// Info receives transaction info by transaction id.
func (t Transactions) Info(ctx context.Context, id TransactionID) (*TransactionInfo, error) {
	values := make(url.Values)
	values.Set("txid", id.String())

	txInfo := new(TransactionInfo)

	res, err := t.client.do(ctx, cmdGetTransactionInfo, values)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if err = json.Unmarshal(res, txInfo); err != nil {
		return nil, Error.Wrap(err)
	}

	return txInfo, nil
}

// ListInfos returns transaction infos.
func (t Transactions) ListInfos(ctx context.Context, ids TransactionIDList) (TransactionInfos, error) {
	// The service supports a max batch size of 25 items
	const batchSize = 25
	var allErrors error
	numIds := len(ids)
	txInfos := make(TransactionInfos, numIds)

	for i := 0; i < len(ids); i += batchSize {
		j := i + batchSize
		if j > numIds {
			j = numIds
		}
		batchInfos := make(TransactionInfos, j-i)
		values := make(url.Values, j-i)
		values.Set("txid", ids[i:j].Encode())
		res, err := t.client.do(ctx, cmdGetTransactionInfoList, values)
		if err != nil {
			allErrors = errs.Combine(allErrors, err)
		}
		if err = json.Unmarshal(res, &batchInfos); err != nil {
			allErrors = errs.Combine(allErrors, err)
		}
		for k, v := range batchInfos {
			txInfos[k] = v
		}
	}
	return txInfos, allErrors
}

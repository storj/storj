// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package coinpayments

import "strings"

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

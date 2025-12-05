// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package usedserials

import (
	"context"
	"encoding/binary"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/common/storj"
)

var (
	// ErrSerials defines the usedserials store error class.
	ErrSerials = errs.Class("usedserials")
	// ErrSerialAlreadyExists defines an error class for duplicate usedserials.
	ErrSerialAlreadyExists = errs.Class("used serial already exists in store")

	mon                   = monkit.Package()
	monAdd                = mon.Task()
	monDeleteExpired      = mon.Task()
	monDeleteRandomSerial = mon.Task()
)

const (
	// PartialSize is the size of a partial serial number.
	PartialSize = memory.Size(len(Partial{}))
	// FullSize is the size of a full serial number.
	FullSize = memory.Size(len(storj.SerialNumber{}))
)

// Partial represents the last 8 bytes of a serial number. It is used when the first 8 are based on the expiration date.
type Partial [8]byte

// Less returns true if partial serial a is less than partial serial b and false otherwise.
func (a Partial) Less(b Partial) bool {
	return binary.BigEndian.Uint64(a[:]) < binary.BigEndian.Uint64(b[:])
}

// Full is a copy of the SerialNumber type. It is necessary so we can define a Less function on it.
type Full storj.SerialNumber

// Less returns true if partial serial a is less than partial serial b and false otherwise.
func (a Full) Less(b Full) bool {
	return binary.BigEndian.Uint64(a[:]) < binary.BigEndian.Uint64(b[:])
}

// serialsList is a structure that contains a list of partial serials and a list of full serials.
//
// For serials where expiration time is the first 8 bytes, it uses partialSerials.
// It uses fullSerials otherwise.
type serialsList struct {
	partialSerials map[Partial]struct{}
	fullSerials    map[storj.SerialNumber]struct{}
}

// Table is an in-memory store for serial numbers.
type Table struct {
	mu sync.Mutex

	// key 1: satellite ID, key 2: expiration hour (in unix time), value: a list of serial numbers
	serials map[storj.NodeID]map[int64]serialsList

	maxMemory  memory.Size
	memoryUsed memory.Size
}

// NewTable creates and returns a new usedserials in-memory store.
func NewTable(maxMemory memory.Size) *Table {
	if maxMemory <= 0 {
		panic("max memory for usedserials store is 0")
	}
	return &Table{
		serials:   make(map[storj.NodeID]map[int64]serialsList),
		maxMemory: maxMemory,
	}
}

// Add adds a serial to the store, or returns an error if the serial number was already added.
// It randomly deletes items from the store if the set maxMemory is exceeded.
func (table *Table) Add(ctx context.Context, satelliteID storj.NodeID, serialNumber storj.SerialNumber, expiration time.Time) (err error) {
	defer monAdd(&ctx)(&err)

	table.mu.Lock()
	defer table.mu.Unlock()

	satMap, ok := table.serials[satelliteID]
	if !ok {
		satMap = make(map[int64]serialsList)
		table.serials[satelliteID] = satMap
	}

	expirationHour := ceilExpirationHour(expiration)
	list, ok := satMap[expirationHour]
	if !ok {
		list = serialsList{}
		satMap[expirationHour] = list
	}

	// determine whether we can use a partial serial number
	partialSerial, usePartial := tryTruncate(serialNumber, expiration)

	if usePartial {
		if list.partialSerials == nil {
			list.partialSerials = map[Partial]struct{}{}
		}

		err = insertPartial(list.partialSerials, partialSerial)
		if err != nil {
			return err
		}

		table.serials[satelliteID][expirationHour] = list
		table.memoryUsed += PartialSize
	} else {
		if list.fullSerials == nil {
			list.fullSerials = map[storj.SerialNumber]struct{}{}
		}
		err = insertSerial(list.fullSerials, serialNumber)
		if err != nil {
			return err
		}

		table.serials[satelliteID][expirationHour] = list
		table.memoryUsed += FullSize
	}

	// Check to see if the structure exceeds the max allowed size.
	// If so, delete random items until there is enough space.
	for table.memoryUsed > table.maxMemory {
		err = table.deleteRandomSerial(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

// DeleteExpired deletes expired serial numbers if their expiration hour has passed.
func (table *Table) DeleteExpired(ctx context.Context, now time.Time) {
	defer monDeleteExpired(&ctx)(nil)

	table.mu.Lock()
	defer table.mu.Unlock()

	partialToDelete := 0
	fullToDelete := 0
	for _, satMap := range table.serials {
		for expirationHour, list := range satMap {
			if expirationHour < now.Unix() {
				partialToDelete += len(list.partialSerials)
				fullToDelete += len(list.fullSerials)

				delete(satMap, expirationHour)
			}
		}
	}

	table.memoryUsed -= memory.Size(partialToDelete) * PartialSize
	table.memoryUsed -= memory.Size(fullToDelete) * FullSize
}

// Exists determines whether a serial number exists in the table.
func (table *Table) Exists(satelliteID storj.NodeID, serialNumber storj.SerialNumber, expiration time.Time) bool {
	table.mu.Lock()
	defer table.mu.Unlock()

	expirationHour := ceilExpirationHour(expiration)
	serialsList := table.serials[satelliteID][expirationHour]

	partial, usePartial := tryTruncate(serialNumber, expiration)
	if usePartial {
		_, exists := serialsList.partialSerials[partial]
		return exists
	} else {
		_, exists := serialsList.fullSerials[serialNumber]
		return exists
	}
}

// Count iterates over all the items in the table and returns the number.
func (table *Table) Count() int {
	table.mu.Lock()
	defer table.mu.Unlock()

	count := 0
	for _, satMap := range table.serials {
		for _, serialsList := range satMap {
			count += len(serialsList.fullSerials)
			count += len(serialsList.partialSerials)
		}
	}

	return count
}

// deleteRandomSerial deletes a random item.
// It expects the mutex to be locked before being called.
func (table *Table) deleteRandomSerial(ctx context.Context) (err error) {
	defer monDeleteRandomSerial(&ctx)(&err)

	mon.Meter("delete_random_serial").Mark(1)

	for _, satMap := range table.serials {
		for _, serialList := range satMap {
			if len(serialList.partialSerials) > 0 {
				for k := range serialList.partialSerials {
					delete(serialList.partialSerials, k)
					table.memoryUsed -= PartialSize
					break
				}
				return nil
			} else if len(serialList.fullSerials) > 0 {
				for k := range serialList.fullSerials {
					delete(serialList.fullSerials, k)
					table.memoryUsed -= FullSize
					break
				}
				return nil
			}
		}
	}
	// we should never get to this path unless config.MaxTableSize is 0
	return ErrSerials.New("could not delete a random item")
}

// insertPartial inserts a partial serial in the correct position in a sorted list,
// or returns an error if it is already in the list.
func insertPartial(list map[Partial]struct{}, serial Partial) error {
	_, ok := list[serial]
	if ok {
		return ErrSerialAlreadyExists.New("")
	}
	list[serial] = struct{}{}
	return nil
}

// insertSerial inserts a serial in the correct position in a sorted list,
// or returns an error if it is already in the list.
func insertSerial(list map[storj.SerialNumber]struct{}, serial storj.SerialNumber) error {
	_, ok := list[serial]
	if ok {
		return ErrSerialAlreadyExists.New("")
	}
	list[serial] = struct{}{}
	return nil
}

func tryTruncate(serial storj.SerialNumber, expiration time.Time) (partial Partial, succeeded bool) {
	// If the first 8 bytes of the serial number are based on the expiration date
	// then we can use a partial serial number with the last 8 bytes.
	// Otherwise, we need to use the full serial number.
	// see satellite/orders/service.go, createSerial() for how expiration date is used in the serial number.
	if binary.BigEndian.Uint64(serial[0:8]) == uint64(expiration.Unix()) {
		partialSerial := Partial{}
		copy(partialSerial[:], serial[8:])
		return partialSerial, true
	}

	return Partial{}, false
}

func ceilExpirationHour(expiration time.Time) int64 {
	// time.Truncate rounds down; adding (Hour-Nanosecond) ensures that we round down to the actual expiration hour
	return expiration.Add(time.Hour - time.Nanosecond).Truncate(time.Hour).Unix()
}

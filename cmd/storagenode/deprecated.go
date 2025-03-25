// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"reflect"
	"strconv"
	"time"

	"go.uber.org/zap"
)

// Deprecated contains deprecated config structs.
type Deprecated struct {
	Kademlia struct {
		ExternalAddress string `default:"" hidden:"true"`
		Operator        struct {
			Email  string `default:"" hidden:"true"`
			Wallet string `default:"" hidden:"true"`
		}
	}
	Storage2 struct {
		Monitor struct {
			VerifyDirInterval string `default:"" hidden:"true"`
		}
	}
}

// maps deprecated config values to new values if applicable.
func mapDeprecatedConfigs(log *zap.Logger, cfg *StorageNodeFlags) {
	if cfg == nil {
		return
	}

	type migration struct {
		newValue        interface{}
		newConfigString string
		oldValue        string
		oldConfigString string
	}
	migrations := []migration{
		{
			newValue:        &cfg.Contact.ExternalAddress,
			newConfigString: "contact.external-address",
			oldValue:        cfg.Deprecated.Kademlia.ExternalAddress,
			oldConfigString: "kademlia.external-address",
		},
		{
			newValue:        &cfg.Operator.Wallet,
			newConfigString: "operator.wallet",
			oldValue:        cfg.Deprecated.Kademlia.Operator.Wallet,
			oldConfigString: "kademlia.operator.wallet",
		},
		{
			newValue:        &cfg.Operator.Email,
			newConfigString: "operator.email",
			oldValue:        cfg.Deprecated.Kademlia.Operator.Email,
			oldConfigString: "kademlia.operator.email",
		},
		{
			newValue:        &cfg.Config.Storage2.Monitor.VerifyDirReadableInterval,
			newConfigString: "storage2.monitor.verify-dir-readable-interval",
			oldValue:        cfg.Deprecated.Storage2.Monitor.VerifyDirInterval,
			oldConfigString: "storage2.monitor.verify-dir-interval",
		},
	}

	for _, migration := range migrations {
		if migration.oldValue != "" {
			typ := reflect.TypeOf(migration.newValue).Elem()
			override := parseOverride(typ, migration.oldValue)

			reflect.ValueOf(migration.newValue).Elem().Set(reflect.ValueOf(override))
			log.Debug("Found deprecated flag. Migrating value.",
				zap.Stringer("Value", reflect.ValueOf(migration.newValue).Elem()),
				zap.String("From", migration.oldConfigString),
				zap.String("To", migration.newConfigString),
			)
		}
	}
}

func parseOverride(typ reflect.Type, value string) interface{} {
	switch typ {
	case reflect.TypeOf(int(0)):
		val, err := strconv.ParseInt(value, 0, strconv.IntSize)
		if err != nil {
			panic(err)
		}
		return int(val)
	case reflect.TypeOf(int64(0)):
		val, err := strconv.ParseInt(value, 0, 64)
		if err != nil {
			panic(err)
		}
		return val
	case reflect.TypeOf(uint(0)):
		val, err := strconv.ParseUint(value, 0, strconv.IntSize)
		if err != nil {
			panic(err)
		}
		return uint(val)
	case reflect.TypeOf(uint64(0)):
		val, err := strconv.ParseUint(value, 0, 64)
		if err != nil {
			panic(err)
		}
		return val
	case reflect.TypeOf(time.Duration(0)):
		val, err := time.ParseDuration(value)
		if err != nil {
			panic(err)
		}
		return val
	case reflect.TypeOf(float64(0)):
		val, err := strconv.ParseFloat(value, 64)
		if err != nil {
			panic(err)
		}
		return val
	case reflect.TypeOf(string("")):
		return value
	case reflect.TypeOf(bool(false)):
		val, err := strconv.ParseBool(value)
		if err != nil {
			panic(err)
		}
		return val
	default:
		panic("invalid field type: " + typ.String())
	}
}

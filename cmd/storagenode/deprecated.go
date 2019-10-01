// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"reflect"
	"strconv"
	"time"

	"go.uber.org/zap"
)

// Deprecated contains deprecated config structs
type Deprecated struct {
	Kademlia struct {
		ExternalAddress string `default:"undefined"`
		Operator        struct {
			Email  string `default:"undefined"`
			Wallet string `default:"undefined"`
		}
	}
}

// maps deprecated config values to new values if applicable
func mapDeprecatedConfigs(log *zap.Logger) {
	type config struct {
		new     interface{}
		newFlag string
		old     string
		oldFlag string
	}
	configs := []config{
		{
			new:     runCfg.Contact.ExternalAddress,
			newFlag: "contact.external-address",
			old:     runCfg.Kademlia.ExternalAddress,
			oldFlag: "kademlia.external-address",
		},
		{
			new:     runCfg.Operator.Wallet,
			newFlag: "operator.wallet",
			old:     runCfg.Kademlia.Operator.Wallet,
			oldFlag: "kademlia.operator.wallet",
		},
		{
			new:     runCfg.Operator.Email,
			newFlag: "operator.email",
			old:     runCfg.Kademlia.Operator.Email,
			oldFlag: "kademlia.operator.email",
		},
	}

	for _, config := range configs {
		if config.old != "undefined" {
			overwrite(&config.new, config.old)
			log.Sugar().Warnf("Found deprecated flag. Migrating value %v from %s to %s", config.new, config.oldFlag, config.newFlag)
		}
	}
}

func overwrite(field *interface{}, value string) {
	switch reflect.TypeOf(*field) {
	case reflect.TypeOf(int(0)):
		val, err := strconv.ParseInt(value, 0, strconv.IntSize)
		if err != nil {
			panic(err)
		}
		*field = val
	case reflect.TypeOf(int64(0)):
		val, err := strconv.ParseInt(value, 0, 64)
		if err != nil {
			panic(err)
		}
		*field = val
	case reflect.TypeOf(uint(0)):
		val, err := strconv.ParseUint(value, 0, strconv.IntSize)
		if err != nil {
			panic(err)
		}
		*field = val
	case reflect.TypeOf(uint64(0)):
		val, err := strconv.ParseUint(value, 0, 64)
		if err != nil {
			panic(err)
		}
		*field = val
	case reflect.TypeOf(time.Duration(0)):
		val, err := time.ParseDuration(value)
		if err != nil {
			panic(err)
		}
		*field = val
	case reflect.TypeOf(float64(0)):
		val, err := strconv.ParseFloat(value, 64)
		if err != nil {
			panic(err)
		}
		*field = val
	case reflect.TypeOf(string("")):
		*field = value
	case reflect.TypeOf(bool(false)):
		val, err := strconv.ParseBool(value)
		if err != nil {
			panic(err)
		}
		*field = val
	default:
		panic(fmt.Sprintf("invalid field type: %s", reflect.TypeOf(field).String()))
	}
}

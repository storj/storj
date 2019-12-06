// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pb_test

import (
	fmt "fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/pb"
)

func TestCompatibility(t *testing.T) {
	// when these fail, the X and XSigning definitions are out of sync
	// remember to update the conversions in auth/signing
	check(t, pb.OrderLimit{}, pb.OrderLimitSigning{})
	check(t, pb.Order{}, pb.OrderSigning{})
	check(t, pb.PieceHash{}, pb.PieceHashSigning{})
}

func check(t *testing.T, a, b interface{}) {
	afields := fields(a)
	bfields := fields(b)
	assert.Equal(t, afields, bfields, fmt.Sprintf("%T and %T definitions don't match", a, b))
}

type Field struct {
	Name  string
	Type  string
	Index string
}

func fields(v interface{}) []Field {
	t := reflect.ValueOf(v).Type()
	if t.Kind() != reflect.Struct {
		panic(t.Kind())
	}

	var fields []Field
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		tag := f.Tag.Get("protobuf")
		if tag == "" {
			continue
		}
		tags := strings.Split(tag, ",")
		fields = append(fields, Field{
			Name:  f.Name,
			Type:  tags[0],
			Index: tags[1],
		})
	}

	sort.Slice(fields, func(i, k int) bool {
		return fields[i].Name < fields[k].Name
	})

	return fields
}

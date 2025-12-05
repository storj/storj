// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil

import (
	"strconv"

	"github.com/zeebo/errs"
)

// inty requires that a type has some form of signed or unsigned integer as an underlying type.
type inty interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

// IntValueDecoder is a type wrapping an int pointer so it can decode integer values from
// Spanner directly (Spanner prefers to work only in int64s).
type IntValueDecoder[T inty] struct {
	pointer *T
}

// DecodeSpanner decodes a value from a Spanner-stored type to the appropriate int type.
// It implements spanner.Decoder.
func (s IntValueDecoder[T]) DecodeSpanner(input any) error {
	if sVal, ok := input.(*string); ok {
		if sVal == nil {
			*s.pointer = 0
			return nil
		}
		input = *sVal
	}

	if sVal, ok := input.(string); ok {
		iVal, err := strconv.ParseInt(sVal, 10, 64)
		if err != nil {
			return err
		}
		if int64(T(iVal)) != iVal {
			return errs.New("value out of bounds for %T: %d", T(0), iVal)
		}
		*s.pointer = T(iVal)
		return nil
	}
	return errs.New("unable to decode %T to %T", input, T(0))
}

// Int wraps a pointer to an int-based type in a type that can be decoded directly from Spanner.
//
// In general, it is preferable to add EncodeSpanner/DecodeSpanner methods to our specialized
// int types, but this can be used for types we don't own or otherwise can't put methods on.
func Int[T inty](val *T) IntValueDecoder[T] {
	return IntValueDecoder[T]{pointer: val}
}

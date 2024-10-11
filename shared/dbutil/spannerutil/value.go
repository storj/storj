// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil

import (
	"encoding/base64"
	"strconv"
	"time"

	"cloud.google.com/go/civil"
	"cloud.google.com/go/spanner/apiv1/spannerpb"
	"golang.org/x/exp/constraints"
	"google.golang.org/protobuf/types/known/structpb"
)

// ArrayOf is a convenience method to construct a Spanner protobuf type for an array of elements.
func ArrayOf(typ *spannerpb.Type) *spannerpb.Type {
	return &spannerpb.Type{
		Code:             spannerpb.TypeCode_ARRAY,
		ArrayElementType: typ,
	}
}

// StructOf is a convenience method to construct a Spanner protobuf type for a struct.
func StructOf(fields ...*spannerpb.StructType_Field) *spannerpb.Type {
	return &spannerpb.Type{
		Code: spannerpb.TypeCode_STRUCT,
		StructType: &spannerpb.StructType{
			Fields: fields,
		},
	}
}

// FieldOf is a convenience method to construct a Spanner protobuf type for a field of a struct.
func FieldOf(name string, typ *spannerpb.Type) *spannerpb.StructType_Field {
	return &spannerpb.StructType_Field{
		Name: name,
		Type: typ,
	}
}

// The following EncodeXToValue methods follow what the official google cloud Go package does for encoding values
// in protobufs to send to Spanner. See the encodeValue method here for more details:
// https://github.com/googleapis/google-cloud-go/blob/4927b533dd352bee0c3efdc3a88ec96279532e64/spanner/value.go#L4018

// EncodeBytesToValue encodes a bytes to what Spanner expects via protobuf.
func EncodeBytesToValue(bytes []byte) *structpb.Value {
	return structpb.NewStringValue(base64.StdEncoding.EncodeToString(bytes))
}

// EncodeTimeToValue encodes a time.Time to what Spanner expects via protobuf.
func EncodeTimeToValue(t time.Time) *structpb.Value {
	return structpb.NewStringValue(t.UTC().Format(time.RFC3339))
}

// EncodeIntToValue encodes any integer type to what Spanner expects via protobuf.
func EncodeIntToValue[T constraints.Integer](i T) *structpb.Value {
	return structpb.NewStringValue(strconv.FormatInt(int64(i), 10))
}

// EncodeFloat64ToValue encodes any float64 type to what Spanner expects via protobuf.
func EncodeFloat64ToValue(i float64) *structpb.Value {
	return structpb.NewNumberValue(i)
}

// EncodeDateToValue encodes civil.Date type to what Spanner expects via protobuf.
func EncodeDateToValue(date civil.Date) *structpb.Value {
	return structpb.NewStringValue(date.String())
}

// EncodeStringToValue encodes string type to what Spanner expects via protobuf.
func EncodeStringToValue(str string) *structpb.Value {
	return structpb.NewStringValue(str)
}

// BytesType is a convenience method to define a Spanner BYTES value.
func BytesType() *spannerpb.Type { return &spannerpb.Type{Code: spannerpb.TypeCode_BYTES} }

// DateType is a convenience method to define a Spanner DATE value.
func DateType() *spannerpb.Type { return &spannerpb.Type{Code: spannerpb.TypeCode_DATE} }

// Int64Type is a convenience method to define a Spanner INT64 value.
func Int64Type() *spannerpb.Type { return &spannerpb.Type{Code: spannerpb.TypeCode_INT64} }

// Float64Type is a convenience method to define a Spanner FLOAT64 value.
func Float64Type() *spannerpb.Type { return &spannerpb.Type{Code: spannerpb.TypeCode_FLOAT64} }

// StringType is a convenience method to define a Spanner STRING value.
func StringType() *spannerpb.Type { return &spannerpb.Type{Code: spannerpb.TypeCode_STRING} }

// TimestampType is a convenience method to define a Spanner STRING value.
func TimestampType() *spannerpb.Type { return &spannerpb.Type{Code: spannerpb.TypeCode_TIMESTAMP} }

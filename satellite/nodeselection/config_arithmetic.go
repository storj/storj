// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"math"
	"reflect"

	"github.com/jtolio/mito"
	"github.com/zeebo/errs"
)

func convertType(b any, t reflect.Type) (any, error) {
	switch t {
	case reflect.TypeOf(1):
		switch bv := b.(type) {
		case int:
			return bv, nil
		case int64:
			return int(bv), nil
		}
	case reflect.TypeOf(float64(1)):
		switch bv := b.(type) {
		case float64:
			return bv, nil
		case float32:
			return float64(bv), nil
		case int:
			return float64(bv), nil
		case int64:
			return float64(bv), nil
		}
	case reflect.TypeOf(NodeValue(nil)):
		switch bv := b.(type) {
		case NodeValue:
			return bv, nil
		case float64:
			return NodeValue(func(node SelectedNode) float64 {
				return bv
			}), nil
		case float32:
			return NodeValue(func(node SelectedNode) float64 {
				return float64(bv)
			}), nil
		case int:
			return NodeValue(func(node SelectedNode) float64 {
				return float64(bv)
			}), nil
		case int64:
			return NodeValue(func(node SelectedNode) float64 {
				return float64(bv)
			}), nil
		}
	}
	return nil, errs.New("unsupported type conversion: %T -> %v", b, t)
}

func targetType(a any, b any) reflect.Type {
	at := reflect.TypeOf(a)
	bt := reflect.TypeOf(b)

	// highest priority is the first. Lowest priority is int
	supportedTypes := []reflect.Type{
		reflect.TypeOf(ScoreNode(nil)),
		reflect.TypeOf(NodeValue(nil)),
		reflect.TypeOf(float64(0)),
	}

	for _, t := range supportedTypes {
		if at == t || bt == t {
			return t
		}
	}
	return reflect.TypeOf(1)
}

func addArithmetic(in map[any]interface{}) map[any]interface{} {
	in[mito.OpExp] = func(env map[any]any, a, b any) (val any, err error) {
		targetType := targetType(a, b)
		a, err = convertType(a, targetType)
		if err != nil {
			return nil, err
		}
		b, err = convertType(b, targetType)
		if err != nil {
			return nil, err
		}
		switch targetType {
		case reflect.TypeOf(1):
			return math.Pow(float64(a.(int)), float64(b.(int))), nil
		case reflect.TypeOf(int64(1)):
			return math.Pow(float64(a.(int64)), float64(b.(int64))), nil
		case reflect.TypeOf(float64(1)):
			return math.Pow(a.(float64), b.(float64)), nil
		case reflect.TypeOf(NodeValue(nil)):
			return NodeValue(func(node SelectedNode) float64 {
				av := a.(NodeValue)(node)
				bv := b.(NodeValue)(node)
				return math.Pow(float64(av), float64(bv))
			}), nil
		default:
			return nil, errs.New("unsupported type for exponentiation: %T", a)
		}
	}
	in[mito.OpAdd] = func(env map[any]any, a, b any) (val any, err error) {
		targetType := targetType(a, b)
		a, err = convertType(a, targetType)
		if err != nil {
			return nil, err
		}
		b, err = convertType(b, targetType)
		if err != nil {
			return nil, err
		}
		switch targetType {
		case reflect.TypeOf(1):
			return a.(int) + b.(int), nil
		case reflect.TypeOf(int64(1)):
			return a.(int64) + b.(int64), nil
		case reflect.TypeOf(float64(1)):
			return a.(float64) + b.(float64), nil
		case reflect.TypeOf(NodeValue(nil)):
			return NodeValue(func(node SelectedNode) float64 {
				return a.(NodeValue)(node) + b.(NodeValue)(node)
			}), nil
		default:
			return nil, errs.New("unsupported type for exponentiation: %T", a)
		}
	}
	in[mito.OpSub] = func(env map[any]any, a, b any) (val any, err error) {
		targetType := targetType(a, b)
		a, err = convertType(a, targetType)
		if err != nil {
			return nil, err
		}
		b, err = convertType(b, targetType)
		if err != nil {
			return nil, err
		}
		switch targetType {
		case reflect.TypeOf(1):
			return a.(int) - b.(int), nil
		case reflect.TypeOf(int64(1)):
			return a.(int64) - b.(int64), nil
		case reflect.TypeOf(float64(1)):
			return a.(float64) - b.(float64), nil
		case reflect.TypeOf(NodeValue(nil)):
			return NodeValue(func(node SelectedNode) float64 {
				return a.(NodeValue)(node) - b.(NodeValue)(node)
			}), nil
		default:
			return nil, errs.New("unsupported type for exponentiation: %T", a)
		}
	}
	in[mito.OpMul] = func(env map[any]any, a, b any) (val any, err error) {
		targetType := targetType(a, b)
		a, err = convertType(a, targetType)
		if err != nil {
			return nil, err
		}
		b, err = convertType(b, targetType)
		if err != nil {
			return nil, err
		}
		switch targetType {
		case reflect.TypeOf(1):
			return a.(int) * b.(int), nil
		case reflect.TypeOf(int64(1)):
			return a.(int64) * b.(int64), nil
		case reflect.TypeOf(float64(1)):
			return a.(float64) * b.(float64), nil
		case reflect.TypeOf(NodeValue(nil)):
			return NodeValue(func(node SelectedNode) float64 {
				return a.(NodeValue)(node) * b.(NodeValue)(node)
			}), nil
		default:
			return nil, errs.New("unsupported type for exponentiation: %T", a)
		}
	}
	in[mito.OpDiv] = func(env map[any]any, a, b any) (val any, err error) {
		targetType := targetType(a, b)
		a, err = convertType(a, targetType)
		if err != nil {
			return nil, err
		}
		b, err = convertType(b, targetType)
		if err != nil {
			return nil, err
		}
		switch targetType {
		case reflect.TypeOf(1):
			return a.(int) / b.(int), nil
		case reflect.TypeOf(int64(1)):
			return a.(int64) / b.(int64), nil
		case reflect.TypeOf(float64(1)):
			return a.(float64) / b.(float64), nil
		case reflect.TypeOf(NodeValue(nil)):
			return NodeValue(func(node SelectedNode) float64 {
				return a.(NodeValue)(node) / b.(NodeValue)(node)
			}), nil
		default:
			return nil, errs.New("unsupported type for exponentiation: %T", a)
		}
	}
	return in
}

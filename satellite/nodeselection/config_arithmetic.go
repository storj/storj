// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"math"
	"reflect"

	"github.com/jtolio/mito"
	"github.com/zeebo/errs"

	"storj.io/common/storj"
)

// ConvertType tries to convert a type for the most generic mathmetical type which supports math operations.
func ConvertType(b any, t reflect.Type) (any, error) {
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
	case reflect.TypeOf(new(ScoreNode)).Elem():
		switch bv := b.(type) {
		case ScoreNodeFunc, ScoreNode:
			return bv, nil
		case NodeValue:
			return ScoreNodeFunc(func(uplink storj.NodeID, node *SelectedNode) float64 {
				return bv(*node)
			}), nil
		case float64:
			return ScoreNodeFunc(func(uplink storj.NodeID, node *SelectedNode) float64 {
				return bv
			}), nil
		case float32:
			return ScoreNodeFunc(func(uplink storj.NodeID, node *SelectedNode) float64 {
				return float64(bv)
			}), nil
		case int:
			return ScoreNodeFunc(func(uplink storj.NodeID, node *SelectedNode) float64 {
				return float64(bv)
			}), nil
		case int64:
			return ScoreNodeFunc(func(uplink storj.NodeID, node *SelectedNode) float64 {
				return float64(bv)
			}), nil
		}
	}
	return nil, errs.New("unsupported type conversion: %T -> %v", b, t)
}

func targetType(a any, b any) reflect.Type {
	at := reflect.TypeOf(a)
	bt := reflect.TypeOf(b)

	snType := reflect.TypeOf(new(ScoreNode)).Elem()

	if at.Implements(snType) || bt.Implements(snType) {
		return snType
	}

	// highest priority is the first. Lowest priority is int
	supportedTypes := []reflect.Type{
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

// AddArithmetic defines generic math operation for various types.
func AddArithmetic(in map[any]interface{}) map[any]interface{} {
	in[mito.OpExp] = func(env map[any]any, a, b any) (val any, err error) {
		targetType := targetType(a, b)
		a, err = ConvertType(a, targetType)
		if err != nil {
			return nil, err
		}
		b, err = ConvertType(b, targetType)
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
		case reflect.TypeOf(ScoreNode(nil)):
			return ScoreNodeFunc(func(uplink storj.NodeID, node *SelectedNode) float64 {
				av := a.(ScoreNode).Get(uplink)(node)
				bv := b.(ScoreNode).Get(uplink)(node)
				return math.Pow(av, bv)
			}), nil
		default:
			return nil, errs.New("unsupported type for exponentiation: %T", a)
		}
	}
	in[mito.OpAdd] = func(env map[any]any, a, b any) (val any, err error) {
		targetType := targetType(a, b)
		a, err = ConvertType(a, targetType)
		if err != nil {
			return nil, err
		}
		b, err = ConvertType(b, targetType)
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
		case reflect.TypeOf(new(ScoreNode)).Elem():
			return ScoreNodeFunc(func(uplink storj.NodeID, node *SelectedNode) float64 {
				av := a.(ScoreNode).Get(uplink)(node)
				bv := b.(ScoreNode).Get(uplink)(node)
				return av + bv
			}), nil
		default:
			return nil, errs.New("unsupported type for exponentiation: %T", a)
		}
	}
	in[mito.OpSub] = func(env map[any]any, a, b any) (val any, err error) {
		targetType := targetType(a, b)
		a, err = ConvertType(a, targetType)
		if err != nil {
			return nil, err
		}
		b, err = ConvertType(b, targetType)
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
		case reflect.TypeOf(new(ScoreNode)).Elem():
			return ScoreNodeFunc(func(uplink storj.NodeID, node *SelectedNode) float64 {
				av := a.(ScoreNode).Get(uplink)(node)
				bv := b.(ScoreNode).Get(uplink)(node)
				return av - bv
			}), nil
		default:
			return nil, errs.New("unsupported type for exponentiation: %T", a)
		}
	}
	in[mito.OpMul] = func(env map[any]any, a, b any) (val any, err error) {
		targetType := targetType(a, b)
		a, err = ConvertType(a, targetType)
		if err != nil {
			return nil, err
		}
		b, err = ConvertType(b, targetType)
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
		case reflect.TypeOf(new(ScoreNode)).Elem():
			return ScoreNodeFunc(func(uplink storj.NodeID, node *SelectedNode) float64 {
				av := a.(ScoreNode).Get(uplink)(node)
				bv := b.(ScoreNode).Get(uplink)(node)
				return av * bv
			}), nil
		default:
			return nil, errs.New("unsupported type for exponentiation: %T", a)
		}
	}
	in[mito.OpDiv] = func(env map[any]any, a, b any) (val any, err error) {
		targetType := targetType(a, b)
		a, err = ConvertType(a, targetType)
		if err != nil {
			return nil, err
		}
		b, err = ConvertType(b, targetType)
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
		case reflect.TypeOf(new(ScoreNode)).Elem():
			return ScoreNodeFunc(func(uplink storj.NodeID, node *SelectedNode) float64 {
				av := a.(ScoreNode).Get(uplink)(node)
				bv := b.(ScoreNode).Get(uplink)(node)
				return av / bv
			}), nil
		default:
			return nil, errs.New("unsupported type for division: %T", a)
		}
	}
	in["max"] = func(a, b any) (val any, err error) {
		targetType := targetType(a, b)
		a, err = ConvertType(a, targetType)
		if err != nil {
			return nil, err
		}
		b, err = ConvertType(b, targetType)
		if err != nil {
			return nil, err
		}
		switch targetType {
		case reflect.TypeOf(1):
			if a.(int) > b.(int) {
				return a.(int), nil
			}
			return b.(int), nil
		case reflect.TypeOf(int64(1)):
			if a.(int64) > b.(int64) {
				return a.(int64), nil
			}
			return b.(int64), nil
		case reflect.TypeOf(float64(1)):
			if a.(float64) > b.(float64) {
				return a.(float64), nil
			}
			return b.(float64), nil
		case reflect.TypeOf(NodeValue(nil)):
			return NodeValue(func(node SelectedNode) float64 {
				av := a.(NodeValue)(node)
				bv := b.(NodeValue)(node)
				if av > bv {
					return av
				}
				return bv
			}), nil
		case reflect.TypeOf(new(ScoreNode)).Elem():
			return ScoreNodeFunc(func(uplink storj.NodeID, node *SelectedNode) float64 {
				av := a.(ScoreNode).Get(uplink)(node)
				bv := b.(ScoreNode).Get(uplink)(node)
				if av > bv {
					return av
				}
				return bv
			}), nil
		default:
			return nil, errs.New("unsupported type for exponentiation: %T", a)
		}
	}
	in["min"] = func(a, b any) (val any, err error) {
		targetType := targetType(a, b)
		a, err = ConvertType(a, targetType)
		if err != nil {
			return nil, err
		}
		b, err = ConvertType(b, targetType)
		if err != nil {
			return nil, err
		}
		switch targetType {
		case reflect.TypeOf(1):
			if a.(int) < b.(int) {
				return a.(int), nil
			}
			return b.(int), nil
		case reflect.TypeOf(int64(1)):
			if a.(int64) < b.(int64) {
				return a.(int64), nil
			}
			return b.(int64), nil
		case reflect.TypeOf(float64(1)):
			if a.(float64) < b.(float64) {
				return a.(float64), nil
			}
			return b.(float64), nil
		case reflect.TypeOf(NodeValue(nil)):
			return NodeValue(func(node SelectedNode) float64 {
				av := a.(NodeValue)(node)
				bv := b.(NodeValue)(node)
				if av < bv {
					return av
				}
				return bv
			}), nil
		case reflect.TypeOf(new(ScoreNode)).Elem():
			return ScoreNodeFunc(func(uplink storj.NodeID, node *SelectedNode) float64 {
				av := a.(ScoreNode).Get(uplink)(node)
				bv := b.(ScoreNode).Get(uplink)(node)
				if av < bv {
					return av
				}
				return bv
			}), nil
		default:
			return nil, errs.New("unsupported type for exponentiation: %T", a)
		}
	}
	in["round"] = func(a any) (val any, err error) {
		switch av := a.(type) {
		case int:
			return av, nil
		case int64:
			return av, nil
		case float64:
			return math.Round(av), nil
		case float32:
			return float32(math.Round(float64(av))), nil
		case NodeValue:
			return NodeValue(func(node SelectedNode) float64 {
				return math.Round(av(node))
			}), nil
		case ScoreNodeFunc:
			return ScoreNodeFunc(func(uplink storj.NodeID, node *SelectedNode) float64 {
				return math.Round(av(uplink, node))
			}), nil
		case ScoreNode:
			return ScoreNodeFunc(func(uplink storj.NodeID, node *SelectedNode) float64 {
				return math.Round(av.Get(uplink)(node))
			}), nil

		default:
			return nil, errs.New("unsupported type for round: %T", a)
		}
	}
	return in
}

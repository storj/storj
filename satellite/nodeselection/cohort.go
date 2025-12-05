// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"slices"
	"strconv"

	"github.com/jtolio/mito"
	"github.com/zeebo/errs"

	"storj.io/common/pb"
)

// CohortRequirements is a configured set of cohort requirements for uploads.
type CohortRequirements pb.CohortRequirements

// CohortName will generate a name for a specific cohort. For example, let's
// say that we want the Uplink to consider unique racks across data centers
// as individual cohorts? We want all nodes that share the same DC and Rack to
// have the same cohort name. Perhaps we have DCs ewr1 and fra2, and in each
// one, racks 1, 2, and 3. So there is a node tag of "dc" with values "ewr1"
// and another node tag of "rack" with value "1". So what a cohort name might
// be is "ewr1-1" or "fra2-3". This is configured in the cohort requirements
// definition as `tag($SIGNER_ZERO, "dc") + "-" + tag($SIGNER_ZERO, "rack")`.
type CohortName func(n SelectedNode) []byte

// CohortRequirementsFromString parses a cohort requirement definition.
func CohortRequirementsFromString(val string) (*CohortRequirements, map[string]CohortName, error) {
	if val == "" {
		return nil, map[string]CohortName{}, nil
	}

	names := map[string]CohortName{}

	parseAttr := func(attr string) (CohortName, error) {
		nodeAddr, err := CreateNodeAttribute(attr)
		if err != nil {
			return nil, err
		}

		return func(n SelectedNode) []byte {
			return []byte(nodeAddr(n))
		}, nil
	}

	requirementAnd := func(x *pb.CohortRequirements, y *pb.CohortRequirements) *pb.CohortRequirements {
		return &pb.CohortRequirements{
			Requirement: &pb.CohortRequirements_And_{
				And: &pb.CohortRequirements_And{
					Requirements: []*pb.CohortRequirements{x, y},
				},
			},
		}
	}

	requirementLiteral := func(val int64) *pb.CohortRequirements {
		return &pb.CohortRequirements{
			Requirement: &pb.CohortRequirements_Literal_{
				Literal: &pb.CohortRequirements_Literal{
					Value: int32(val),
				},
			},
		}
	}

	requirementWithhold := func(group CohortName, amount int64, child *pb.CohortRequirements) *pb.CohortRequirements {
		name := strconv.Itoa(len(names))
		names[name] = group
		return &pb.CohortRequirements{
			Requirement: &pb.CohortRequirements_Withhold_{
				Withhold: &pb.CohortRequirements_Withhold{
					TagKey: name,
					Amount: int32(amount),
					Child:  child,
				},
			},
		}
	}

	parsed, err := mito.Eval(val, map[any]any{
		"attr":     parseAttr,
		"and":      requirementAnd,
		"min":      requirementLiteral,
		"withhold": requirementWithhold,
		mito.OpAnd: func(env map[any]any, a, b any) (any, error) {
			left, ok1 := a.(*pb.CohortRequirements)
			right, ok2 := b.(*pb.CohortRequirements)
			if !ok1 || !ok2 {
				return nil, ErrPlacement.New("&& is supported only between CohortRequirement instances, got %T && %T", a, b)
			}
			return requirementAnd(left, right), nil
		},
		mito.OpAdd: func(env map[any]any, a, b any) (any, error) {
			switch av := a.(type) {
			default:
				return nil, ErrPlacement.New("+ is supported only between strings and tag definitions, got %T + %T", a, b)
			case string:
				switch bv := b.(type) {
				default:
					return nil, ErrPlacement.New("+ is supported only between strings and tag definitions, got %T + %T", a, b)
				case string:
					return av + bv, nil
				case []byte:
					return av + string(bv), nil
				case CohortName:
					return CohortName(func(n SelectedNode) []byte {
						return slices.Concat([]byte(av), bv(n))
					}), nil
				}
			case []byte:
				switch bv := b.(type) {
				default:
					return nil, ErrPlacement.New("+ is supported only between strings and tag definitions, got %T + %T", a, b)
				case string:
					return string(av) + bv, nil
				case []byte:
					return slices.Concat(av, bv), nil
				case CohortName:
					return CohortName(func(n SelectedNode) []byte {
						return slices.Concat(av, bv(n))
					}), nil
				}
			case CohortName:
				switch bv := b.(type) {
				default:
					return nil, ErrPlacement.New("+ is supported only between strings and tag definitions, got %T + %T", a, b)
				case string:
					return CohortName(func(n SelectedNode) []byte {
						return slices.Concat(av(n), []byte(bv))
					}), nil
				case []byte:
					return CohortName(func(n SelectedNode) []byte {
						return slices.Concat(av(n), bv)
					}), nil
				case CohortName:
					return CohortName(func(n SelectedNode) []byte {
						return slices.Concat(av(n), bv(n))
					}), nil
				}
			}
		},
	})
	if err != nil {
		return nil, names, err
	}
	if rv, ok := parsed.(*pb.CohortRequirements); ok {
		return (*CohortRequirements)(rv), names, nil
	}
	return nil, names, errs.New("evaluation didn't return cohort requirements: %#v", parsed)
}

// ToProto converts the parsed CohortRequirements to a protobuf.
func (c *CohortRequirements) ToProto() *pb.CohortRequirements {
	return (*pb.CohortRequirements)(c)
}

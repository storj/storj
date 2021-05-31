// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

// TODO: this whole file can be removed in a future PR.

package testplanet

import (
	"reflect"

	"github.com/zeebo/errs"

	"storj.io/storj/satellite/compensation"
)

// showInequality can be removed in a future PR. This is a lightly edited version
// of reflect.DeepEqual but made to show what is different.
func showInequality(v1, v2 reflect.Value) error {
	if !v1.IsValid() || !v2.IsValid() {
		if v1.IsValid() != v2.IsValid() {
			return errs.New("mismatch on validity")
		}
		return nil
	}
	if v1.Type() != v2.Type() {
		return errs.New("type mismatch")
	}

	if v1.CanInterface() {
		if dv1, ok := v1.Interface().(compensation.Rate); ok {
			dv2 := v2.Interface().(compensation.Rate)
			if dv1.String() == dv2.String() {
				return nil
			}
			return errs.New("compensation.Rate mismatch: %q != %q", dv1.String(), dv2.String())
		}
	}

	switch v1.Kind() {
	case reflect.Array:
		for i := 0; i < v1.Len(); i++ {
			if err := showInequality(v1.Index(i), v2.Index(i)); err != nil {
				return err
			}
		}
		return nil
	case reflect.Slice:
		if v1.IsNil() != v2.IsNil() {
			return errs.New("a slice is nil")
		}
		if v1.Len() != v2.Len() {
			return errs.New("slice length mismatch")
		}
		if v1.Pointer() == v2.Pointer() {
			return nil
		}
		for i := 0; i < v1.Len(); i++ {
			if err := showInequality(v1.Index(i), v2.Index(i)); err != nil {
				return err
			}
		}
		return nil
	case reflect.Interface:
		if v1.IsNil() || v2.IsNil() {
			if v1.IsNil() != v2.IsNil() {
				return errs.New("an interface is nil")
			}
			return nil
		}
		return showInequality(v1.Elem(), v2.Elem())
	case reflect.Ptr:
		if v1.Pointer() == v2.Pointer() {
			return nil
		}
		return showInequality(v1.Elem(), v2.Elem())
	case reflect.Struct:
		for i, n := 0, v1.NumField(); i < n; i++ {
			if err := showInequality(v1.Field(i), v2.Field(i)); err != nil {
				return errs.New("struct field %q: %+v", v1.Type().Field(i).Name, err)
			}
		}
		return nil
	case reflect.Map:
		if v1.IsNil() != v2.IsNil() {
			return errs.New("a map is nil")
		}
		if v1.Len() != v2.Len() {
			return errs.New("map len mismatch")
		}
		if v1.Pointer() == v2.Pointer() {
			return nil
		}
		for _, k := range v1.MapKeys() {
			val1 := v1.MapIndex(k)
			val2 := v2.MapIndex(k)
			if !val1.IsValid() || !val2.IsValid() {
				return errs.New("invalid map index")
			}
			if err := showInequality(val1, val2); err != nil {
				return err
			}
		}
		return nil
	case reflect.Func:
		if v1.IsNil() && v2.IsNil() {
			return nil
		}
		// Can't do better than this:
		return errs.New("funcs can't be equal")
	default:
		// Normal equality suffices
		if v1.Interface() != v2.Interface() {
			return errs.New("v1 %q != v2 %q", v1, v2)
		}
		return nil
	}
}

// deepEqual is simply reflect.DeepEqual but with special handling for
// compensation.Rate.
func deepEqual(x, y interface{}) bool {
	if x == nil || y == nil {
		return x == y
	}
	v1 := reflect.ValueOf(x)
	v2 := reflect.ValueOf(y)
	if v1.Type() != v2.Type() {
		return false
	}
	return deepValueEqual(v1, v2)
}

// deepValueEqual is simply reflect.deepValueEqual but with special handling
// for compensation.Rate.
func deepValueEqual(v1, v2 reflect.Value) bool {
	if !v1.IsValid() || !v2.IsValid() {
		return v1.IsValid() == v2.IsValid()
	}
	if v1.Type() != v2.Type() {
		return false
	}

	if v1.CanInterface() {
		if dv1, ok := v1.Interface().(compensation.Rate); ok {
			return dv1.String() == v2.Interface().(compensation.Rate).String()
		}
	}

	switch v1.Kind() {
	case reflect.Array:
		for i := 0; i < v1.Len(); i++ {
			if !deepValueEqual(v1.Index(i), v2.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Slice:
		if v1.IsNil() != v2.IsNil() {
			return false
		}
		if v1.Len() != v2.Len() {
			return false
		}
		if v1.Pointer() == v2.Pointer() {
			return true
		}
		for i := 0; i < v1.Len(); i++ {
			if !deepValueEqual(v1.Index(i), v2.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Interface:
		if v1.IsNil() || v2.IsNil() {
			return v1.IsNil() == v2.IsNil()
		}
		return deepValueEqual(v1.Elem(), v2.Elem())
	case reflect.Ptr:
		if v1.Pointer() == v2.Pointer() {
			return true
		}
		return deepValueEqual(v1.Elem(), v2.Elem())
	case reflect.Struct:
		for i, n := 0, v1.NumField(); i < n; i++ {
			if !deepValueEqual(v1.Field(i), v2.Field(i)) {
				return false
			}
		}
		return true
	case reflect.Map:
		if v1.IsNil() != v2.IsNil() {
			return false
		}
		if v1.Len() != v2.Len() {
			return false
		}
		if v1.Pointer() == v2.Pointer() {
			return true
		}
		for _, k := range v1.MapKeys() {
			val1 := v1.MapIndex(k)
			val2 := v2.MapIndex(k)
			if !val1.IsValid() || !val2.IsValid() || !deepValueEqual(val1, val2) {
				return false
			}
		}
		return true
	case reflect.Func:
		if v1.IsNil() && v2.IsNil() {
			return true
		}
		// Can't do better than this:
		return false
	default:
		// Normal equality suffices
		return v1.Interface() == v2.Interface()
	}
}
